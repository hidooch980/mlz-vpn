// Package model defines shared domain types across the MLZ VPN orchestrator,
// IPC server, mobile bindings, and Flutter UI.
package model

import "time"

// ConnectionState represents the live lifecycle state of the VPN tunnel.
type ConnectionState string

const (
	StateDisconnected ConnectionState = "disconnected"
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
	StateHealing      ConnectionState = "healing"
	StateError        ConnectionState = "error"
)

// TunnelMode determines how traffic is captured on the host device.
type TunnelMode string

const (
	TunnelModeTUN   TunnelMode = "tun"
	TunnelModeProxy TunnelMode = "proxy"
)

// EngineMode maps the user's drawer selection to core routing strategy.
type EngineMode string

const (
	EngineModeAuto             EngineMode = "auto"
	EngineModeSingBoxFast      EngineMode = "singbox_fast"
	EngineModeXrayAntiDPI      EngineMode = "xray_antidpi"
	EngineModeSidecarModern    EngineMode = "sidecar_modern"
	EngineModeSidecarEmergency EngineMode = "sidecar_emergency"
	EngineModeDPIDesync        EngineMode = "dpi_desync"
	EngineModeBatterySaver     EngineMode = "battery_saver"
)

// CoreKind identifies the underlying proxy or anti-censorship core.
type CoreKind string

const (
	CoreSingBox    CoreKind = "sing-box"
	CoreXray       CoreKind = "xray"
	CoreAmneziaWG  CoreKind = "amneziawg"
	CoreSpoofDPI   CoreKind = "spoofdpi"
	CoreUsque      CoreKind = "usque"
	CoreLyrebird   CoreKind = "lyrebird"
	CoreDNSTT      CoreKind = "dnstt"
	CoreCloak      CoreKind = "cloak"
	CoreByeDPI     CoreKind = "byedpi"
	CoreArti       CoreKind = "arti"
	CoreNaiveProxy CoreKind = "naiveproxy"
	CorePsiphon    CoreKind = "psiphon"
	CoreBrook      CoreKind = "brook"
	CoreTrojanGo   CoreKind = "trojan-go"
	CoreLantern    CoreKind = "lantern"
	CoreMieru      CoreKind = "mieru"
	CoreJuicity    CoreKind = "juicity"
	CoreGost       CoreKind = "gost"
	CoreWSTunnel   CoreKind = "wstunnel"
	CoreChisel     CoreKind = "chisel"
	CoreKcptun     CoreKind = "kcptun"
	CoreUDP2Raw    CoreKind = "udp2raw"
	CoreGoodbyeDPI CoreKind = "goodbyedpi"
	CoreZapret     CoreKind = "zapret"
)

// CountryPreference specifies automatic selection or a target ISO 3166-1 alpha-2 code.
type CountryPreference struct {
	Auto bool   `json:"auto"`
	Code string `json:"code,omitempty"`
}

// Node represents a parsed proxy endpoint from the signed 5-minute feed.
type Node struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Engine    CoreKind          `json:"engine"`
	Protocol  string            `json:"protocol"`
	Address   string            `json:"address"`
	Port      int               `json:"port"`
	Region    string            `json:"region"`
	Tags      []string          `json:"tags,omitempty"`
	Weight    float64           `json:"weight"`
	RawURI    string            `json:"raw_uri,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	RealDelay int               `json:"real_delay_ms,omitempty"`
	Score     float64           `json:"score,omitempty"`
}

// FeedNode is the compact server entry distributed in signed feed.v1.json bundles.
type FeedNode struct {
	ID        string   `json:"id"`
	Engine    CoreKind `json:"engine"`
	Protocol  string   `json:"protocol"`
	Region    string   `json:"region"`
	Tags      []string `json:"tags,omitempty"`
	Weight    float64  `json:"weight"`
	ConfigURI string   `json:"config_uri"`
}

// FeedPayload is the Ed25519-signed 5-minute server configuration bundle.
type FeedPayload struct {
	Schema      string     `json:"schema"`
	Sequence    int64      `json:"sequence"`
	GeneratedAt time.Time  `json:"generated_at"`
	TTLSeconds  int        `json:"ttl_seconds"`
	Mirrors     []string   `json:"mirrors,omitempty"`
	Nodes       []FeedNode `json:"nodes"`
	Signature   string     `json:"signature"`
}

// IPTestResult is returned when the user clicks the IP rectangle to test connectivity.
type IPTestResult struct {
	PublicIP     string `json:"public_ip"`
	CountryCode  string `json:"country_code"`
	CountryName  string `json:"country_name_fa"`
	RealDelayMS  int    `json:"real_delay_ms"`
	DNSProtected bool   `json:"dns_protected"`
	IranDirectOK bool   `json:"iran_direct_ok"`
	TestedAt     int64  `json:"tested_at"`
}

// TrafficStats holds instantaneous throughput and cumulative session usage.
type TrafficStats struct {
	UploadBPS     int64 `json:"upload_bps"`
	DownloadBPS   int64 `json:"download_bps"`
	UploadTotal   int64 `json:"upload_total_bytes"`
	DownloadTotal int64 `json:"download_total_bytes"`
	RealDelayMS   int   `json:"real_delay_ms"`
}

// DiagnosticStep represents one automated check in the 7-step diagnostic pipeline.
type DiagnosticStep struct {
	ID          string `json:"id"`
	TitleFA     string `json:"title_fa"`
	Status      string `json:"status"` // pending | ok | fixed | failed
	DetailFA    string `json:"detail_fa"`
	AutoApplied bool   `json:"auto_applied"`
}

// DiagnosticsReport summarizes the full smart diagnostic run and remediation.
type DiagnosticsReport struct {
	Timestamp int64            `json:"timestamp"`
	SummaryFA string           `json:"summary_fa"`
	Steps     []DiagnosticStep `json:"steps"`
}
