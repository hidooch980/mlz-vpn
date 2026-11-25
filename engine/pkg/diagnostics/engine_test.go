package diagnostics

import (
	"context"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

type mockProber struct {
	hasNet     bool
	skew       float64
	poisoned   bool
	conflict   bool
	udpOK      bool
	iranDirect bool
}

func (m mockProber) HasDefaultInterface(_ context.Context) bool      { return m.hasNet }
func (m mockProber) CheckClockSkewSeconds(_ context.Context) (float64, error) { return m.skew, nil }
func (m mockProber) CheckDNSPoisoned(_ context.Context) (bool, error) { return m.poisoned, nil }
func (m mockProber) HasConflictingSystemProxy() bool                  { return m.conflict }
func (m mockProber) TestUDPReachable(_ context.Context) bool          { return m.udpOK }
func (m mockProber) TestIranDirect(_ context.Context) bool            { return m.iranDirect }

func TestRunDiagnosticsAutoHeal(t *testing.T) {
	report, action := RunDiagnostics(context.Background(), mockProber{
		hasNet:     true,
		skew:       1.5,
		poisoned:   true,
		conflict:   false,
		udpOK:      false,
		iranDirect: true,
	})

	if len(report.Steps) != 7 {
		t.Fatalf("expected 7 diagnostic steps, got %d", len(report.Steps))
	}
	if !action.SwitchToDoH {
		t.Fatal("expected SwitchToDoH when DNS is poisoned")
	}
	if !action.ForceTCPFallback || action.RecommendedMode != model.EngineModeXrayAntiDPI {
		t.Fatalf("expected Xray anti-DPI TCP fallback when UDP is blocked: %+v", action)
	}
}
