# ساخت، امضا و انتشار MLZ VPN

این سند دستورهای تکرارپذیر برای ساخت ویندوز و اندروید، ساخت کلیدهای امضا و انتشار امن را توضیح می‌دهد.

## پیش‌نیازها
- Go **1.25.5+** (وابستگی `sing-box v1.14.4` حداقل این نسخه را می‌خواهد).
- Flutter **3.50+** و Dart 3.14+.
- برای اندروید: Android SDK 35/36، NDK **28.2.13676358** و `gomobile` فork سازگار با sing-box.
- برای نصب‌کنندهٔ ویندوز: **NSIS 3.x** (`makensis`).

## تست و بررسی
```powershell
cd engine
go test ./...
go vet ./...
cd ..\app
flutter pub get
flutter analyze
flutter test
```

تست کانفیگ هسته‌ها به باینری واقعی نیاز دارد:
```powershell
$env:SINGBOX_BIN='C:\tools\sing-box.exe'
$env:XRAY_BIN='C:\tools\xray.exe'
cd engine; go test ./pkg/cores ./pkg/orchestrator
```

## ساخت ویندوز
```powershell
cd engine
New-Item -ItemType Directory -Force bin | Out-Null
go build -trimpath -ldflags="-s -w -H=windowsgui -X main.version=1.0.0 -X main.buildNumber=100" -o bin\mlz-engine.exe .\cmd\mlz-engine
cd ..\app
flutter build windows --release
```
خروجی در `app\build\windows\x64\runner\Release` است. فایل‌های `sing-box.exe`، `xray.exe` و پوشه `rules` باید کنار `mlz-engine.exe` قرار گیرند (workflow انتشار این کار را انجام می‌دهد).

## ساخت نصب‌کنندهٔ ویندوز (Setup)
پس از کامل‌شدن Release ویندوز و قرارگرفتن `cores` و `rules` در آن:
```powershell
cd D:\aziz\VPN
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build_windows_installer.ps1
```
خروجی پیش‌فرض `dist\windows-x64\mlz-vpn-windows-x64-setup.exe` است. نصب‌کننده per-user و بدون نیاز به Administrator در `%LOCALAPPDATA%\Programs\MLZ VPN` نصب می‌کند، میان‌بر Start Menu و Uninstaller می‌سازد و در صورت انتخاب، میان‌بر دسکتاپ اضافه می‌کند. برای اتصال TUN، خود برنامه هنگام Connect درخواست UAC می‌دهد؛ حالت Proxy بدون UAC کار می‌کند.

> نصب‌کننده با گواهی Code Signing امضا نشده است؛ ویندوز ممکن است پیام SmartScreen نشان دهد. برای انتشار عمومی، `setup.exe` را با گواهی EV/Code Signing امضا کنید. workflow انتشار، `setup.exe` را در Release و `version.json` به‌عنوان asset `windows_x64_setup` قرار می‌دهد و به‌روزرسانی داخل برنامه دقیقاً همین فایل را پس از بررسی SHA-256 دانلود و اجرا می‌کند؛ بستهٔ ZIP برای دانلود دستی باقی می‌ماند.

> ویندوز برای ساخت پلاگین‌های Flutter به symlink نیاز دارد. اگر Developer Mode فعال نیست، یا آن را از `start ms-settings:developers` روشن کنید یا برای هر پلاگین در `app\windows\flutter\ephemeral\.plugin_symlinks` یک Junction بسازید. فایل `windows/CMakeLists.txt` نیز تعریف `_SILENCE_EXPERIMENTAL_COROUTINE_DEPRECATION_WARNINGS` را برای سازگاری `local_auth_windows` با MSVC جدید تنظیم می‌کند.

## ساخت AAR و APK اندروید
داده‌مسیر واقعی اندروید داخل AAR قرار دارد؛ بدون آن برنامه به‌صورت امن اتصال را رد می‌کند.
```powershell
go install github.com/sagernet/gomobile/cmd/gomobile@v0.1.13
go install github.com/sagernet/gomobile/cmd/gobind@v0.1.13
$env:ANDROID_HOME='C:\Android\Sdk'
$env:ANDROID_NDK_HOME='C:\Android\Sdk\ndk\28.2.13676358'
gomobile init
cd engine
gomobile bind `
  '-target=android/arm64,android/arm,android/amd64' -androidapi 24 `
  -tags 'with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_clash_api,badlinkname,tfogo_checklinkname0' `
  '-ldflags=-checklinkname=0' `
  -o ..\app\android\app\libs\mlz-core.aar .\mobile
cd ..\app
flutter build apk --release --split-per-abi --target-platform android-arm,android-arm64,android-x64
```
`badlinkname` و `-checklinkname=0` برای نسخه Go 1.25 الزامی‌اند؛ بدون آن‌ها لینک `libgojni.so` با خطای `os.checkPidfdOnce` شکست می‌خورد. `--split-per-abi` همان مسیر workflow است و سه APK جدا برای arm64، armv7 و x86_64 می‌سازد؛ versionCode آن‌ها برای build پایهٔ ۱۰۰ به‌ترتیب 2100، 1100 و 4100 می‌شود و بررسی به‌روزرسانی در Kotlin بخش هزارگان را حذف می‌کند. نسخهٔ x86_64 برای شبیه‌ساز و Chromebook است.

> داده‌مسیر اندروید از libbox استفاده می‌کند. Go نباید با `netlink` اینترفیس‌ها را شمارش کند؛ روی Android 10+ SELinux مسیر `route ip+net: netlinkrib` را رد می‌کند. Kotlin پیش از `init` یک snapshot از `ConnectivityManager` و `NetworkInterface` می‌سازد و به Go می‌دهد. این رفتار با شبیه‌ساز x86_64 و اتصال واقعی TUN تست شده است.

### تست روی شبیه‌ساز x86_64
```powershell
$adb = "$env:ANDROID_HOME\platform-tools\adb.exe"
& $adb install -r app\build\app\outputs\flutter-apk\app-x86_64-release.apk
& $adb shell am start -n com.mlzvpn.app/.MainActivity
```
پس از تأیید پنجرهٔ VpnService، دکمهٔ اتصال را بزنید. مسیر درست با نمایش «متصل»، عبور یک صفحهٔ وب از مرورگر و تست IP مشخص می‌شود. لاگ‌های SELinux نباید باعث توقف سرویس شوند؛ snapshot شبکه در Kotlin ساخته می‌شود و Go دیگر `netlink` را روی اندروید صدا نمی‌زند.

## به‌روزرسانی داخل برنامه
1. نسخهٔ `app/pubspec.yaml` را مثلاً به `1.0.3+103` افزایش دهید و همان عدد را در `engine/cmd/mlz-engine/main.go` و `engine/mobile/api.go` هم بگذارید.
2. تگ `v1.0.3` را push کنید تا workflow، `version.json` امضاشده و همهٔ assetها را بسازد.
3. در ویندوز، برنامه فایل `windows_x64_setup` را دانلود و پس از بررسی SHA-256 اجرا می‌کند.
4. در اندروید، برنامه asset متناظر ABI (`android_arm64`، `android_armv7` یا `android_x86_64`) را دانلود می‌کند، SHA-256 را تطبیق می‌دهد و با `FileProvider` نصب را شروع می‌کند. اگر مجوز «نصب از منابع ناشناس» داده نشده باشد، تنظیمات مربوطه باز می‌شود.

## کلیدهای امضا
### کلید فید و مانیفست
```powershell
cd feeds
go run ./cmd/keygen
```
خروجی دو مقدار می‌دهد: `MLZ_FEED_PRIVKEY` (فقط در GitHub Secrets) و `MLZ_FEED_PUBKEY` (باید با مقدار `feedPublicKey` در `engine/cmd/mlz-engine/main.go` و `initFeed` در `engine/mobile/api.go` یکسان باشد).

### کلید APK
```powershell
keytool -genkeypair -v -keystore app\android\keystore.jks -alias mlz -keyalg RSA -keysize 4096 -validity 10000
```
فایل `app/android/key.properties` (هرگز commit نشود):
```properties
storePassword=...
keyPassword=...
keyAlias=mlz
storeFile=../keystore.jks
```

## انتشار خودکار
workflow `.github/workflows/release.yml` با push تگ `v*` اجرا می‌شود، ZIP portable و `setup.exe` ویندوز را می‌سازد و باید این Secretها تنظیم باشند:
- `MLZ_FEED_PRIVKEY`: کلید خصوصی Ed25519 (Base64).
- `ANDROID_KEYSTORE_BASE64`: محتوای Base64 فایل `keystore.jks`.
- `ANDROID_KEYSTORE_PASSWORD`، `ANDROID_KEY_ALIAS`، `ANDROID_KEY_PASSWORD`.

اگر Secretهای اندروید موجود نباشند، job اندروید باید متوقف شود؛ انتشار APK با امضای debug مجاز نیست.

## فید و مانیفست
- `feeds/feed.v1.json` هر ۵ دقیقه با workflow `sync-feed-and-rules.yml` و کلید `MLZ_FEED_PRIVKEY` امضا می‌شود.
- `version.json` در انتشار با `feeds/cmd/signmanifest` امضا می‌شود و شامل `android_arm64`، `android_armv7`، `android_x86_64`، `windows_x64` و `windows_x64_setup` است.
- کلاینت هر دو را با کلید عمومی داخل باینری بررسی می‌کند؛ امضای نامعتبر، فید قدیمی‌تر از TTL یا rollback رد می‌شود.
- نصب‌کنندهٔ ویندوز و APKها باید با کلید یکسان نسخه‌های قبلی امضا شوند؛ در غیر این صورت به‌روزرسانی درجا توسط سیستم‌عامل رد می‌شود.
