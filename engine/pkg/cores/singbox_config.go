package cores

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
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
	UpstreamSocks    string
	UpstreamUser     string
	UpstreamPassword string
	PrimaryNode      model.Node
	AutoInterface    bool
	Android          bool
	EnableIPv6       bool
	TUNMTU           int
}

// BuildSingBoxConfig produces a full sing-box 1.12+ JSON configuration where sing-box
// owns TUN/Proxy inbounds, Split-DNS, .ir direct routing, LAN sharing, and Clash stats API.
func BuildSingBoxConfig(opts SingBoxBuildOptions) ([]byte, error) {
	if runtime.GOOS == "android" {
		opts.Android = true
	}
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
		tun := map[string]any{
			"type":    "tun",
			"tag":     "tun-in",
			"address": []string{"172.19.0.1/30"},
			"mtu":     opts.TUNMTU,
			"stack":   "mixed",
		}
		if opts.Android {
			tun["address"] = []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"}
			tun["auto_route"] = false
			tun["strict_route"] = false
			tun["stack"] = "gvisor"
			tun["dns_mode"] = "hijack"
		} else {
			tun["interface_name"] = "mlz0"
			tun["auto_route"] = true
			tun["strict_route"] = true
			if opts.EnableIPv6 {
				tun["address"] = []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"}
			}
		}
		inbounds = append(inbounds, tun)
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
		{"type": "direct", "tag": "direct", "domain_resolver": "dns-iran-direct"},
	}

	available := availableRuleSets(opts.RuleSetDir)
	ruleSets := make([]map[string]any, 0, 2)
	if available["geosite-ir"] {
		ruleSets = append(ruleSets, localRuleSet("geosite-ir", opts.RuleSetDir, "geosite-ir.srs"))
	}
	if available["geoip-ir"] {
		ruleSets = append(ruleSets, localRuleSet("geoip-ir", opts.RuleSetDir, "geoip-ir.srs"))
	}

	dnsConfig := routing.BuildIranSplitDNS(proxyTag)
	dnsConfig.Rules = filterDNSRules(dnsConfig.Rules, available)

	routeRules := make([]any, 0, 8)
	if opts.TunnelMode == model.TunnelModeTUN {
		// Many public upstreams carry TCP but silently drop UDP/443. If QUIC is
		// allowed through TUN, Chrome/Edge latch onto it and show ERR_QUIC_*
		// without falling back. Reject QUIC on the tunnel inbound so browsers
		// immediately use TCP; the mixed proxy inbound keeps QUIC for clients
		// that can use it.
		routeRules = append(routeRules, map[string]any{
			"action":  "reject",
			"inbound": []string{"tun-in"},
			"network": []string{"udp"},
			"port":    []int{443},
		})
	}
	for _, rule := range filterRouteRules(routing.BuildIranRouteRules(proxyTag), available) {
		routeRules = append(routeRules, rule)
	}
	route := map[string]any{
		"final":                   proxyTag,
		"rule_set":                ruleSets,
		"rules":                   routeRules,
		"default_domain_resolver": map[string]any{"server": "dns-remote", "strategy": "prefer_ipv4"},
	}
	if opts.Android {
		route["auto_detect_interface"] = false
	} else {
		route["auto_detect_interface"] = opts.AutoInterface
	}

	experimental := map[string]any{}
	if opts.ClashAPIPort > 0 {
		experimental["clash_api"] = map[string]any{
			"external_controller": fmt.Sprintf("127.0.0.1:%d", opts.ClashAPIPort),
			"secret":              opts.ClashSecret,
			"default_mode":        "rule",
		}
	}

	cfg := map[string]any{
		"log":          map[string]any{"level": "warn", "timestamp": true},
		"dns":          dnsConfig,
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

	return buildSingBoxNodeOutbound(tag, opts.PrimaryNode)
}

func buildSingBoxNodeOutbound(tag string, n model.Node) (map[string]any, error) {
	if n.Address == "" || n.Port == 0 {
		return nil, fmt.Errorf("missing node server address or port")
	}
	p := n.Params
	ob := map[string]any{
		"tag":             tag,
		"server":          n.Address,
		"server_port":     n.Port,
		"domain_resolver": "dns-iran-direct",
	}

	switch strings.ToLower(n.Protocol) {
	case "vless":
		if p["uuid"] == "" {
			return nil, fmt.Errorf("vless node is missing uuid")
		}
		ob["type"] = "vless"
		ob["uuid"] = p["uuid"]
		if flow := p["flow"]; flow != "" {
			ob["flow"] = flow
		}
		if pe := p["packet_encoding"]; pe != "" {
			ob["packet_encoding"] = pe
		}
		if err := applyTLSAndTransport(ob, n.Protocol, p); err != nil {
			return nil, err
		}
	case "vmess":
		if p["uuid"] == "" {
			return nil, fmt.Errorf("vmess node is missing uuid")
		}
		ob["type"] = "vmess"
		ob["uuid"] = p["uuid"]
		ob["security"] = defaultString(p["security"], "auto")
		if alter, err := strconv.Atoi(defaultString(p["alterid"], "0")); err == nil && alter > 0 {
			ob["alter_id"] = alter
		}
		if pe := p["packet_encoding"]; pe != "" {
			ob["packet_encoding"] = pe
		}
		if err := applyTLSAndTransport(ob, n.Protocol, p); err != nil {
			return nil, err
		}
	case "trojan":
		if p["password"] == "" {
			return nil, fmt.Errorf("trojan node is missing password")
		}
		ob["type"] = "trojan"
		ob["password"] = p["password"]
		if err := applyTLSAndTransport(ob, n.Protocol, p); err != nil {
			return nil, err
		}
	case "hysteria2", "hy2":
		if p["password"] == "" {
			return nil, fmt.Errorf("hysteria2 node is missing password")
		}
		ob["type"] = "hysteria2"
		ob["password"] = p["password"]
		if obfs := p["obfs"]; obfs != "" && obfs != "none" {
			ob["obfs"] = map[string]any{"type": obfs, "password": p["obfs-password"]}
		}
		if ports := p["server_ports"]; ports != "" {
			ob["server_ports"] = strings.Split(ports, ",")
		}
		tls, err := buildTLS("hysteria2", p, n.Address)
		if err != nil {
			return nil, err
		}
		ob["tls"] = tls
	case "tuic":
		if p["uuid"] == "" || p["password"] == "" {
			return nil, fmt.Errorf("tuic node is missing uuid or password")
		}
		ob["type"] = "tuic"
		ob["uuid"] = p["uuid"]
		ob["password"] = p["password"]
		if cc := p["congestion_control"]; cc != "" {
			ob["congestion_control"] = cc
		}
		if mode := p["udp_relay_mode"]; mode != "" {
			ob["udp_relay_mode"] = mode
		}
		tls, err := buildTLS("tuic", p, n.Address)
		if err != nil {
			return nil, err
		}
		ob["tls"] = tls
	case "shadowsocks", "ss":
		if p["method"] == "" || p["password"] == "" {
			return nil, fmt.Errorf("shadowsocks node is missing method or password")
		}
		ob["type"] = "shadowsocks"
		ob["method"] = p["method"]
		ob["password"] = p["password"]
		if plugin := p["plugin"]; plugin != "" {
			ob["plugin"] = plugin
			if pluginOpts := p["plugin_opts"]; pluginOpts != "" {
				ob["plugin_opts"] = pluginOpts
			}
		}
	default:
		return nil, fmt.Errorf("unsupported sing-box protocol %q", n.Protocol)
	}

	return ob, nil
}

func applyTLSAndTransport(ob map[string]any, protocol string, p map[string]string) error {
	server, _ := ob["server"].(string)
	tls, err := buildTLS(protocol, p, server)
	if err != nil {
		return err
	}
	if tls != nil {
		ob["tls"] = tls
	}
	transport, err := buildTransport(p)
	if err != nil {
		return err
	}
	if transport != nil {
		ob["transport"] = transport
	}
	return nil
}

func buildTLS(protocol string, p map[string]string, server string) (map[string]any, error) {
	security := strings.ToLower(p["security"])
	if security == "none" {
		return nil, nil
	}
	enabled := security == "tls" || security == "reality" ||
		protocol == "trojan" || protocol == "hysteria2" || protocol == "tuic" ||
		truthy(p["tls"])
	if !enabled {
		return nil, nil
	}
	tls := map[string]any{"enabled": true}
	sni := firstNonEmpty(p["sni"], p["server_name"], p["host"], server)
	if sni != "" {
		tls["server_name"] = sni
	}
	if truthy(p["insecure"]) || truthy(p["allowinsecure"]) || truthy(p["skip-cert-verify"]) {
		tls["insecure"] = true
	}
	if alpn := firstNonEmpty(p["alpn"]); alpn != "" {
		tls["alpn"] = splitList(alpn)
	}
	fp := firstNonEmpty(p["fp"], p["fingerprint"])
	if fp == "" && security == "reality" {
		fp = "chrome"
	}
	if fp != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
	}
	if security == "reality" {
		if p["pbk"] == "" {
			return nil, fmt.Errorf("reality node is missing public key")
		}
		reality := map[string]any{"enabled": true, "public_key": p["pbk"]}
		if sid := p["sid"]; sid != "" {
			reality["short_id"] = sid
		}
		tls["reality"] = reality
	}
	return tls, nil
}

func buildTransport(p map[string]string) (map[string]any, error) {
	network := strings.ToLower(firstNonEmpty(p["type"], p["net"]))
	switch network {
	case "", "tcp":
		if strings.EqualFold(p["headertype"], "http") {
			return buildHTTPTransport(p, false), nil
		}
		return nil, nil
	case "ws", "websocket":
		tr := map[string]any{"type": "ws", "path": defaultString(p["path"], "/")}
		if host := p["host"]; host != "" {
			tr["headers"] = map[string]any{"Host": host}
		}
		return tr, nil
	case "grpc":
		service := firstNonEmpty(p["servicename"], strings.TrimPrefix(p["path"], "/"))
		tr := map[string]any{"type": "grpc"}
		if service != "" {
			tr["service_name"] = service
		}
		return tr, nil
	case "httpupgrade":
		tr := map[string]any{"type": "httpupgrade", "path": defaultString(p["path"], "/")}
		if host := p["host"]; host != "" {
			tr["host"] = host
		}
		return tr, nil
	case "http", "h2", "h3":
		return buildHTTPTransport(p, true), nil
	case "quic":
		return map[string]any{"type": "quic"}, nil
	case "xhttp", "splithttp":
		return nil, fmt.Errorf("xhttp transport requires Xray-core")
	default:
		return nil, fmt.Errorf("unsupported sing-box transport %q", network)
	}
}

func buildHTTPTransport(p map[string]string, withHost bool) map[string]any {
	tr := map[string]any{"type": "http", "path": defaultString(p["path"], "/")}
	if withHost && p["host"] != "" {
		tr["host"] = splitList(p["host"])
	}
	return tr
}

func localRuleSet(tag, dir, file string) map[string]any {
	return map[string]any{
		"tag":    tag,
		"type":   "local",
		"format": "binary",
		"path":   filepath.ToSlash(filepath.Join(dir, file)),
	}
}

func availableRuleSets(dir string) map[string]bool {
	available := map[string]bool{}
	if dir == "" {
		return available
	}
	for _, name := range []string{"geosite-ir", "geoip-ir"} {
		if info, err := os.Stat(filepath.Join(dir, name+".srs")); err == nil && !info.IsDir() {
			available[name] = true
		}
	}
	return available
}

func filterDNSRules(rules []routing.DNSRule, available map[string]bool) []routing.DNSRule {
	filtered := make([]routing.DNSRule, 0, len(rules))
	for _, rule := range rules {
		if len(rule.RuleSet) > 0 {
			rule.RuleSet = filterNames(rule.RuleSet, available)
			if len(rule.RuleSet) == 0 {
				continue
			}
		}
		filtered = append(filtered, rule)
	}
	return filtered
}

func filterRouteRules(rules []routing.RouteRule, available map[string]bool) []routing.RouteRule {
	filtered := make([]routing.RouteRule, 0, len(rules))
	for _, rule := range rules {
		if len(rule.RuleSet) > 0 {
			rule.RuleSet = filterNames(rule.RuleSet, available)
			if len(rule.RuleSet) == 0 {
				continue
			}
		}
		filtered = append(filtered, rule)
	}
	return filtered
}

func filterNames(names []string, available map[string]bool) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if available[name] {
			out = append(out, name)
		}
	}
	return out
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func defaultString(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}
