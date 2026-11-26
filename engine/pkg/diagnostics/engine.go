// Package diagnostics runs the smart 9-step network diagnostic pipeline,
// classifies ISP censorship patterns, applies automatic healing, and reports
// simple Persian advice with a health score and remediation hints.
package diagnostics

import (
	"context"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

// Prober abstracts external probes so tests can simulate ISP blocks deterministically.
type Prober interface {
	HasDefaultInterface(ctx context.Context) bool
	CheckClockSkewSeconds(ctx context.Context) (float64, error)
	CheckDNSPoisoned(ctx context.Context) (poisoned bool, err error)
	HasConflictingSystemProxy() bool
	TestUDPReachable(ctx context.Context) bool
	TestIranDirect(ctx context.Context) bool
}

// RuntimeContext carries live orchestrator state into the deterministic probes
// without coupling this package to the orchestrator (and without import cycles).
type RuntimeContext struct {
	Connected    bool
	State        string
	EngineMode   string
	TunnelMode   string
	ActiveCore   string
	ActiveNode   string
	NodeCount    int
	XrayNodes    int
	SingBoxNodes int
	FeedAge      time.Duration
	LastIP       *model.IPTestResult
	LastErrorFA  string
	ProxyPort    int
	CoreFailure  string
	// RuntimeKnown is set by the orchestrator when the VPN engine has live
	// state. The mobile AAR can also run diagnostics while disconnected (no
	// engine yet), and in that mode feed/core checks must not be false alarms.
	RuntimeKnown bool
}

// LiveProber executes real network checks against the host network.
type LiveProber struct {
	Client *http.Client
}

func NewLiveProber() *LiveProber {
	return &LiveProber{
		Client: &http.Client{Timeout: 4 * time.Second},
	}
}

func (l *LiveProber) HasDefaultInterface(_ context.Context) bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			return true
		}
	}
	return false
}

func (l *LiveProber) CheckClockSkewSeconds(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://cloudflare.com", nil)
	if err != nil {
		return 0, err
	}
	resp, err := l.Client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()

	dateHdr := resp.Header.Get("Date")
	if dateHdr == "" {
		return 0, nil
	}
	serverTime, err := http.ParseTime(dateHdr)
	if err != nil {
		return 0, err
	}
	return math.Abs(time.Since(serverTime).Seconds()), nil
}

func (l *LiveProber) CheckDNSPoisoned(ctx context.Context) (bool, error) {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, "www.youtube.com")
	if err != nil {
		return true, err
	}
	for _, ip := range ips {
		// Common Iranian DNS hijack sinkhole range (10.10.34.x) or private IPs for public domains.
		if ip.IP.IsPrivate() || strings.HasPrefix(ip.IP.String(), "10.10.34.") {
			return true, nil
		}
	}
	return false, nil
}

func (l *LiveProber) HasConflictingSystemProxy() bool {
	for _, env := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" && !strings.Contains(v, "127.0.0.1") {
			return true
		}
	}
	return false
}

func (l *LiveProber) TestUDPReachable(ctx context.Context) bool {
	dialer := net.Dialer{Timeout: 2500 * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "udp", "1.1.1.1:53")
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	// Minimal DNS A query for example.com: proves UDP egress and a response path.
	query := []byte{
		0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x07, 'e', 'x', 'a',
		'm', 'p', 'l', 'e', 0x03, 'c', 'o', 'm', 0x00,
		0x00, 0x01, 0x00, 0x01,
	}
	if _, err := conn.Write(query); err != nil {
		return false
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	return err == nil && n >= 12
}

func (l *LiveProber) TestIranDirect(ctx context.Context) bool {
	for _, endpoint := range []string{"https://dns.shecan.ir", "https://www.aparat.com"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
		if err != nil {
			continue
		}
		resp, err := l.Client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 500 {
			return true
		}
	}
	return false
}

// HealingAction tells the orchestrator what automatic remediation should be applied.
type HealingAction struct {
	SwitchToDoH        bool
	RecommendedMode    model.EngineMode
	ForceTCPFallback   bool
	EnableAntiDPIChain bool
	RefreshFeed        bool
	Reconnect          bool
	CoreUnhealthy      bool
}

// RunDiagnostics executes all 9 steps in parallel where possible and computes
// both human-readable Persian guidance and automatic fixes. The optional runtime
// argument keeps the old signature usable from tests and callers without state.
func RunDiagnostics(ctx context.Context, prober Prober, runtime ...RuntimeContext) (model.DiagnosticsReport, HealingAction) {
	rt := RuntimeContext{}
	if len(runtime) > 0 {
		rt = runtime[0]
	}
	action := HealingAction{RecommendedMode: model.EngineModeAuto}
	// Any live state (connected, connecting, error, or a known node pool) means
	// the caller can evaluate feed and core readiness. Truly stateless callers
	// (the offline mobile fallback) get informational feed/core steps instead.
	runtimeKnown := rt.RuntimeKnown || rt.Connected || rt.State != "" || rt.NodeCount > 0
	steps := make([]model.DiagnosticStep, 0, 9)

	probeCtx, cancel := context.WithTimeout(ctx, 9*time.Second)
	defer cancel()

	var (
		wg       sync.WaitGroup
		hasNet   bool
		skew     float64
		skewErr  error
		poisoned bool
		dnsErr   error
		conflict bool
		udpOK    bool
		iranOK   bool
	)
	wg.Add(6)
	go func() { defer wg.Done(); hasNet = prober.HasDefaultInterface(probeCtx) }()
	go func() { defer wg.Done(); skew, skewErr = prober.CheckClockSkewSeconds(probeCtx) }()
	go func() { defer wg.Done(); poisoned, dnsErr = prober.CheckDNSPoisoned(probeCtx) }()
	go func() { defer wg.Done(); conflict = prober.HasConflictingSystemProxy() }()
	go func() { defer wg.Done(); udpOK = prober.TestUDPReachable(probeCtx) }()
	go func() { defer wg.Done(); iranOK = prober.TestIranDirect(probeCtx) }()
	wg.Wait()

	// 1) Base network interface. Android's VPN process may expose only the
	// tun interface (or report empty flags), so a live VPN session and successful
	// outbound probes are also proof of a working base connection.
	baseOK := hasNet || rt.Connected || skewErr == nil || udpOK || iranOK
	baseDetail := "اتصال شبکه دستگاه فعال است."
	switch {
	case rt.Connected:
		baseDetail = "اینترنت پایه از طریق تونل VPN در دسترس است."
	case !hasNet && baseOK:
		baseDetail = "پروازهای شبکه پاسخ دادند؛ اتصال پایه در دسترس است."
	}
	steps = append(steps, diagnosticStep(
		"interface", "بررسی اتصال اینترنت پایه", baseOK, "critical",
		baseDetail,
		"هیچ کارت شبکه فعالی یافت نشد؛ Wi-Fi یا اینترنت همراه را بررسی کنید.",
		false, "",
	))

	// 2) Clock skew (TLS).
	clockStatus := "ok"
	clockSeverity := "ok"
	clockDetail := "ساعت سیستم دقیق است."
	if skewErr != nil {
		clockStatus, clockSeverity = "info", "info"
		clockDetail = "سرور زمان در دسترس نبود؛ این بررسی در این شبکه انجام نشد و مانع اتصال نیست."
	} else if skew > 120 {
		clockStatus, clockSeverity = "failed", "warning"
		clockDetail = "ساعت دستگاه با زمان جهانی اختلاف دارد؛ برای TLS و گواهی‌ها، تنظیم خودکار ساعت را روشن کنید."
		if skew > 3600 {
			clockSeverity = "critical"
			clockDetail = "اختلاف ساعت دستگاه بیش از یک ساعت است و TLS را خراب می‌کند؛ تنظیم خودکار زمان را فوراً روشن کنید."
		}
	}
	steps = append(steps, model.DiagnosticStep{
		ID: "clock", TitleFA: "هماهنگی ساعت سیستم (TLS)", Status: clockStatus,
		Severity: clockSeverity, DetailFA: clockDetail,
	})

	// 3) DNS poisoning / hijack.
	dnsStep := model.DiagnosticStep{
		ID: "dns", TitleFA: "سلامت DNS و جلوگیری از فیلتر نام دامنه",
		Status: "ok", Severity: "ok",
		DetailFA: "DNS سالم است و پرس‌وجوهای خارجی از DoH رمزنگاری‌شدهٔ داخل تونل عبور می‌کنند.",
	}
	if dnsErr != nil {
		dnsStep.Status, dnsStep.Severity = "info", "info"
		dnsStep.DetailFA = "آزمون DNS محلی کامل نشد؛ تونل همچنان از DoH داخلی استفاده می‌کند."
	} else if poisoned {
		dnsStep.Status, dnsStep.Severity = "failed", "critical"
		dnsStep.DetailFA = "DNS اپراتور مسموم است و دامنه‌های فیلترشده را به آدرس اشتباه هدایت می‌کند. MLZ می‌تواند پرس‌وجوها را به DoH امن منتقل کند."
		dnsStep.CanAutoFix, dnsStep.FixLabelFA = true, "فعال‌سازی DoH و بازسازی اتصال"
		action.SwitchToDoH = true
		action.Reconnect = true
	}
	steps = append(steps, dnsStep)

	// 4) Local proxy/port conflict.
	if conflict {
		steps = append(steps, model.DiagnosticStep{
			ID: "conflict", TitleFA: "بررسی تداخل پورت و پراکسی دیگر",
			Status: "fixed", Severity: "info", AutoApplied: true,
			DetailFA: "پراکسی/متغیر محیطی خارجی شناسایی شد؛ MLZ از پورت محلی اختصاصی خود استفاده می‌کند و تنظیمات شما را تغییر نمی‌دهد.",
		})
	} else {
		steps = append(steps, model.DiagnosticStep{
			ID: "conflict", TitleFA: "بررسی تداخل پورت و پراکسی دیگر",
			Status: "ok", Severity: "ok", DetailFA: "تداخلی در پورت‌های سیستم وجود ندارد.",
		})
	}

	// 5) UDP throttling / DPI check.
	if !udpOK {
		action.ForceTCPFallback = true
		action.EnableAntiDPIChain = true
		action.RecommendedMode = model.EngineModeXrayAntiDPI
		action.Reconnect = true
		steps = append(steps, model.DiagnosticStep{
			ID: "dpi", TitleFA: "تشخیص محدودیت UDP و فیلترینگ DPI اپراتور",
			Status: "failed", Severity: "warning", AutoApplied: false,
			CanAutoFix: true, FixLabelFA: "انتقال خودکار به TCP/Xray مقاوم",
			DetailFA: "محدودیت UDP تشخیص داده شد؛ در صورت اتصال، مسیر مقاوم TCP/Xray به‌طور خودکار انتخاب می‌شود.",
		})
	} else {
		steps = append(steps, model.DiagnosticStep{
			ID: "dpi", TitleFA: "تشخیص محدودیت UDP و فیلترینگ DPI اپراتور",
			Status: "ok", Severity: "ok", DetailFA: "پروتکل‌های پرسرعت UDP و TCP در دسترس هستند.",
		})
	}

	// 6) Iran .ir direct check.
	steps = append(steps, diagnosticStep(
		"iran_direct", "سلامت دسترسی مستقیم به سایت‌های .ir", iranOK, "warning",
		"سایت‌های ایرانی و بانکی به‌صورت مستقیم و بدون افت سرعت باز می‌شوند.",
		"دسترسی مستقیم به شبکه داخلی با اختلال موقت مواجه است؛ مسیریابی .ir همچنان مستقیم می‌ماند.",
		false, "",
	))

	// 7) Signed feed freshness and node pool.
	feedStep := model.DiagnosticStep{
		ID: "feed", TitleFA: "تازگی فید امضاشده و تعداد سرورها",
		Status: "ok", Severity: "ok",
	}
	switch {
	case !runtimeKnown:
		feedStep.Status, feedStep.Severity = "info", "info"
		feedStep.DetailFA = "برای بررسی فید و استخر سرورها ابتدا اتصال VPN را برقرار کنید."
	case rt.NodeCount <= 0:
		feedStep.Status, feedStep.Severity = "failed", "critical"
		feedStep.DetailFA = "هیچ سروری در فید امضاشده وجود ندارد؛ اتصال بدون تازه‌سازی فید ممکن نیست."
		feedStep.CanAutoFix, feedStep.FixLabelFA = true, "دریافت فوری فید و بازسازی استخر"
		action.RefreshFeed = true
	case rt.FeedAge > 30*time.Minute:
		feedStep.Status, feedStep.Severity = "failed", "warning"
		feedStep.DetailFA = "فید سرورها قدیمی‌تر از ۳۰ دقیقه است؛ ممکن است گره‌های جدید از دست رفته باشند."
		feedStep.CanAutoFix, feedStep.FixLabelFA = true, "تازه‌سازی فوری فید"
		action.RefreshFeed = true
	case rt.FeedAge <= 0:
		feedStep.Status, feedStep.Severity = "info", "info"
		feedStep.DetailFA = "زمان آخرین همگام‌سازی فید مشخص نیست؛ هنگام نیاز به‌صورت خودکار تازه می‌شود."
	default:
		feedStep.DetailFA = "فید امضاشده تازه است و سرورهای کافی برای انتخاب بهترین مسیر وجود دارد."
	}
	steps = append(steps, feedStep)

	// 8) Exit IP / tunnel health.
	exitStep := model.DiagnosticStep{
		ID: "exit", TitleFA: "سلامت خروج تونل و آی‌پی نهایی",
		Status: "info", Severity: "info",
		DetailFA: "برای بررسی خروج، ابتدا اتصال را برقرار کنید.",
	}
	if !rt.Connected {
		// A disconnected VPN is a warning, not a broken local network: the
		// offline checks above can still be fully healthy.
		exitStep.Status, exitStep.Severity = "failed", "warning"
		exitStep.DetailFA = "اتصال VPN برقرار نیست؛ برای آزمون خروج و تأخیر واقعی ابتدا وصل شوید."
	} else if rt.Connected {
		if rt.LastIP == nil || strings.TrimSpace(rt.LastIP.PublicIP) == "" {
			exitStep.Status, exitStep.Severity = "failed", "warning"
			exitStep.DetailFA = "اتصال برقرار است ولی آزمون خروج هنوز نتیجهٔ سالمی ندارد؛ کارت آی‌پی را لمس کنید یا خودترمیمی را اجرا کنید."
			exitStep.CanAutoFix, exitStep.FixLabelFA = true, "بازسازی اتصال و آزمون دوبارهٔ خروج"
		} else {
			exitStep.Status, exitStep.Severity = "ok", "ok"
			country := rt.LastIP.CountryName
			if country == "" {
				country = rt.LastIP.CountryCode
			}
			exitStep.DetailFA = "خروج تونل سالم است: " + rt.LastIP.PublicIP
			if country != "" {
				exitStep.DetailFA += " • " + country
			}
			if rt.LastIP.RealDelayMS > 0 {
				exitStep.DetailFA += " • تأخیر واقعی " + itoa(rt.LastIP.RealDelayMS) + "ms"
			}
		}
	}
	steps = append(steps, exitStep)

	// 9) Core readiness / fallback pool.
	coreStep := model.DiagnosticStep{
		ID: "cores", TitleFA: "آماده‌باش هسته‌ها و مسیرهای پشتیبان",
		Status: "ok", Severity: "ok",
	}
	switch {
	case !runtimeKnown:
		coreStep.Status, coreStep.Severity = "info", "info"
		coreStep.DetailFA = "برای بررسی آماده‌باش هسته‌ها و مسیر پشتیبان ابتدا اتصال VPN را برقرار کنید."
	case rt.NodeCount <= 0:
		coreStep.Status, coreStep.Severity = "failed", "critical"
		coreStep.DetailFA = "بدون سرور، هسته‌ها نمی‌توانند مسیری برای اتصال بسازند."
		coreStep.CanAutoFix, coreStep.FixLabelFA = true, "تازه‌سازی فید و تلاش دوباره"
	case rt.State == "error" && strings.TrimSpace(rt.LastErrorFA) != "":
		coreStep.Status, coreStep.Severity = "failed", "critical"
		coreStep.DetailFA = "آخرین تلاش اتصال ناموفق بود: " + trimDetail(rt.LastErrorFA)
		coreStep.CanAutoFix, coreStep.FixLabelFA = true, "تازه‌سازی فید و تلاش دوباره برای اتصال"
	case rt.Connected && rt.ProxyPort > 0 && !localPortOpen(rt.ProxyPort):
		coreStep.Status, coreStep.Severity = "failed", "critical"
		coreStep.DetailFA = "وضعیت اتصال سبز است اما هستهٔ تونل روی پورت محلی پاسخ نمی‌دهد؛ احتمالاً هسته متوقف شده است."
		if strings.TrimSpace(rt.CoreFailure) != "" {
			coreStep.DetailFA += " علت: " + trimDetail(rt.CoreFailure)
		}
		coreStep.CanAutoFix, coreStep.FixLabelFA = true, "راه‌اندازی دوبارهٔ هسته و اتصال"
		action.CoreUnhealthy = true
		action.Reconnect = true
	case rt.Connected && strings.TrimSpace(rt.ActiveCore) == "":
		coreStep.Status, coreStep.Severity = "failed", "warning"
		coreStep.DetailFA = "اتصال برقرار است ولی هستهٔ فعال شناسایی نشد؛ اتصال را بازسازی کنید."
		coreStep.CanAutoFix, coreStep.FixLabelFA = true, "بازسازی اتصال"
	default:
		coreStep.DetailFA = "sing-box و Xray-core آماده تغییر مسیر آنی هستند؛ استخر پشتیبان در دسترس است."
		if rt.NodeCount > 0 {
			coreStep.DetailFA = "sing-box و Xray-core آماده‌اند؛ " + itoa(rt.NodeCount) + " سرور در استخر پشتیبان بررسی شد."
		}
	}
	steps = append(steps, coreStep)

	report := model.DiagnosticsReport{
		Timestamp: time.Now().Unix(),
		Steps:     steps,
	}
	recalculate(&report)
	if !rt.Connected {
		report.HeadlineFA = "اتصال VPN برقرار نیست"
	}
	return report, action
}

func diagnosticStep(id, title string, ok bool, severity, okMsg, failMsg string, canFix bool, fixLabel string) model.DiagnosticStep {
	st := model.DiagnosticStep{
		ID: id, TitleFA: title, Status: "ok", Severity: "ok", DetailFA: okMsg,
	}
	if !ok {
		st.Status, st.Severity, st.DetailFA = "failed", severity, failMsg
		st.CanAutoFix, st.FixLabelFA = canFix, fixLabel
	}
	return st
}

// Recalculate recomputes score, headline, counts, and action flag after the
// orchestrator applies automatic fixes. Callers may mutate step statuses first.
func Recalculate(report *model.DiagnosticsReport) {
	recalculate(report)
}

func recalculate(report *model.DiagnosticsReport) {
	critical, warnings, fixed, actions := 0, 0, 0, 0
	hasInterfaceFailure := false
	for _, s := range report.Steps {
		switch s.Status {
		case "failed":
			actions++
			if s.Severity == "critical" {
				critical++
			} else {
				warnings++
			}
			if s.ID == "interface" {
				hasInterfaceFailure = true
			}
		case "fixed":
			fixed++
		}
	}

	score := 100 - critical*30 - warnings*14 - fixed*3
	if hasInterfaceFailure {
		score = 12
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	report.HealthScore = score
	report.HealthLabelFA = healthLabel(score)
	report.AutoFixCount = fixed
	report.AutoHealed = fixed > 0
	report.NeedsAction = actions > 0

	switch {
	case hasInterfaceFailure:
		report.HeadlineFA = "اتصال اینترنت دستگاه قطع است"
	case stepFailed(report, "dns"):
		report.HeadlineFA = "DNS اپراتور مسموم است"
	case stepFailed(report, "dpi"):
		report.HeadlineFA = "محدودیت UDP/DPI شناسایی شد"
	case stepFailed(report, "feed"):
		report.HeadlineFA = "فید سرورها نیاز به تازه‌سازی دارد"
	case stepFailed(report, "exit"):
		report.HeadlineFA = "خروج تونل هنوز سالم نیست"
	case stepFailed(report, "cores"):
		report.HeadlineFA = "آخرین اتصال ناموفق بود"
	case fixed > 0:
		report.HeadlineFA = "مشکل شناسایی و خودکار اصلاح شد"
	case warnings > 0:
		report.HeadlineFA = "چند مورد نیازمند توجه است"
	default:
		report.HeadlineFA = "همه‌چیز سالم است"
	}

	if report.SummaryFA == "" {
		report.SummaryFA = buildSummary(report)
	}
}

func buildSummary(report *model.DiagnosticsReport) string {
	if !stepFailed(report, "interface") && !stepFailed(report, "dns") && !stepFailed(report, "dpi") &&
		!stepFailed(report, "feed") && !stepFailed(report, "exit") {
		if report.AutoFixCount > 0 {
			return "عیب‌یابی کامل شد؛ موارد قابل اصلاح به‌صورت خودکار ترمیم شدند."
		}
		return "همه بخش‌های شبکه سالم هستند و بهترین مسیر آماده اتصال است."
	}
	parts := make([]string, 0, 3)
	for _, s := range report.Steps {
		if s.Status == "failed" {
			parts = append(parts, s.DetailFA)
			if len(parts) == 2 {
				break
			}
		}
	}
	if len(parts) == 0 {
		return report.HeadlineFA
	}
	return report.HeadlineFA + ": " + strings.Join(parts, " ")
}

func stepFailed(report *model.DiagnosticsReport, id string) bool {
	for _, s := range report.Steps {
		if s.ID == id {
			return s.Status == "failed"
		}
	}
	return false
}

func healthLabel(score int) string {
	switch {
	case score >= 90:
		return "عالی"
	case score >= 75:
		return "خوب"
	case score >= 55:
		return "نیازمند توجه"
	default:
		return "بحرانی"
	}
}

func localPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), 1200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func trimDetail(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= 140 {
		return s
	}
	return string([]rune(s)[:140]) + "…"
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
