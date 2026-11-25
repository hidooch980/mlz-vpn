package orchestrator

import (
	"context"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/diagnostics"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
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
	if len(report.Steps) != 7 {
		t.Fatalf("expected 7 diagnostic steps, got %d", len(report.Steps))
	}
	_ = diagnostics.HealingAction{}
}
