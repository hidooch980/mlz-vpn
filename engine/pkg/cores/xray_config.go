package cores

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

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
	if opts.SocksUser == "" || opts.SocksPass == "" {
		return nil, fmt.Errorf("xray chain requires socks credentials")
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
		"sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}},
	}

	streamSettings, err := buildXrayStreamSettings(opts.Node.Params, opts.Node.Protocol)
	if err != nil {
		return nil, err
	}
	if opts.EnableFragment {
		streamSettings["sockopt"] = map[string]any{"dialerProxy": "fragment-out"}
	}

	settings, err := buildXrayOutboundSettings(opts.Node)
	if err != nil {
		return nil, err
	}

	proxyOutbound := map[string]any{
		"tag":            "xray-proxy-out",
		"protocol":       strings.ToLower(opts.Node.Protocol),
		"settings":       settings,
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

func buildXrayOutboundSettings(node model.Node) (map[string]any, error) {
	p := node.Params
	switch strings.ToLower(node.Protocol) {
	case "vless":
		if p["uuid"] == "" {
			return nil, fmt.Errorf("vless node is missing uuid")
		}
		user := map[string]any{
			"id":         p["uuid"],
			"encryption": defaultString(p["encryption"], "none"),
			"level":      0,
		}
		if flow := p["flow"]; flow != "" {
			user["flow"] = flow
		}
		return map[string]any{
			"vnext": []map[string]any{
				{
					"address": node.Address,
					"port":    node.Port,
					"users":   []map[string]any{user},
				},
			},
		}, nil
	case "vmess":
		if p["uuid"] == "" {
			return nil, fmt.Errorf("vmess node is missing uuid")
		}
		alter, _ := strconv.Atoi(defaultString(p["alterid"], "0"))
		return map[string]any{
			"vnext": []map[string]any{
				{
					"address": node.Address,
					"port":    node.Port,
					"users": []map[string]any{
						{
							"id":       p["uuid"],
							"alterId":  alter,
							"security": defaultString(p["security"], "auto"),
							"level":    0,
						},
					},
				},
			},
		}, nil
	case "trojan":
		if p["password"] == "" {
			return nil, fmt.Errorf("trojan node is missing password")
		}
		return map[string]any{
			"servers": []map[string]any{
				{
					"address":  node.Address,
					"port":     node.Port,
					"password": p["password"],
					"level":    0,
				},
			},
		}, nil
	case "shadowsocks", "ss":
		if p["method"] == "" || p["password"] == "" {
			return nil, fmt.Errorf("shadowsocks node is missing method or password")
		}
		return map[string]any{
			"servers": []map[string]any{
				{
					"address":  node.Address,
					"port":     node.Port,
					"method":   p["method"],
					"password": p["password"],
					"level":    0,
				},
			},
		}, nil
	case "socks":
		server := map[string]any{"address": node.Address, "port": node.Port}
		if user := p["username"]; user != "" {
			server["users"] = []map[string]string{{"user": user, "pass": p["password"]}}
		}
		return map[string]any{"servers": []map[string]any{server}}, nil
	case "http":
		server := map[string]any{"address": node.Address, "port": node.Port}
		if user := p["username"]; user != "" {
			server["users"] = []map[string]string{{"user": user, "pass": p["password"]}}
		}
		return map[string]any{"servers": []map[string]any{server}}, nil
	default:
		return nil, fmt.Errorf("protocol %q is not supported by Xray-core", node.Protocol)
	}
}

func buildXrayStreamSettings(p map[string]string, protocol string) (map[string]any, error) {
	network := strings.ToLower(firstNonEmpty(p["type"], p["net"]))
	if network == "" {
		network = "tcp"
	}
	if network == "splithttp" {
		network = "xhttp"
	}
	stream := map[string]any{"network": network}

	security := strings.ToLower(p["security"])
	if protocol == "vmess" {
		if tls := strings.ToLower(p["tls"]); tls != "" {
			security = tls
		} else if security != "tls" && security != "reality" {
			security = ""
		}
	}
	if protocol == "trojan" && security == "" {
		security = "tls"
	}
	switch security {
	case "tls":
		tlsSettings := map[string]any{}
		if sni := firstNonEmpty(p["sni"], p["host"]); sni != "" {
			tlsSettings["serverName"] = sni
		}
		if truthy(p["insecure"]) || truthy(p["allowinsecure"]) {
			tlsSettings["allowInsecure"] = true
		}
		if fp := firstNonEmpty(p["fp"], p["fingerprint"]); fp != "" {
			tlsSettings["fingerprint"] = fp
		}
		if alpn := p["alpn"]; alpn != "" {
			tlsSettings["alpn"] = splitList(alpn)
		}
		stream["security"] = "tls"
		stream["tlsSettings"] = tlsSettings
	case "reality":
		if p["pbk"] == "" {
			return nil, fmt.Errorf("reality node is missing public key")
		}
		reality := map[string]any{
			"serverName":  firstNonEmpty(p["sni"], p["host"]),
			"publicKey":   p["pbk"],
			"fingerprint": defaultString(firstNonEmpty(p["fp"], p["fingerprint"]), "chrome"),
		}
		if sid := p["sid"]; sid != "" {
			reality["shortId"] = sid
		}
		if spx := p["spx"]; spx != "" {
			reality["spiderX"] = spx
		}
		stream["security"] = "reality"
		stream["realitySettings"] = reality
	}

	switch network {
	case "tcp":
		if strings.EqualFold(p["headertype"], "http") {
			request := map[string]any{"path": []string{defaultString(p["path"], "/")}}
			if host := p["host"]; host != "" {
				request["headers"] = map[string]any{"Host": []string{host}}
			}
			stream["tcpSettings"] = map[string]any{
				"header": map[string]any{"type": "http", "request": request},
			}
		}
	case "ws", "websocket":
		ws := map[string]any{"path": defaultString(p["path"], "/")}
		if host := p["host"]; host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		stream["network"] = "ws"
		stream["wsSettings"] = ws
	case "grpc":
		grpc := map[string]any{"serviceName": firstNonEmpty(p["servicename"], strings.TrimPrefix(p["path"], "/"))}
		if strings.EqualFold(p["mode"], "multi") {
			grpc["multiMode"] = true
		}
		stream["grpcSettings"] = grpc
	case "httpupgrade":
		hu := map[string]any{"path": defaultString(p["path"], "/")}
		if host := p["host"]; host != "" {
			hu["host"] = host
		}
		stream["httpupgradeSettings"] = hu
	case "xhttp":
		xhttp := map[string]any{"path": defaultString(p["path"], "/"), "mode": defaultString(p["mode"], "auto")}
		if host := p["host"]; host != "" {
			xhttp["host"] = host
		}
		stream["xhttpSettings"] = xhttp
	case "http", "h2":
		if network == "h2" {
			stream["network"] = "http"
		}
		httpSettings := map[string]any{"path": defaultString(p["path"], "/")}
		if host := p["host"]; host != "" {
			httpSettings["host"] = splitList(host)
		}
		stream["httpSettings"] = httpSettings
	case "kcp":
		kcp := map[string]any{}
		if seed := p["seed"]; seed != "" {
			kcp["seed"] = seed
		}
		if header := p["headertype"]; header != "" {
			kcp["header"] = map[string]any{"type": header}
		}
		stream["kcpSettings"] = kcp
	case "quic":
		quic := map[string]any{}
		if security := p["quicsecurity"]; security != "" {
			quic["security"] = security
		}
		if key := p["key"]; key != "" {
			quic["key"] = key
		}
		if header := p["headertype"]; header != "" {
			quic["header"] = map[string]any{"type": header}
		}
		stream["quicSettings"] = quic
	default:
		return nil, fmt.Errorf("unsupported Xray transport %q", network)
	}

	return stream, nil
}
