Unicode true
ManifestDPIAware true

; -----------------------------------------------------------------------------
; MLZ VPN Windows installer (NSIS 3.x)
; -----------------------------------------------------------------------------
; Build with scripts/build_windows_installer.ps1, which supplies:
;   APP_VERSION   semantic version from app/pubspec.yaml (e.g. 1.0.0)
;   APP_BUILD     build number from app/pubspec.yaml (e.g. 100)
;   SOURCE_DIR    Flutter Windows Release directory
;   OUTPUT_FILE   destination setup .exe
;   APP_ICON      app icon .ico
;   LICENSE_FILE  project LICENSE
; -----------------------------------------------------------------------------

!ifndef APP_VERSION
  !error "APP_VERSION is required"
!endif
!ifndef APP_BUILD
  !define APP_BUILD "0"
!endif
!ifndef SOURCE_DIR
  !error "SOURCE_DIR is required"
!endif
!ifndef OUTPUT_FILE
  !error "OUTPUT_FILE is required"
!endif
!ifndef APP_ICON
  !define APP_ICON "..\runner\resources\app_icon.ico"
!endif
!ifndef LICENSE_FILE
  !define LICENSE_FILE "..\..\..\LICENSE"
!endif

!define APP_NAME "MLZ VPN"
!define APP_PUBLISHER "MLZ VPN"
!define APP_URL "https://github.com/hidooch980/mlz-vpn"
!define APP_EXE "mlz_vpn.exe"
!define APP_ENGINE "mlz-engine.exe"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\MLZVPN"

Name "${APP_NAME} ${APP_VERSION}"
Caption "${APP_NAME} ${APP_VERSION} Setup"
OutFile "${OUTPUT_FILE}"
Icon "${APP_ICON}"
UninstallIcon "${APP_ICON}"
BrandingText "${APP_NAME} ${APP_VERSION}"
InstallDir "$LOCALAPPDATA\Programs\${APP_NAME}"
InstallDirRegKey HKCU "Software\${APP_NAME}" "InstallDir"
RequestExecutionLevel user
SetCompressor /SOLID lzma
SetCompressorDictSize 64

VIProductVersion "${APP_VERSION}.${APP_BUILD}"
VIAddVersionKey /LANG=0 "ProductName" "${APP_NAME}"
VIAddVersionKey /LANG=0 "CompanyName" "${APP_PUBLISHER}"
VIAddVersionKey /LANG=0 "FileDescription" "${APP_NAME} Setup"
VIAddVersionKey /LANG=0 "FileVersion" "${APP_VERSION}.${APP_BUILD}"
VIAddVersionKey /LANG=0 "ProductVersion" "${APP_VERSION}"
VIAddVersionKey /LANG=0 "LegalCopyright" "MIT License"

!include "MUI2.nsh"
!include "FileFunc.nsh"

!define MUI_ABORTWARNING
!define MUI_ICON "${APP_ICON}"
!define MUI_UNICON "${APP_ICON}"
!define MUI_WELCOMEPAGE_TITLE "نصب ${APP_NAME}"
!define MUI_WELCOMEPAGE_TEXT "این برنامه نسخهٔ رایگان و متن‌باز ${APP_NAME} را روی ویندوز نصب می‌کند.$\r$\n$\r$\nاتصال TUN در زمان استفاده درخواست دسترسی Administrator (UAC) می‌دهد؛ حالت پروکسی بدون UAC کار می‌کند."
!define MUI_FINISHPAGE_TITLE "${APP_NAME} نصب شد"
!define MUI_FINISHPAGE_TEXT "می‌توانید همین حالا برنامه را اجرا کنید. برای اتصال TUN، ویندوز درخواست UAC نمایش می‌دهد."
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "اجرای ${APP_NAME}"
!define MUI_FINISHPAGE_NOREBOOTSUPPORT

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${LICENSE_FILE}"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "Farsi"

Section "برنامهٔ اصلی (الزامی)" SEC_MAIN
  SectionIn RO
  SetShellVarContext current

  ; Stop an old build before replacing its executables. Ignore failures when
  ; the app is not running.
  nsExec::ExecToLog "taskkill /F /IM ${APP_EXE}"
  nsExec::ExecToLog "taskkill /F /IM ${APP_ENGINE}"
  Sleep 500

  SetOutPath "$INSTDIR"
  File /r "${SOURCE_DIR}\*"
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  CreateDirectory "$SMPROGRAMS\${APP_NAME}"
  CreateShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0 SW_SHOWNORMAL "" "${APP_NAME}"
  CreateShortcut "$SMPROGRAMS\${APP_NAME}\Uninstall ${APP_NAME}.lnk" "$INSTDIR\Uninstall.exe"

  WriteRegStr HKCU "Software\${APP_NAME}" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "${APP_PUBLISHER}"
  WriteRegStr HKCU "${UNINST_KEY}" "URLInfoAbout" "${APP_URL}"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${APP_EXE},0"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" "$\"$INSTDIR\Uninstall.exe$\""
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\Uninstall.exe$\" /S"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
SectionEnd

Section /o "میان‌بر روی دسکتاپ" SEC_DESKTOP
  SetShellVarContext current
  CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0 SW_SHOWNORMAL "" "${APP_NAME}"
SectionEnd

Section "Uninstall"
  SetShellVarContext current
  nsExec::ExecToLog "taskkill /F /IM ${APP_EXE}"
  nsExec::ExecToLog "taskkill /F /IM ${APP_ENGINE}"
  Sleep 500

  Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}\Uninstall ${APP_NAME}.lnk"
  RMDir "$SMPROGRAMS\${APP_NAME}"
  Delete "$DESKTOP\${APP_NAME}.lnk"

  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\${APP_NAME}"

  RMDir /r "$INSTDIR"
SectionEnd
