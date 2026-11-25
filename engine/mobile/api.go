// Package mobile exposes gomobile-compatible APIs compiled into mlz-core.aar
// for direct invocation by Kotlin MlzVpnService on Android.
package mobile

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/orchestrator"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/subscription"
)

// SocketProtector is implemented in Kotlin via Android VpnService.protect(fd)
// to prevent outgoing proxy sockets from looping back into the TUN interface.
type SocketProtector interface {
	Protect(fd int32) bool
}

type androidRunner struct {
	tunFD     int32
	protector SocketProtector
}

func (a *androidRunner) StartSingBox(_ context.Context, _ []byte) error { return nil }
func (a *androidRunner) StopSingBox() error                             { return nil }
func (a *androidRunner) StartXray(_ context.Context, _ []byte) error    { return nil }
func (a *androidRunner) StopXray() error                                { return nil }

var (
	mu   sync.Mutex
	orch *orchestrator.Orchestrator
)

// Init initializes the shared Android Go engine inside MlzVpnService.
func Init(cacheDir, ruleSetDir, nativeLibDir string, tunFD int32, protector SocketProtector) {
	mu.Lock()
	defer mu.Unlock()
	runner := &androidRunner{tunFD: tunFD, protector: protector}
	orch = orchestrator.New(orchestrator.Config{
		CurrentBuild:   100,
		CacheDir:       cacheDir,
		RuleSetDir:     ruleSetDir,
		AndroidLibDir:  nativeLibDir,
		LocalMixedPort: 10809,
		LANSharePort:   10808,
		FeedSource: subscription.FeedSourceConfig{
			PublicKeyBase64: "6ZcGl3tK1+MmN/ix3C14i//iC8kyBWt7vagvP7NfWm8=",
			FeedURLs: []string{
				"https://cdn.jsdelivr.net/gh/hidooch980/mlz-vpn@main/feeds/feed.v1.json",
				"https://raw.githubusercontent.com/hidooch980/mlz-vpn/main/feeds/feed.v1.json",
			},
		},
		ManifestURLs: []string{
			"https://github.com/hidooch980/mlz-vpn/releases/latest/download/version.json",
		},
	}, runner)
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
	res, err := o.TestIPHealth(context.Background())
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	b, _ := json.Marshal(res)
	return string(b)
}

// RunDiagnostics runs the 7-step diagnostic pipeline and returns JSON.
func RunDiagnostics() string {
	mu.Lock()
	o := orch
	mu.Unlock()
	if o == nil {
		return `{"error":"engine not initialized"}`
	}
	b, _ := json.Marshal(o.RunSmartDiagnostics(context.Background()))
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
	info, _ := o.SetLANSharing(context.Background(), enabled, usePIN)
	b, _ := json.Marshal(info)
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
