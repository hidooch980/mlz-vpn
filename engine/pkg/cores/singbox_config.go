package cores

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/routing"
)

// SingBoxBuildOptions controls the unified sing-box master configuration.
type SingBoxBuildOptions struct {
	TunnelMode       model.TunnelMode
	LocalMixedPort   int
	EnableLANShare   bool
	LANSharePort     int
	LANUsername      string
	LANPassword      string
	ClashAPIPort     int
	ClashSecret      string
	RuleSetDir       string
	UpstreamSocks    string // If non-empty, sing-box chains proxy traffic to Xray or a Sidecar at this 127.0.0.1:port
	UpstreamUser     string
	UpstreamPassword string
	PrimaryNode      model.Node
	AutoInterface    bool
	TUNMTU           int
}

// BuildSingBoxConfig produces a full sing-box JSON configuration where sing-box owns
// TUN/Proxy inbounds, Split-DNS, .ir direct routing, LAN sharing, and Clash stats API.
func BuildSingBoxConfig(opts SingBoxBuildOptions) ([]byte, error) {
	if opts.LocalMixedPort == 0 {
		opts.LocalMixedPort = 10809
	}
	if opts.LANSharePort == 0 {
		opts.LANSharePort = 10808
	}
	if opts.TUNMTU == 0 {
		opts.TUNMTU = 1360
	}

	proxyTag := "proxy-out"
	inbounds := []map[string]any{
		{
			"type":        "mixed",
			"tag":         "mixed-local",
			"listen":      "127.0.0.1",
			"listen_port": opts.LocalMixedPort,
		},
	}

	if opts.TunnelMode == model.TunnelModeTUN {
		inbounds = append(inbounds, map[string]any{
			"type":          "tun",
			"tag":           "tun-in",
			"interface_name": "mlz0",
			"address":       []string{"172.19.0.1/30"},
			"mtu":           opts.TUNMTU,
			"auto_route":    true,
			"strict_route":  true,
			"stack":         "mixed",
			"sniff":         true,
		})
	}

	if opts.EnableLANShare {
		lanInbound := map[string]any{
			"type":        "mixed",
			"tag":         "mixed-lan",
			"listen":      "0.0.0.0",
			"listen_port": opts.LANSharePort,
		}
		if opts.LANUsername != "" && opts.LANPassword != "" {
			lanInbound["users"] = []map[string]string{
				{
					"username": opts.LANUsername,
					"password": opts.LANPassword,
				},
			}
		}
		inbounds = append(inbounds, lanInbound)
	}

	proxyOutbound, err := buildSingBoxOutbound(proxyTag, opts)
	if err != nil {
		return nil, err
	}

	outbounds := []map[string]any{
		proxyOutbound,
		{"type": "direct", "tag": "direct"},
		{"type": "block", "tag": "block"},
		{"type": "dns", "tag": "dns-out"},
	}

	ruleSets := []map[string]any{
		{
			"tag":    "geosite-ir",
			"type":   "local",
			"format": "binary",
			"path":   filepath.ToSlash(filepath.Join(opts.RuleSetDir, "geosite-ir.srs")),
		},
		{
			"tag":    "geoip-ir",
			"type":   "local",
			"format": "binary",
			"path":   filepath.ToSlash(filepath.Join(opts.RuleSetDir, "geoip-ir.srs")),
		},
	}

	route := map[string]any{
		"auto_detect_interface": opts.AutoInterface,
		"final":                 proxyTag,
		"rule_set":              ruleSets,
		"rules":                 routing.BuildIranRouteRules(proxyTag),
	}

	experimental := map[string]any{}
	if opts.ClashAPIPort > 0 {
		experimental["clash_api"] = map[string]any{
			"external_controller": fmt.Sprintf("127.0.0.1:%d", opts.ClashAPIPort),
			"secret":              opts.ClashSecret,
		}
	}

	cfg := map[string]any{
		"log":          map[string]any{"level": "warn", "timestamp": true},
		"dns":          routing.BuildIranSplitDNS(proxyTag),
		"inbounds":     inbounds,
		"outbounds":    outbounds,
		"route":        route,
		"experimental": experimental,
	}

	return json.MarshalIndent(cfg, "", "  ")
}

func buildSingBoxOutbound(tag string, opts SingBoxBuildOptions) (map[string]any, error) {
	if opts.UpstreamSocks != "" {
		host, portStr, err := net.SplitHostPort(opts.UpstreamSocks)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream socks address %q: %w", opts.UpstreamSocks, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, err
		}
		ob := map[string]any{
			"type":        "socks",
			"tag":         tag,
			"server":      host,
			"server_port": port,
			"version":     "5",
		}
		if opts.UpstreamUser != "" {
			ob["username"] = opts.UpstreamUser
			ob["password"] = opts.UpstreamPassword
		}
		return ob, nil
	}

	n := opts.PrimaryNode
	if n.Address == "" || n.Port == 0 {
		return nil, fmt.Errorf("missing node server address or port")
	}

	switch strings.ToLower(n.Protocol) {
	case "hysteria2":
		ob := map[string]any{
			"type":        "hysteria2",
			"tag":         tag,
			"server":      n.Address,
			"server_port": n.Port,
			"password":    n.Params["password"],
		}
		if sni := n.Params["sni"]; sni != "" {
			ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
		}
		return ob, nil
	case "vless":
		ob := map[string]any{
			"type":        "vless",
			"tag":         tag,
			"server":      n.Address,
			"server_port": n.Port,
			"uuid":        n.Params["uuid"],
		}
		if flow := n.Params["flow"]; flow != "" {
			ob["flow"] = flow
		}
		if sni := n.Params["sni"]; sni != "" {
			ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
		}
		return ob, nil
	case "vmess":
		return map[string]any{
			"type":        "vmess",
			"tag":         tag,
			"server":      n.Address,
			"server_port": n.Port,
			"uuid":        n.Params["uuid"],
			"security":    "auto",
		}, nil
	case "trojan":
		ob := map[string]any{
			"type":        "trojan",
			"tag":         tag,
			"server":      n.Address,
			"server_port": n.Port,
			"password":    n.Params["password"],
		}
		if sni := n.Params["sni"]; sni != "" {
			ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
		}
		return ob, nil
	case "tuic":
		return map[string]any{
			"type":        "tuic",
			"tag":         tag,
			"server":      n.Address,
			"server_port": n.Port,
			"uuid":        n.Params["uuid"],
			"password":    n.Params["password"],
			"tls":         map[string]any{"enabled": true, "server_name": n.Params["sni"]},
		}, nil
	default:
		return map[string]any{
			"type":        "shadowsocks",
			"tag":         tag,
			"server":      n.Address,
			"server_port": n.Port,
			"method":      defaultString(n.Params["method"], "aes-256-gcm"),
			"password":    n.Params["password"],
		}, nil
	}
}

func defaultString(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}
