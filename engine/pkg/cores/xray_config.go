package cores

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

// XrayChainOptions builds a headless Xray-core configuration exposing an authenticated
// loopback SOCKS5 inbound so sing-box can chain XHTTP / TLS Fragment / Noise traffic through it.
type XrayChainOptions struct {
	ListenAddr     string
	SocksUser      string
	SocksPass      string
	Node           model.Node
	EnableFragment bool
	FragmentLength string
	FragmentDelay  string
}

// BuildXrayChainConfig emits Xray-core JSON with no TUN interface and anti-DPI dialer options.
func BuildXrayChainConfig(opts XrayChainOptions) ([]byte, error) {
	if opts.Node.Address == "" || opts.Node.Port <= 0 {
		return nil, fmt.Errorf("xray chain requires valid node host and port")
	}
	host, portStr, err := net.SplitHostPort(opts.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid xray listen addr %q: %w", opts.ListenAddr, err)
	}
	listenPort, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	inbound := map[string]any{
		"tag":      "socks-chain-in",
		"listen":   host,
		"port":     listenPort,
		"protocol": "socks",
		"settings": map[string]any{
			"auth": "password",
			"accounts": []map[string]string{
				{"user": opts.SocksUser, "pass": opts.SocksPass},
			},
			"udp": true,
		},
	}

	streamSettings := map[string]any{
		"network": defaultString(opts.Node.Params["type"], "tcp"),
	}
	if opts.Node.Params["security"] == "reality" {
		streamSettings["security"] = "reality"
		streamSettings["realitySettings"] = map[string]any{
			"serverName":  opts.Node.Params["sni"],
			"publicKey":   opts.Node.Params["pbk"],
			"shortId":     opts.Node.Params["sid"],
			"fingerprint": defaultString(opts.Node.Params["fp"], "chrome"),
		}
	} else if opts.Node.Params["sni"] != "" {
		streamSettings["security"] = "tls"
		streamSettings["tlsSettings"] = map[string]any{
			"serverName":  opts.Node.Params["sni"],
			"fingerprint": defaultString(opts.Node.Params["fp"], "chrome"),
		}
	}

	if opts.EnableFragment {
		streamSettings["sockopt"] = map[string]any{
			"dialerProxy": "fragment-out",
		}
	}

	proxyOutbound := map[string]any{
		"tag":      "xray-proxy-out",
		"protocol": defaultString(opts.Node.Protocol, "vless"),
		"settings": map[string]any{
			"vnext": []map[string]any{
				{
					"address": opts.Node.Address,
					"port":    opts.Node.Port,
					"users": []map[string]any{
						{
							"id":         opts.Node.Params["uuid"],
							"encryption": "none",
							"flow":       opts.Node.Params["flow"],
						},
					},
				},
			},
		},
		"streamSettings": streamSettings,
	}

	outbounds := []map[string]any{proxyOutbound}
	if opts.EnableFragment {
		outbounds = append(outbounds, map[string]any{
			"tag":      "fragment-out",
			"protocol": "freedom",
			"settings": map[string]any{
				"fragment": map[string]any{
					"packets":  "tlshello",
					"length":   defaultString(opts.FragmentLength, "10-50"),
					"interval": defaultString(opts.FragmentDelay, "10-30"),
				},
			},
		})
	}

	cfg := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  []map[string]any{inbound},
		"outbounds": outbounds,
	}

	return json.MarshalIndent(cfg, "", "  ")
}
