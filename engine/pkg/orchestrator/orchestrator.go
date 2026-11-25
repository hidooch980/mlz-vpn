// Package orchestrator coordinates the full MLZ VPN connection lifecycle:
// 5-minute signed feed refresh, Real Delay probing, hybrid sing-box + Xray + All-In Sidecar startup,
// self-healing diagnostics, LAN sharing, and Windows/Android tunnel setup.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/cores"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/diagnostics"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/lan"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/probe"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/subscription"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/updater"
)

// CoreRunner abstracts starting/stopping sing-box & Xray runtime instances
// (implemented via libbox/libXray on Android gomobile or managed subprocesses/libs on Windows).
type CoreRunner interface {
	StartSingBox(ctx context.Context, singBoxJSON []byte) error
	StopSingBox() error
	StartXray(ctx context.Context, xrayJSON []byte) error
	StopXray() error
}

// Config configures the orchestrator instance.
type Config struct {
	CurrentBuild    int
	CacheDir        string
	RuleSetDir      string
	AndroidLibDir   string
	IsElevated      bool
	FeedSource      subscription.FeedSourceConfig
	ManifestURLs    []string
	LocalMixedPort  int
	LANSharePort    int
	ClashAPIPort    int
}

// ConnectRequest carries the user's zero-config mode and country selections.
type ConnectRequest struct {
	TunnelMode   model.TunnelMode
	EngineMode   model.EngineMode
	Country      model.CountryPreference
	BatterySaver bool
}

// StatusSnapshot is emitted to Flutter UI / Android Kotlin over IPC/EventChannel.
type StatusSnapshot struct {
	State          model.ConnectionState `json:"state"`
	TunnelMode     model.TunnelMode      `json:"tunnel_mode"`
	EngineMode     model.EngineMode      `json:"engine_mode"`
	Country        model.CountryPreference `json:"country"`
	ActiveCore     model.CoreKind        `json:"active_core"`
	ActiveNodeName string                `json:"active_node_name"`
	ActiveRegion   string                `json:"active_region"`
	Stats          model.TrafficStats    `json:"stats"`
	LastSyncUnix   int64                 `json:"last_sync_unix"`
	LastIPTest     *model.IPTestResult   `json:"last_ip_test,omitempty"`
	LAN            lan.ShareInfo         `json:"lan"`
	ErrorMessageFA string                `json:"error_message_fa,omitempty"`
}

// Orchestrator owns the complete VPN runtime state machine.
type Orchestrator struct {
	mu           sync.RWMutex
	cfg          Config
	fetcher      *subscription.Fetcher
	sidecars     *cores.Manager
	runner       CoreRunner
	dialer       probe.NodeDialer
	prober       diagnostics.Prober

	state           model.ConnectionState
	tunnelMode      model.TunnelMode
	engineMode      model.EngineMode
	country         model.CountryPreference
	batterySaver    bool
	activeNode      *model.Node
	activeCore      model.CoreKind
	lastIPTest      *model.IPTestResult
	lanInfo         lan.ShareInfo
	stats           model.TrafficStats
	errorMsgFA      string
	connectedAt     time.Time
	clashSecret     string
	telemetryCancel context.CancelFunc
}

// New constructs an Orchestrator with defaults and embedded dependencies.
func New(cfg Config, runner CoreRunner) *Orchestrator {
	if cfg.LocalMixedPort == 0 {
		cfg.LocalMixedPort = 10809
	}
	if cfg.LANSharePort == 0 {
		cfg.LANSharePort = 10808
	}
	if cfg.ClashAPIPort == 0 {
		cfg.ClashAPIPort = 9090
	}
	ip, ips := lan.DetectLANAddresses()
	return &Orchestrator{
		cfg:        cfg,
		fetcher:    subscription.NewFetcher(cfg.FeedSource),
		sidecars:   cores.NewManager(cfg.CacheDir, cfg.AndroidLibDir, cfg.IsElevated),
		runner:     runner,
		dialer:     probe.DirectDialer{},
		prober:     diagnostics.NewLiveProber(),
		state:      model.StateDisconnected,
		tunnelMode: model.TunnelModeTUN,
		engineMode: model.EngineModeAuto,
		country:    model.CountryPreference{Auto: true},
		activeCore: model.CoreSingBox,
		lanInfo: lan.ShareInfo{
			Enabled:      false,
			PrimaryIP:    ip,
			CandidateIPs: ips,
			Port:         cfg.LANSharePort,
		},
	}
}

// Snapshot returns a thread-safe copy of the current VPN status for the UI.
func (o *Orchestrator) Snapshot() StatusSnapshot {
	o.mu.RLock()
	defer o.mu.RUnlock()

	_, lastSync := o.fetcher.CachedNodes()
	nodeName := ""
	region := ""
	if o.activeNode != nil {
		nodeName = o.activeNode.Name
		region = o.activeNode.Region
	}
	return StatusSnapshot{
		State:          o.state,
		TunnelMode:     o.tunnelMode,
		EngineMode:     o.engineMode,
		Country:        o.country,
		ActiveCore:     o.activeCore,
		ActiveNodeName: nodeName,
		ActiveRegion:   region,
		Stats:          o.stats,
		LastSyncUnix:   lastSync.Unix(),
		LastIPTest:     o.lastIPTest,
		LAN:            o.lanInfo,
		ErrorMessageFA: o.errorMsgFA,
	}
}

// StartBackgroundSync runs the 5-minute feed refresh loop (backing off to 15m in battery saver).
func (o *Orchestrator) StartBackgroundSync(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = o.SyncFeed(ctx, false)
		}
	}
}

// SyncFeed checks if the 5-minute interval elapsed and updates the standby node pool seamlessly.
func (o *Orchestrator) SyncFeed(ctx context.Context, force bool) (bool, error) {
	o.mu.RLock()
	battery := o.batterySaver
	proxyPort := 0
	if o.state == model.StateConnected {
		proxyPort = o.cfg.LocalMixedPort
	}
	o.mu.RUnlock()

	return o.fetcher.SyncIfDue(ctx, force, battery, proxyPort)
}

// Connect selects the optimal node using Real Delay, starts Xray/Sidecars if needed,
// and launches sing-box as the master TUN/DNS router.
func (o *Orchestrator) Connect(ctx context.Context, req ConnectRequest) error {
	o.mu.Lock()
	o.state = model.StateConnecting
	o.tunnelMode = req.TunnelMode
	o.engineMode = req.EngineMode
	o.country = req.Country
	o.batterySaver = req.BatterySaver
	o.errorMsgFA = ""
	o.mu.Unlock()

	// Refresh cached feed opportunistically if stale, falling back to seeds cleanly.
	_, _ = o.fetcher.SyncIfDue(ctx, false, req.BatterySaver, 0)
	nodes, _ := o.fetcher.CachedNodes()
	if len(nodes) == 0 {
		o.failWith("هیچ سروری در مخزن آفلاین و فید پیدا نشد؛ اتصال اینترنت را بررسی کنید.")
		return errors.New("no candidate nodes")
	}

	ranked := probe.RankNodes(ctx, nodes, req.EngineMode, req.Country, o.dialer)
	if len(ranked) == 0 {
		// Automatic self-healing attempt before failing!
		o.mu.Lock()
		o.state = model.StateHealing
		o.mu.Unlock()
		_, healAction := diagnostics.RunDiagnostics(ctx, o.prober)
		if healAction.RecommendedMode != "" && healAction.RecommendedMode != req.EngineMode {
			ranked = probe.RankNodes(ctx, nodes, healAction.RecommendedMode, req.Country, o.dialer)
		}
		if len(ranked) == 0 {
			o.failWith("سرور مناسبی پاسخ نداد؛ لطفاً بخش عیب‌یابی هوشمند را اجرا کنید.")
			return errors.New("all candidate nodes unreachable")
		}
	}

	chosen := ranked[0]
	if err := o.startCoresForNode(ctx, chosen.Node, req); err != nil {
		o.failWith(fmt.Sprintf("خطا در راه‌اندازی هسته: %v", err))
		return err
	}

	if req.TunnelMode == model.TunnelModeProxy {
		_ = ApplySystemProxy(o.cfg.LocalMixedPort)
	}

	o.mu.Lock()
	o.state = model.StateConnected
	o.activeNode = &chosen.Node
	o.activeCore = chosen.Node.Engine
	o.stats.RealDelayMS = chosen.RealDelayMS
	o.connectedAt = time.Now()
	if o.telemetryCancel != nil {
		o.telemetryCancel()
	}
	telCtx, telCancel := context.WithCancel(context.Background())
	o.telemetryCancel = telCancel
	secret := o.clashSecret
	clashPort := o.cfg.ClashAPIPort
	o.mu.Unlock()

	if clashPort > 0 {
		go o.streamClashTelemetry(telCtx, clashPort, secret)
	}
	return nil
}

func (o *Orchestrator) streamClashTelemetry(ctx context.Context, port int, secret string) {
	cc := cores.NewClashClient(fmt.Sprintf("http://127.0.0.1:%d", port), secret)
	for {
		if ctx.Err() != nil {
			return
		}
		_ = cc.StreamTraffic(ctx, func(upBPS, downBPS int64) {
			o.mu.Lock()
			o.stats.UploadBPS = upBPS
			o.stats.DownloadBPS = downBPS
			o.stats.UploadTotal += upBPS
			o.stats.DownloadTotal += downBPS
			o.mu.Unlock()
		})
		select {
		case <-ctx.Done():
			return
		case <-time.After(1500 * time.Millisecond):
		}
	}
}

func (o *Orchestrator) startCoresForNode(ctx context.Context, node model.Node, req ConnectRequest) error {
	if o.runner == nil {
		return nil
	}
	_ = o.runner.StopSingBox()
	_ = o.runner.StopXray()
	o.sidecars.StopAll()

	upstreamSocks := ""
	upstreamUser := ""
	upstreamPass := ""

	useSidecar := req.EngineMode == model.EngineModeSidecarModern ||
		req.EngineMode == model.EngineModeSidecarEmergency ||
		req.EngineMode == model.EngineModeDPIDesync ||
		(node.Engine != "" && node.Engine != model.CoreSingBox && node.Engine != model.CoreXray)

	if useSidecar && node.Engine != "" && node.Engine != model.CoreSingBox && node.Engine != model.CoreXray {
		if inst, err := o.sidecars.StartProcessSidecar(ctx, node.Engine, node); err == nil {
			upstreamSocks = inst.SocksAddress()
			upstreamUser = inst.Username()
			upstreamPass = inst.Password()
		}
	}

	useXray := upstreamSocks == "" && (req.EngineMode == model.EngineModeXrayAntiDPI || node.Engine == model.CoreXray)
	if useXray {
		addr, err := cores.AllocateLoopbackPort()
		if err != nil {
			return err
		}
		user := cores.RandomCredential()
		pass := cores.RandomCredential()
		xrayJSON, err := cores.BuildXrayChainConfig(cores.XrayChainOptions{
			ListenAddr:     addr,
			SocksUser:      user,
			SocksPass:      pass,
			Node:           node,
			EnableFragment: true,
		})
		if err != nil {
			return err
		}
		if err := o.runner.StartXray(ctx, xrayJSON); err != nil {
			return err
		}
		upstreamSocks = addr
		upstreamUser = user
		upstreamPass = pass
	}

	o.mu.RLock()
	lanEnabled := o.lanInfo.Enabled
	lanPort := o.lanInfo.Port
	lanUser := o.lanInfo.Username
	lanPin := o.lanInfo.PIN
	o.mu.RUnlock()

	if lanEnabled && req.TunnelMode == model.TunnelModeTUN {
		_ = lan.OpenFirewallPort(lanPort)
	}

	clashSecret := cores.RandomCredential()
	o.mu.Lock()
	o.clashSecret = clashSecret
	o.mu.Unlock()

	sbJSON, err := cores.BuildSingBoxConfig(cores.SingBoxBuildOptions{
		TunnelMode:       req.TunnelMode,
		LocalMixedPort:   o.cfg.LocalMixedPort,
		EnableLANShare:   lanEnabled,
		LANSharePort:     lanPort,
		LANUsername:      lanUser,
		LANPassword:      lanPin,
		ClashAPIPort:     o.cfg.ClashAPIPort,
		ClashSecret:      clashSecret,
		RuleSetDir:       o.cfg.RuleSetDir,
		UpstreamSocks:    upstreamSocks,
		UpstreamUser:     upstreamUser,
		UpstreamPassword: upstreamPass,
		PrimaryNode:      node,
		AutoInterface:    true,
	})
	if err != nil {
		return err
	}

	// Persist active config in cache for debug/diagnostics visibility.
	if o.cfg.CacheDir != "" {
		_ = os.MkdirAll(o.cfg.CacheDir, 0o755)
		_ = os.WriteFile(filepath.Join(o.cfg.CacheDir, "active-singbox.json"), sbJSON, 0o600)
	}
	return o.runner.StartSingBox(ctx, sbJSON)
}

// Disconnect cleanly tears down cores, temporary Windows firewall rules, and system proxy.
func (o *Orchestrator) Disconnect() error {
	o.mu.Lock()
	if o.telemetryCancel != nil {
		o.telemetryCancel()
		o.telemetryCancel = nil
	}
	o.mu.Unlock()

	if o.runner != nil {
		_ = o.runner.StopSingBox()
		_ = o.runner.StopXray()
	}
	o.sidecars.StopAll()
	_ = ClearSystemProxy()
	_ = lan.CloseFirewallPort()

	o.mu.Lock()
	o.state = model.StateDisconnected
	o.activeNode = nil
	o.stats.UploadBPS = 0
	o.stats.DownloadBPS = 0
	o.mu.Unlock()
	return nil
}

// TestIPHealth runs the live IP, latency, DNS, and .ir check triggered by clicking the IP box.
func (o *Orchestrator) TestIPHealth(ctx context.Context) (model.IPTestResult, error) {
	o.mu.RLock()
	port := 0
	if o.state == model.StateConnected {
		port = o.cfg.LocalMixedPort
	}
	o.mu.RUnlock()

	res, err := probe.RunIPHealthCheck(ctx, port)
	if err != nil {
		return model.IPTestResult{}, err
	}
	o.mu.Lock()
	o.lastIPTest = &res
	o.stats.RealDelayMS = res.RealDelayMS
	o.mu.Unlock()
	return res, nil
}

// RunSmartDiagnostics runs the 7-step diagnostic suite and returns human-friendly Persian output.
func (o *Orchestrator) RunSmartDiagnostics(ctx context.Context) model.DiagnosticsReport {
	report, _ := diagnostics.RunDiagnostics(ctx, o.prober)
	return report
}

// LANInfo returns the current LAN/Hotspot proxy sharing configuration.
func (o *Orchestrator) LANInfo() lan.ShareInfo {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.lanInfo
}

// SetLANSharing toggles hotspot/LAN proxy sharing and opens/closes Windows Firewall cleanly.
func (o *Orchestrator) SetLANSharing(_ context.Context, enabled, usePIN bool) (lan.ShareInfo, error) {
	ip, ips := lan.DetectLANAddresses()
	pin := ""
	user := ""
	if usePIN {
		user = "mlz"
		pin = lan.GeneratePIN()
	}

	fwManaged := false
	if enabled {
		if err := lan.OpenFirewallPort(o.cfg.LANSharePort); err == nil {
			fwManaged = true
		}
	} else {
		_ = lan.CloseFirewallPort()
	}

	o.mu.Lock()
	o.lanInfo = lan.ShareInfo{
		Enabled:         enabled,
		PrimaryIP:       ip,
		CandidateIPs:    ips,
		Port:            o.cfg.LANSharePort,
		Username:        user,
		PIN:             pin,
		FirewallManaged: fwManaged,
	}
	info := o.lanInfo
	o.mu.Unlock()
	return info, nil
}

// CheckAppUpdate checks GitHub Releases / CDN mirrors for newer builds.
func (o *Orchestrator) CheckAppUpdate(ctx context.Context) (updater.CheckResult, error) {
	o.mu.RLock()
	port := 0
	if o.state == model.StateConnected {
		port = o.cfg.LocalMixedPort
	}
	o.mu.RUnlock()
	return updater.CheckForUpdate(ctx, o.cfg.CurrentBuild, o.cfg.ManifestURLs, port)
}

// DownloadAppUpdate fetches and SHA-256 verifies a release artifact into CacheDir/updates.
func (o *Orchestrator) DownloadAppUpdate(ctx context.Context, asset updater.AssetInfo, fileName string) (string, error) {
	o.mu.RLock()
	port := 0
	if o.state == model.StateConnected {
		port = o.cfg.LocalMixedPort
	}
	cacheDir := o.cfg.CacheDir
	o.mu.RUnlock()

	if cacheDir == "" {
		cacheDir = os.TempDir()
	}
	updDir := filepath.Join(cacheDir, "updates")
	if err := os.MkdirAll(updDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(updDir, filepath.Base(fileName))
	if err := updater.DownloadAndVerify(ctx, asset, dest, port); err != nil {
		return "", err
	}
	return dest, nil
}

func (o *Orchestrator) failWith(msgFA string) {
	o.mu.Lock()
	o.state = model.StateError
	o.errorMsgFA = msgFA
	o.mu.Unlock()
}

// UpdateTrafficStats updates live upload/download speeds and totals from Clash API telemetry.
func (o *Orchestrator) UpdateTrafficStats(upBPS, downBPS, upTotal, downTotal int64) {
	o.mu.Lock()
	o.stats.UploadBPS = upBPS
	o.stats.DownloadBPS = downBPS
	if upTotal >= o.stats.UploadTotal {
		o.stats.UploadTotal = upTotal
	}
	if downTotal >= o.stats.DownloadTotal {
		o.stats.DownloadTotal = downTotal
	}
	o.mu.Unlock()
}
