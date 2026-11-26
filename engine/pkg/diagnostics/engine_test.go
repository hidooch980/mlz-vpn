package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

type mockProber struct {
	hasNet     bool
	skew       float64
	skewErr    error
	poisoned   bool
	conflict   bool
	udpOK      bool
	iranDirect bool
}

func (m mockProber) HasDefaultInterface(_ context.Context) bool { return m.hasNet }
func (m mockProber) CheckClockSkewSeconds(_ context.Context) (float64, error) {
	return m.skew, m.skewErr
}
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

	if len(report.Steps) != 9 {
		t.Fatalf("expected 9 diagnostic steps, got %d", len(report.Steps))
	}
	if !action.SwitchToDoH {
		t.Fatal("expected SwitchToDoH when DNS is poisoned")
	}
	if !action.ForceTCPFallback || action.RecommendedMode != model.EngineModeXrayAntiDPI {
		t.Fatalf("expected Xray anti-DPI TCP fallback when UDP is blocked: %+v", action)
	}
	if !action.Reconnect {
		t.Fatal("expected a reconnect plan for DNS/DPI issues")
	}
	if report.HealthScore >= 90 {
		t.Fatalf("expected degraded health score, got %d", report.HealthScore)
	}
}

func TestRunDiagnosticsDetectsDeadCore(t *testing.T) {
	report, action := RunDiagnostics(context.Background(), mockProber{
		hasNet: true, skew: 0, poisoned: false, conflict: false, udpOK: true, iranDirect: true,
	}, RuntimeContext{
		Connected: true, State: "connected", ActiveCore: "sing-box", NodeCount: 5,
		FeedAge: time.Minute, ProxyPort: 1,
	})
	if !action.CoreUnhealthy || !action.Reconnect {
		t.Fatalf("expected dead-core healing action, got %+v", action)
	}
	found := false
	for _, step := range report.Steps {
		if step.ID == "cores" && step.Status == "failed" && step.CanAutoFix {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a failed, auto-fixable cores step")
	}
}

func TestRunDiagnosticsBaseInterfaceUsesTunnel(t *testing.T) {
	// Android can expose only the VPN tun interface; a connected session should
	// still be reported as healthy instead of "no active network card".
	report, _ := RunDiagnostics(context.Background(), mockProber{
		hasNet: false, skewErr: context.DeadlineExceeded, udpOK: false, iranDirect: false,
	}, RuntimeContext{
		Connected: true, State: "connected", ActiveCore: "sing-box", NodeCount: 3,
		FeedAge: time.Minute, LastIP: &model.IPTestResult{PublicIP: "198.51.100.7"},
	})
	step := stepByID(report, "interface")
	if step.Status == "failed" {
		t.Fatalf("connected tunnel must pass the interface check: %+v", step)
	}
	if !strings.Contains(step.DetailFA, "تونل VPN") {
		t.Fatalf("expected tunnel-aware detail, got %q", step.DetailFA)
	}

	offline, _ := RunDiagnostics(context.Background(), mockProber{
		hasNet: false, skewErr: context.DeadlineExceeded, udpOK: false, iranDirect: false,
	})
	if stepByID(offline, "interface").Status != "failed" {
		t.Fatal("expected offline diagnostics to fail the interface check")
	}
}

func stepByID(report model.DiagnosticsReport, id string) model.DiagnosticStep {
	for _, step := range report.Steps {
		if step.ID == id {
			return step
		}
	}
	return model.DiagnosticStep{}
}

func TestRunDiagnosticsOfflineIsUseful(t *testing.T) {
	report, action := RunDiagnostics(context.Background(), mockProber{
		hasNet: true, skew: 0, poisoned: false, conflict: false, udpOK: true, iranDirect: true,
	})
	if stepByID(report, "feed").Status != "info" || stepByID(report, "cores").Status != "info" {
		t.Fatalf("offline feed/core checks must be informational: %+v", report.Steps)
	}
	if stepByID(report, "exit").Status != "failed" {
		t.Fatal("offline run should warn that the VPN is not connected")
	}
	if report.HeadlineFA != "اتصال VPN برقرار نیست" {
		t.Fatalf("unexpected offline headline: %q", report.HeadlineFA)
	}
	if action.RefreshFeed || action.Reconnect || action.CoreUnhealthy {
		t.Fatalf("offline run must not mutate the engine: %+v", action)
	}
}

func TestRunDiagnosticsExplainsFailedConnection(t *testing.T) {
	const lastErr = "هیچ‌کدام از ۸ مسیر برتر اتصال سالم برقرار نکردند"
	report, action := RunDiagnostics(context.Background(), mockProber{
		hasNet: true, skew: 0, poisoned: false, conflict: false, udpOK: true, iranDirect: true,
	}, RuntimeContext{
		State: "disconnected", RuntimeKnown: true, NodeCount: 24, FeedAge: time.Minute,
		LastErrorFA: lastErr,
	})

	exit := stepByID(report, "exit")
	if exit.Status != "failed" || !strings.Contains(exit.DetailFA, lastErr) {
		t.Fatalf("exit step must carry the real connect failure, got %+v", exit)
	}
	if strings.Contains(exit.DetailFA, "ابتدا وصل شوید") {
		t.Fatalf("failed connection must not tell the user to connect first: %q", exit.DetailFA)
	}
	cores := stepByID(report, "cores")
	if cores.Status != "failed" || !strings.Contains(cores.DetailFA, lastErr) {
		t.Fatalf("core step must carry the real connect failure, got %+v", cores)
	}
	if cores.CanAutoFix {
		t.Fatal("a Go-only repair cannot restore Android TUN; do not advertise it here")
	}
	if report.HeadlineFA != "اتصال VPN برقرار نشد" {
		t.Fatalf("unexpected failed-connect headline: %q", report.HeadlineFA)
	}
	if report.HealthScore >= 86 {
		t.Fatalf("failed connection must lower the score, got %d", report.HealthScore)
	}
	if !report.NeedsAction {
		t.Fatal("failed connection must be actionable")
	}
	if action.Reconnect {
		t.Fatal("diagnostics must not start a TUN reconnect on its own")
	}
}

func TestRunDiagnosticsHealthyWithRuntime(t *testing.T) {
	report, action := RunDiagnostics(context.Background(), mockProber{
		hasNet: true, skew: 0, poisoned: false, conflict: false, udpOK: true, iranDirect: true,
	}, RuntimeContext{
		Connected: true, State: "connected", EngineMode: "auto", TunnelMode: "tun",
		ActiveCore: "sing-box", ActiveNode: "de-1", NodeCount: 12,
		XrayNodes: 4, SingBoxNodes: 8, FeedAge: 2 * time.Minute,
		LastIP: &model.IPTestResult{PublicIP: "203.0.113.4", CountryName: "آلمان", RealDelayMS: 55},
	})

	if report.NeedsAction {
		t.Fatalf("expected no required action, got %+v", report)
	}
	if report.HealthScore != 100 || report.HealthLabelFA != "عالی" {
		t.Fatalf("unexpected health: %d %s", report.HealthScore, report.HealthLabelFA)
	}
	if action.Reconnect || action.RefreshFeed || action.ForceTCPFallback {
		t.Fatalf("did not expect healing action for a healthy run: %+v", action)
	}
	for _, step := range report.Steps {
		if step.Status != "ok" && step.Status != "info" {
			t.Fatalf("expected healthy steps, got %s (%s)", step.ID, step.Status)
		}
	}
}
