// Package mobile exposes gomobile-compatible APIs compiled into mlz-core.aar
// for direct invocation by Kotlin MlzVpnService on Android.
package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/diagnostics"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/orchestrator"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/subscription"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/updater"
)

// SocketProtector is implemented in Kotlin via Android VpnService.protect(fd)
// to prevent outgoing proxy sockets from looping back into the TUN interface.
type SocketProtector interface {
	Protect(fd int32) bool
}

var (
	mu           sync.Mutex
	orch         *orchestrator.Orchestrator
	bgCancel     context.CancelFunc
	activeRunner orchestrator.CoreRunner
)

// Init initializes the shared Android Go engine inside MlzVpnService and
// returns a JSON status string (including an error when initialization fails).
func Init(cacheDir, ruleSetDir, nativeLibDir, networkJSON string, tunFD int32, protector SocketProtector) string {
	mu.Lock()
	runner := newAndroidRunner(cacheDir, tunFD, protector, networkJSON)
	activeRunner = runner
	orch = orchestrator.New(orchestrator.Config{
		CurrentBuild:    109,
		CurrentVersion:  "1.0.9",
		UpdatePublicKey: "6ZcGl3tK1+MmN/ix3C14i//iC8kyBWt7vagvP7NfWm8=",
		CacheDir:        cacheDir,
		RuleSetDir:      ruleSetDir,
		AndroidLibDir:   nativeLibDir,
		LocalMixedPort:  10809,
		LANSharePort:    10808,
		FeedSource: subscription.FeedSourceConfig{
			PublicKeyBase64: "6ZcGl3tK1+MmN/ix3C14i//iC8kyBWt7vagvP7NfWm8=",
			EmbeddedSeeds:   subscription.EmbeddedSeeds(),
			FeedURLs: []string{
				"https://raw.githubusercontent.com/hidooch980/mlz-vpn/main/feeds/feed.v1.json",
				"https://cdn.jsdelivr.net/gh/hidooch980/mlz-vpn@main/feeds/feed.v1.json",
			},
		},
		ManifestURLs: []string{
			"https://github.com/hidooch980/mlz-vpn/releases/latest/download/version.json",
		},
	}, runner)
	if bgCancel != nil {
		bgCancel()
	}
	bgCtx, cancel := context.WithCancel(context.Background())
	bgCancel = cancel
	o := orch
	mu.Unlock()
	go o.StartBackgroundSync(bgCtx)
	return GetStatusJSON()
}

// UpdatePlatformNetwork refreshes the Android interface snapshot after Wi-Fi,
// cellular, or hotspot changes. Kotlin calls this from a ConnectivityManager
// callback so the running tunnel can switch its underlying network without a
// full reconnect. It is a no-op on non-Android builds.
func UpdatePlatformNetwork(networkJSON string) string {
	mu.Lock()
	runner := activeRunner
	mu.Unlock()
	if runner == nil {
		return `{"ok":false,"error":"engine not initialized"}`
	}
	updater, ok := runner.(interface{ updateNetworkJSON(string) })
	if !ok {
		return `{"ok":false,"error":"network update is not supported on this platform"}`
	}
	updater.updateNetworkJSON(networkJSON)
	return `{"ok":true}`
}

// Connect starts the smart connection pipeline and returns JSON status.
func Connect(engineMode, countryCode string, batterySaver bool) string {
	mu.Lock()
	o := orch
	mu.Unlock()
	if o == nil {
		return `{"error":"engine not initialized"}`
	}

	country := model.CountryPreference{Auto: countryCode == "" || countryCode == "auto", Code: countryCode}
	_ = o.Connect(context.Background(), orchestrator.ConnectRequest{
		TunnelMode:   model.TunnelModeTUN,
		EngineMode:   model.EngineMode(engineMode),
		Country:      country,
		BatterySaver: batterySaver,
	})
	return GetStatusJSON()
}

// Disconnect stops all cores cleanly.
func Disconnect() string {
	mu.Lock()
	o := orch
	mu.Unlock()
	if o != nil {
		_ = o.Disconnect()
	}
	return GetStatusJSON()
}

// TestIPHealth runs the clickable IP rectangle verification and returns JSON.
func TestIPHealth() string {
	mu.Lock()
	o := orch
	mu.Unlock()
	if o == nil {
		return `{"error":"engine not initialized"}`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res, err := o.TestIPHealth(ctx)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	b, _ := json.Marshal(res)
	return string(b)
}

// RunDiagnostics runs the 9-step diagnostic pipeline and returns JSON.
func RunDiagnostics() string {
	mu.Lock()
	o := orch
	mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if o == nil {
		// The VPN service is not running yet (the user opened diagnostics while
		// disconnected). Still run the local, read-only checks so the screen can
		// explain the base network/DNS/DPI state instead of failing with a raw
		// engine error. Core/exit checks remain informational.
		report, _ := diagnostics.RunDiagnostics(ctx, diagnostics.NewLiveProber())
		b, _ := json.Marshal(report)
		return string(b)
	}
	b, _ := json.Marshal(o.RunSmartDiagnostics(ctx))
	return string(b)
}

// AutoHealDiagnostics runs diagnostics, applies safe automatic repairs, and
// verifies the exit IP. Exposed to Flutter through the same JSON bridge.
func AutoHealDiagnostics() string {
	mu.Lock()
	o := orch
	mu.Unlock()
	if o == nil {
		return `{"error":"برای رفع خودکار، ابتدا اتصال VPN را برقرار کنید."}`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	b, _ := json.Marshal(o.AutoHealDiagnostics(ctx))
	return string(b)
}

// ToggleLANSharing toggles hotspot proxy sharing on 0.0.0.0:10808.
func ToggleLANSharing(enabled, usePIN bool) string {
	mu.Lock()
	o := orch
	mu.Unlock()
	if o == nil {
		return `{"error":"engine not initialized"}`
	}
	info, err := o.SetLANSharing(context.Background(), enabled, usePIN)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	b, _ := json.Marshal(info)
	return string(b)
}

// CheckAppUpdate checks GitHub Releases and returns JSON for the Flutter UI.
func CheckAppUpdate() string {
	mu.Lock()
	o := orch
	mu.Unlock()
	if o == nil {
		return `{"error":"engine not initialized"}`
	}
	res, err := o.CheckAppUpdate(context.Background())
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	b, _ := json.Marshal(res)
	return string(b)
}

// CheckAppUpdateStandalone checks GitHub Releases without requiring a live
// tunnel, so the Android UI can offer updates before connecting.
func CheckAppUpdateStandalone(currentBuild int32, currentVersion string) string {
	res, err := updater.CheckForUpdateVersionSigned(context.Background(), int(currentBuild), currentVersion, []string{
		"https://github.com/hidooch980/mlz-vpn/releases/latest/download/version.json",
		"https://raw.githubusercontent.com/hidooch980/mlz-vpn/main/version.json",
	}, 0, "6ZcGl3tK1+MmN/ix3C14i//iC8kyBWt7vagvP7NfWm8=")
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	b, _ := json.Marshal(res)
	return string(b)
}

// DownloadAppUpdate downloads an APK asset from a signed release manifest into
// the app cache and verifies its SHA-256 before the UI asks Android to install it.
// cacheDir must be the same directory exposed by the FileProvider (cache/updates).
func DownloadAppUpdate(cacheDir, assetURL, sha256 string) string {
	if strings.TrimSpace(cacheDir) == "" {
		return updateErrorJSON("مسیر کش برنامه برای دریافت به‌روزرسانی در دسترس نیست.")
	}
	u, err := url.Parse(strings.TrimSpace(assetURL))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return updateErrorJSON("نشانی دانلود به‌روزرسانی باید HTTPS باشد.")
	}
	if strings.TrimSpace(sha256) == "" {
		return updateErrorJSON("چک‌سام به‌روزرسانی در مانیفست وجود ندارد.")
	}
	dir := filepath.Join(cacheDir, "updates")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return updateErrorJSON(fmt.Sprintf("ساخت پوشه به‌روزرسانی ناموفق بود: %v", err))
	}
	dest := filepath.Join(dir, "mlz-vpn-update.apk")
	_ = os.Remove(dest)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := updater.DownloadAndVerify(ctx, updater.AssetInfo{URL: u.String(), SHA256: sha256}, dest, 0); err != nil {
		return updateErrorJSON(fmt.Sprintf("دریافت به‌روزرسانی ناموفق بود: %v", err))
	}
	info, err := os.Stat(dest)
	if err != nil || info.Size() <= 0 {
		return updateErrorJSON("فایل به‌روزرسانی پس از دانلود معتبر نیست.")
	}
	b, _ := json.Marshal(map[string]any{
		"path":   dest,
		"bytes":  info.Size(),
		"sha256": strings.ToLower(strings.TrimSpace(sha256)),
	})
	return string(b)
}

func updateErrorJSON(message string) string {
	b, _ := json.Marshal(map[string]string{"error": message})
	return string(b)
}

// GetStatusJSON returns the current StatusSnapshot as JSON for Flutter EventChannel.
func GetStatusJSON() string {
	mu.Lock()
	o := orch
	mu.Unlock()
	if o == nil {
		return `{"state":"disconnected"}`
	}
	b, _ := json.Marshal(o.Snapshot())
	return string(b)
}
