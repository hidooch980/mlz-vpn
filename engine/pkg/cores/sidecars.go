// Package cores coordinates sing-box (primary router/TUN/DNS), Xray-core (companion),
// and the modular All-In Sidecar ecosystem through local SOCKS5 chaining.
package cores

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

// SidecarTier defines how a sidecar core is packaged and started.
type SidecarTier int

const (
	// TierEmbeddedGo runs directly inside mlz-engine / mlz-core.aar (no external binary).
	TierEmbeddedGo SidecarTier = 1
	// TierOnDemandBinary uses SHA-256 verified binaries on Windows or nativeLibraryDir on Android.
	TierOnDemandBinary SidecarTier = 2
	// TierPrivilegedWindows requires elevated Windows privileges (WinDivert / raw sockets) during TUN sessions.
	TierPrivilegedWindows SidecarTier = 3
)

// SidecarDescriptor describes metadata, capabilities, and binary checksums for each modular sidecar.
type SidecarDescriptor struct {
	Kind         model.CoreKind `json:"kind"`
	DisplayName  string         `json:"display_name"`
	Tier         SidecarTier    `json:"tier"`
	Capabilities []string       `json:"capabilities"`
	BinaryBase   string         `json:"binary_base,omitempty"`
	AndroidLib   string         `json:"android_lib,omitempty"`
	DownloadURL  string         `json:"download_url,omitempty"`
	SHA256       string         `json:"sha256,omitempty"`
}

// SidecarRunConfig holds runtime parameters when spinning up a sidecar on loopback.
type SidecarRunConfig struct {
	ListenAddr string
	Username   string
	Password   string
	TargetNode model.Node
	CacheDir   string
	AndroidLib string
}

// SidecarInstance represents a running sidecar bound to a local loopback SOCKS5 endpoint.
type SidecarInstance interface {
	Kind() model.CoreKind
	SocksAddress() string
	Username() string
	Password() string
	Stop() error
}

// AllInCatalog returns the full catalog of supported modular sidecars across Tiers 1-3.
func AllInCatalog() map[model.CoreKind]SidecarDescriptor {
	list := []SidecarDescriptor{
		// Tier 1: Pure-Go Embedded Engines (Android AAR + Windows single binary)
		{Kind: model.CoreAmneziaWG, DisplayName: "AmneziaWG", Tier: TierEmbeddedGo, Capabilities: []string{"wireguard_obfs", "udp", "modern"}},
		{Kind: model.CoreSpoofDPI, DisplayName: "SpoofDPI", Tier: TierEmbeddedGo, Capabilities: []string{"sni_split", "dpi_desync", "bootstrap"}},
		{Kind: model.CoreUsque, DisplayName: "usque (WARP MASQUE)", Tier: TierEmbeddedGo, Capabilities: []string{"http3", "masque", "modern"}},
		{Kind: model.CoreLyrebird, DisplayName: "Lyrebird (obfs4/Snowflake/WebTunnel)", Tier: TierEmbeddedGo, Capabilities: []string{"pluggable_transport", "emergency", "webrtc"}},
		{Kind: model.CoreDNSTT, DisplayName: "dnstt (DNS Tunnel)", Tier: TierEmbeddedGo, Capabilities: []string{"dns_tunnel", "blackout", "dpi_desync"}},
		{Kind: model.CoreCloak, DisplayName: "Cloak", Tier: TierEmbeddedGo, Capabilities: []string{"tls_cover", "anti_probing", "modern"}},
		{Kind: model.CorePsiphon, DisplayName: "Psiphon Tunnel", Tier: TierEmbeddedGo, Capabilities: []string{"emergency", "multi_transport"}},
		{Kind: model.CoreBrook, DisplayName: "Brook", Tier: TierEmbeddedGo, Capabilities: []string{"emergency", "custom_crypto"}},
		{Kind: model.CoreTrojanGo, DisplayName: "Trojan-Go", Tier: TierEmbeddedGo, Capabilities: []string{"trojan_mux", "ws"}},
		{Kind: model.CoreLantern, DisplayName: "Lantern Engine", Tier: TierEmbeddedGo, Capabilities: []string{"emergency", "fronted"}},
		{Kind: model.CoreMieru, DisplayName: "Mieru", Tier: TierEmbeddedGo, Capabilities: []string{"anti_probing", "modern"}},
		{Kind: model.CoreJuicity, DisplayName: "Juicity", Tier: TierEmbeddedGo, Capabilities: []string{"quic", "modern"}},
		{Kind: model.CoreGost, DisplayName: "Gost v3", Tier: TierEmbeddedGo, Capabilities: []string{"relay", "chaining"}},
		{Kind: model.CoreChisel, DisplayName: "Chisel", Tier: TierEmbeddedGo, Capabilities: []string{"http_ws_tunnel", "dpi_desync"}},

		// Tier 2: C/Rust Sidecar Binaries (On-demand Windows SHA-256 / Android nativeLibraryDir .so)
		{Kind: model.CoreByeDPI, DisplayName: "ByeDPI (ciadpi)", Tier: TierOnDemandBinary, Capabilities: []string{"dpi_desync", "bootstrap"}, BinaryBase: "ciadpi", AndroidLib: "libmlz_byedpi.so"},
		{Kind: model.CoreArti, DisplayName: "Arti (Tor Rust)", Tier: TierOnDemandBinary, Capabilities: []string{"tor", "emergency"}, BinaryBase: "arti", AndroidLib: "libmlz_arti.so"},
		{Kind: model.CoreNaiveProxy, DisplayName: "NaiveProxy", Tier: TierOnDemandBinary, Capabilities: []string{"chrome_tls", "modern"}, BinaryBase: "naive", AndroidLib: "libmlz_naive.so"},
		{Kind: model.CoreWSTunnel, DisplayName: "wstunnel", Tier: TierOnDemandBinary, Capabilities: []string{"ws_tunnel", "udp_over_tcp"}, BinaryBase: "wstunnel", AndroidLib: "libmlz_wstunnel.so"},
		{Kind: model.CoreKcptun, DisplayName: "kcptun", Tier: TierOnDemandBinary, Capabilities: []string{"kcp", "lossy_udp"}, BinaryBase: "kcptun", AndroidLib: "libmlz_kcptun.so"},

		// Tier 3: Privileged Windows-Only Sidecars (only active during elevated TUN sessions)
		{Kind: model.CoreGoodbyeDPI, DisplayName: "GoodbyeDPI", Tier: TierPrivilegedWindows, Capabilities: []string{"win_windivert", "dpi_desync"}, BinaryBase: "goodbyedpi"},
		{Kind: model.CoreZapret, DisplayName: "Zapret", Tier: TierPrivilegedWindows, Capabilities: []string{"dpi_desync"}, BinaryBase: "winws"},
		{Kind: model.CoreUDP2Raw, DisplayName: "udp2raw", Tier: TierPrivilegedWindows, Capabilities: []string{"fake_tcp_udp", "dpi_desync"}, BinaryBase: "udp2raw"},
	}
	out := make(map[model.CoreKind]SidecarDescriptor, len(list))
	for _, d := range list {
		out[d.Kind] = d
	}
	return out
}

// Manager tracks active sidecar instances and resolves their executable or embedded paths.
type Manager struct {
	mu         sync.Mutex
	cacheDir   string
	androidLib string
	catalog    map[model.CoreKind]SidecarDescriptor
	active     map[model.CoreKind]SidecarInstance
	httpClient *http.Client
	isElevated bool
}

// NewManager initializes a modular sidecar manager.
func NewManager(cacheDir, androidNativeLibDir string, isElevated bool) *Manager {
	return &Manager{
		cacheDir:   cacheDir,
		androidLib: androidNativeLibDir,
		catalog:    AllInCatalog(),
		active:     make(map[model.CoreKind]SidecarInstance),
		httpClient: &http.Client{Timeout: 45 * time.Second},
		isElevated: isElevated,
	}
}

// Descriptor returns metadata for a given CoreKind.
func (m *Manager) Descriptor(kind model.CoreKind) (SidecarDescriptor, bool) {
	d, ok := m.catalog[kind]
	return d, ok
}

// ResolveBinaryPath locates an already cached/packaged sidecar binary for Tier 2/3 cores.
func (m *Manager) ResolveBinaryPath(kind model.CoreKind) (string, error) {
	desc, ok := m.catalog[kind]
	if !ok {
		return "", fmt.Errorf("unknown sidecar: %s", kind)
	}
	if desc.Tier == TierEmbeddedGo {
		return "", nil
	}
	if desc.Tier == TierPrivilegedWindows && (runtime.GOOS != "windows" || !m.isElevated) {
		return "", fmt.Errorf("sidecar %s requires elevated Windows TUN mode", kind)
	}

	if runtime.GOOS == "android" {
		if m.androidLib == "" || desc.AndroidLib == "" {
			return "", fmt.Errorf("android native library missing for %s", kind)
		}
		p := filepath.Join(m.androidLib, desc.AndroidLib)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("android sidecar %s not packaged at %s: %w", kind, p, err)
		}
		return p, nil
	}

	name := desc.BinaryBase
	if runtime.GOOS == "windows" && name != "" {
		name += ".exe"
	}
	p := filepath.Join(m.cacheDir, "sidecars", name)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("sidecar binary %s not installed at %s: %w", kind, p, err)
	}
	return p, nil
}

// EnsureBinaryDownloaded downloads a Tier 2/3 binary on desktop if missing and verifies SHA-256.
func (m *Manager) EnsureBinaryDownloaded(ctx context.Context, kind model.CoreKind, downloadURL, expectedSHA256 string) (string, error) {
	if runtime.GOOS == "android" {
		return m.ResolveBinaryPath(kind)
	}
	if existing, err := m.ResolveBinaryPath(kind); err == nil {
		return existing, nil
	}
	desc, ok := m.catalog[kind]
	if !ok {
		return "", fmt.Errorf("unknown sidecar: %s", kind)
	}
	if downloadURL == "" || expectedSHA256 == "" {
		return "", errors.New("missing download URL or SHA-256 digest for on-demand sidecar")
	}

	targetDir := filepath.Join(m.cacheDir, "sidecars")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	name := desc.BinaryBase
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	finalPath := filepath.Join(targetDir, name)
	tmpPath := finalPath + ".tmp"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with HTTP %d", resp.StatusCode)
	}

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedSHA256 {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("SHA-256 mismatch for %s: got %s want %s", kind, actualHash, expectedSHA256)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return finalPath, nil
}

// StopAll cleanly shuts down all active sidecar subprocesses/instances.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, inst := range m.active {
		_ = inst.Stop()
		delete(m.active, k)
	}
}

// AllocateLoopbackPort picks a free ephemeral TCP port on 127.0.0.1.
func AllocateLoopbackPort() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr, nil
}

// AllocateLoopbackPortInt is the numeric variant used by generated configs.
func AllocateLoopbackPortInt() (int, error) {
	addr, err := AllocateLoopbackPort()
	if err != nil {
		return 0, err
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(portStr)
}

// RandomCredential generates a short random token for local loopback SOCKS5 auth.
func RandomCredential() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ProcessSidecar wraps an external CLI sidecar binary (Tier 2 / Tier 3).
type ProcessSidecar struct {
	kind  model.CoreKind
	socks string
	user  string
	pass  string
	cmd   *exec.Cmd
}

func (p *ProcessSidecar) Kind() model.CoreKind { return p.kind }
func (p *ProcessSidecar) SocksAddress() string { return p.socks }
func (p *ProcessSidecar) Username() string     { return p.user }
func (p *ProcessSidecar) Password() string     { return p.pass }

func (p *ProcessSidecar) Stop() error {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}
