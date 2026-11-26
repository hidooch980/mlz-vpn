package orchestrator

import (
	"context"
	"fmt"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/diagnostics"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/probe"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/subscription"
)

type stubRunner struct {
	singBoxStarted bool
	xrayStarted    bool
}

func (s *stubRunner) StartSingBox(_ context.Context, _ []byte) error {
	s.singBoxStarted = true
	return nil
}
func (s *stubRunner) StopSingBox() error {
	s.singBoxStarted = false
	return nil
}
func (s *stubRunner) StartXray(_ context.Context, _ []byte) error {
	s.xrayStarted = true
	return nil
}
func (s *stubRunner) StopXray() error {
	s.xrayStarted = false
	return nil
}

type stubDialer struct{}

func (stubDialer) ProbeGenerate204(_ context.Context, _ model.Node, _ string) (int, error) {
	return 41, nil
}

type stubProber struct{}

func (stubProber) HasDefaultInterface(_ context.Context) bool               { return true }
func (stubProber) CheckClockSkewSeconds(_ context.Context) (float64, error) { return 0, nil }
func (stubProber) CheckDNSPoisoned(_ context.Context) (bool, error)         { return false, nil }
func (stubProber) HasConflictingSystemProxy() bool                          { return false }
func (stubProber) TestUDPReachable(_ context.Context) bool                  { return true }
func (stubProber) TestIranDirect(_ context.Context) bool                    { return true }

func TestOrchestratorConnectXrayChainAndDisconnect(t *testing.T) {
	runner := &stubRunner{}
	orch := New(Config{
		CacheDir:           t.TempDir(),
		RuleSetDir:         "./rules",
		DisableHealthCheck: true,
		FeedSource: subscription.FeedSourceConfig{
			EmbeddedSeeds: []model.FeedNode{
				{
					ID:        "de-1",
					Engine:    model.CoreXray,
					Protocol:  "vless",
					Region:    "DE",
					Weight:    0.9,
					ConfigURI: "vless://11111111-1111-1111-1111-111111111111@de.example.com:443?type=xhttp&security=reality&pbk=jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0&sid=0123&sni=www.microsoft.com#DE",
				},
			},
		},
	}, runner)
	orch.dialer = stubDialer{}
	orch.prober = stubProber{}

	err := orch.Connect(context.Background(), ConnectRequest{
		TunnelMode: model.TunnelModeTUN,
		EngineMode: model.EngineModeXrayAntiDPI,
		Country:    model.CountryPreference{Auto: false, Code: "DE"},
	})
	if err != nil {
		t.Fatalf("expected connect to succeed, got: %v", err)
	}
	if !runner.xrayStarted || !runner.singBoxStarted {
		t.Fatalf("expected both Xray and sing-box chained, got xray=%v singbox=%v", runner.xrayStarted, runner.singBoxStarted)
	}

	snap := orch.Snapshot()
	if snap.State != model.StateConnected || snap.Stats.RealDelayMS != 41 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}

	if err := orch.Disconnect(); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
	if runner.xrayStarted || runner.singBoxStarted {
		t.Fatal("expected all cores stopped on disconnect")
	}

	report := orch.RunSmartDiagnostics(context.Background())
	if len(report.Steps) != 9 {
		t.Fatalf("expected 9 diagnostic steps, got %d", len(report.Steps))
	}
	_ = diagnostics.HealingAction{}
}

func TestResilientEngineModeOnAndroid(t *testing.T) {
	android := New(Config{AndroidLibDir: "/data/lib"}, nil)
	if got := android.resilientEngineMode(model.EngineModeXrayAntiDPI); got != model.EngineModeAuto {
		t.Fatalf("android must degrade Xray anti-DPI to auto, got %q", got)
	}
	desktop := New(Config{}, nil)
	if got := desktop.resilientEngineMode(model.EngineModeXrayAntiDPI); got != model.EngineModeXrayAntiDPI {
		t.Fatalf("desktop should keep Xray anti-DPI, got %q", got)
	}
	if got := desktop.resilientEngineMode(""); got != model.EngineModeAuto {
		t.Fatalf("empty mode should become auto, got %q", got)
	}
}

func TestShortlistConnectionAttemptsSkipsDuplicateEndpoints(t *testing.T) {
	ranked := make([]probe.ProbeResult, 0, 20)
	for i := 0; i < 10; i++ {
		// Each endpoint appears twice, mimicking merged duplicate feed entries.
		for j := 0; j < 2; j++ {
			ranked = append(ranked, probe.ProbeResult{
				Node: model.Node{Address: "198.51.100.10", Port: 10000 + i},
			})
		}
	}

	attempts := shortlistConnectionAttempts(ranked, 8)
	if len(attempts) != 8 {
		t.Fatalf("expected 8 attempts, got %d", len(attempts))
	}
	seen := make(map[string]bool, len(attempts))
	for _, candidate := range attempts {
		key := fmt.Sprintf("%s:%d", candidate.Node.Address, candidate.Node.Port)
		if seen[key] {
			t.Fatalf("duplicate endpoint %s in attempts", key)
		}
		seen[key] = true
	}
}

func TestShortlistConnectionAttemptsKeepsLessThanLimit(t *testing.T) {
	ranked := []probe.ProbeResult{
		{Node: model.Node{Address: "203.0.113.1", Port: 443}},
		{Node: model.Node{Address: "203.0.113.1", Port: 443}},
		{Node: model.Node{Address: "203.0.113.2", Port: 8443}},
	}

	attempts := shortlistConnectionAttempts(ranked, 8)
	if len(attempts) != 2 {
		t.Fatalf("expected 2 unique endpoints, got %d", len(attempts))
	}
	if attempts[0].Node.Port != 443 || attempts[1].Node.Port != 8443 {
		t.Fatalf("order was not preserved: %+v", attempts)
	}
}
