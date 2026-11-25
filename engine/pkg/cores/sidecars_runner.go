package cores

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

// BuildSidecarCommand builds CLI flags for launching modular Tier 1-3 sidecars
// bound to a local loopback SOCKS5 endpoint for chaining under sing-box.
func BuildSidecarCommand(kind model.CoreKind, listenAddr, user, pass string, target model.Node) ([]string, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen addr %q: %w", listenAddr, err)
	}

	switch kind {
	case model.CoreByeDPI:
		// ByeDPI (ciadpi): rootless TCP/TLS DPI desync SOCKS5 proxy
		return []string{"--ip", host, "--port", port, "--split", "1", "--disorder", "1+s", "--tlsrec", "1+s"}, nil
	case model.CoreSpoofDPI:
		return []string{"--addr", host, "--port", port, "--enable-doh"}, nil
	case model.CoreNaiveProxy:
		if target.Address == "" {
			return nil, fmt.Errorf("naiveproxy requires target node address")
		}
		upstream := fmt.Sprintf("https://%s:%s@%s:%d", user, pass, target.Address, target.Port)
		return []string{"--listen=socks://" + listenAddr, "--proxy=" + upstream}, nil
	case model.CoreArti:
		return []string{"proxy", "--socks-port", listenAddr}, nil
	case model.CorePsiphon:
		return []string{"-listenInterface", host, "-socksProxyPort", port}, nil
	case model.CoreBrook:
		if target.Address == "" {
			return nil, fmt.Errorf("brook requires target server")
		}
		return []string{"client", "--socks5", listenAddr, "--server", net.JoinHostPort(target.Address, fmt.Sprint(target.Port)), "--password", target.Params["password"]}, nil
	case model.CoreGost:
		args := []string{"-L", fmt.Sprintf("socks5://%s:%s@%s", user, pass, listenAddr)}
		if target.Address != "" {
			args = append(args, "-F", fmt.Sprintf("relay+tls://%s:%d", target.Address, target.Port))
		}
		return args, nil
	case model.CoreWSTunnel:
		if target.Address == "" {
			return nil, fmt.Errorf("wstunnel requires remote wss host")
		}
		return []string{"client", "-L", "socks5://" + listenAddr, fmt.Sprintf("wss://%s:%d", target.Address, target.Port)}, nil
	case model.CoreChisel:
		if target.Address == "" {
			return nil, fmt.Errorf("chisel requires remote server")
		}
		return []string{"client", fmt.Sprintf("https://%s:%d", target.Address, target.Port), fmt.Sprintf("%s:socks", listenAddr)}, nil
	case model.CoreDNSTT:
		domain := target.Params["domain"]
		pubkey := target.Params["pubkey"]
		doh := target.Params["doh"]
		if doh == "" {
			doh = "https://dns.google/dns-query"
		}
		return []string{"-doh", doh, "-pubkey", pubkey, domain, listenAddr}, nil
	case model.CoreMieru:
		return []string{"apply", "socks5", listenAddr}, nil
	case model.CoreJuicity:
		return []string{"run", "-l", listenAddr}, nil
	case model.CoreLyrebird:
		return []string{"-addr", listenAddr}, nil
	case model.CoreCloak:
		return []string{"-l", host, "-p", port}, nil
	case model.CoreTrojanGo:
		return []string{"-client", "-local-addr", host, "-local-port", port}, nil
	case model.CoreLantern:
		return []string{"-addr", listenAddr}, nil
	case model.CoreKcptun:
		if target.Address == "" {
			return nil, fmt.Errorf("kcptun requires target server")
		}
		return []string{"-l", listenAddr, "-r", net.JoinHostPort(target.Address, fmt.Sprint(target.Port)), "-mode", "fast2"}, nil
	case model.CoreGoodbyeDPI:
		return []string{"-9", "--dns-addr", "1.1.1.1", "--dns-port", "853"}, nil
	case model.CoreZapret:
		return []string{"--dpi-desync=fake,split2", "--dpi-desync-ttl=5"}, nil
	case model.CoreUDP2Raw:
		if target.Address == "" {
			return nil, fmt.Errorf("udp2raw requires target server")
		}
		return []string{"-c", "-l", listenAddr, "-r", net.JoinHostPort(target.Address, fmt.Sprint(target.Port)), "--raw-mode", "faketcp"}, nil
	case model.CoreAmneziaWG, model.CoreUsque:
		return []string{"--socks", listenAddr}, nil
	default:
		return nil, fmt.Errorf("no CLI adapter defined for sidecar %s", kind)
	}
}

// StartProcessSidecar resolves an installed Tier 2/3 sidecar binary and launches it on loopback.
func (m *Manager) StartProcessSidecar(ctx context.Context, kind model.CoreKind, target model.Node) (SidecarInstance, error) {
	binPath, err := m.ResolveBinaryPath(kind)
	if err != nil {
		return nil, err
	}
	if binPath == "" {
		return nil, fmt.Errorf("sidecar %s is embedded Go; use Go in-process adapter", kind)
	}

	listenAddr, err := AllocateLoopbackPort()
	if err != nil {
		return nil, err
	}
	user := RandomCredential()
	pass := RandomCredential()

	args, err := BuildSidecarCommand(kind, listenAddr, user, pass, target)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start sidecar %s (%s): %w", kind, strings.Join(args, " "), err)
	}

	// Briefly wait for the loopback listener to open.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", listenAddr, 150*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	inst := &ProcessSidecar{
		kind:  kind,
		socks: listenAddr,
		user:  user,
		pass:  pass,
		cmd:   cmd,
	}

	m.mu.Lock()
	if prev, ok := m.active[kind]; ok {
		_ = prev.Stop()
	}
	m.active[kind] = inst
	m.mu.Unlock()
	return inst, nil
}
