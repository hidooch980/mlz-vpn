// Package parser decodes multi-protocol proxy URIs and subscription payloads
// into normalized model.Node structures without user intervention.
package parser

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

var ErrUnsupportedScheme = errors.New("unsupported proxy URI scheme")

// ParseSubscriptionText decodes plain or base64-encoded newline-separated proxy URIs.
func ParseSubscriptionText(raw string) []model.Node {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	content := trimmed
	if !strings.Contains(trimmed, "://") {
		if decoded, err := decodeBase64Flexible(trimmed); err == nil && strings.Contains(string(decoded), "://") {
			content = string(decoded)
		}
	}

	lines := strings.Split(content, "\n")
	nodes := make([]model.Node, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := ParseURI(line)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// ParseURI normalizes a single proxy URI (vless, vmess, trojan, ss, hysteria2, tuic, ssconf).
func ParseURI(rawURI string) (model.Node, error) {
	rawURI = strings.TrimSpace(rawURI)
	if strings.HasPrefix(strings.ToLower(rawURI), "vmess://") {
		return parseVMess(rawURI)
	}
	if strings.HasPrefix(strings.ToLower(rawURI), "ss://") {
		return parseShadowsocks(rawURI)
	}

	u, err := url.Parse(rawURI)
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid URI: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "vless", "trojan", "hysteria2", "hy2", "tuic":
		return parseStandardURL(u, scheme, rawURI)
	case "ssconf":
		return model.Node{
			Name:     fallbackName(u.Fragment, u.Host),
			Engine:   model.CoreSingBox,
			Protocol: "outline-ssconf",
			Address:  u.Hostname(),
			Port:     parsePortOrDefault(u.Port(), 443),
			RawURI:   rawURI,
		}, nil
	default:
		return model.Node{}, fmt.Errorf("%w: %s", ErrUnsupportedScheme, scheme)
	}
}

func parseStandardURL(u *url.URL, scheme, rawURI string) (model.Node, error) {
	host := u.Hostname()
	if host == "" {
		return model.Node{}, errors.New("missing server host")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port <= 0 || port > 65535 {
		return model.Node{}, errors.New("invalid or missing port")
	}

	params := make(map[string]string)
	for k, v := range u.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}

	protocol := scheme
	if scheme == "hy2" {
		protocol = "hysteria2"
	}

	engine := model.CoreSingBox
	if strings.EqualFold(params["type"], "xhttp") || strings.EqualFold(params["type"], "splithttp") || params["fragment"] != "" {
		engine = model.CoreXray
	}

	return model.Node{
		Name:     fallbackName(u.Fragment, net.JoinHostPort(host, strconv.Itoa(port))),
		Engine:   engine,
		Protocol: protocol,
		Address:  host,
		Port:     port,
		RawURI:   rawURI,
		Params:   params,
	}, nil
}

type vmessJSON struct {
	V    any    `json:"v"`
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port any    `json:"port"`
	ID   string `json:"id"`
	Net  string `json:"net"`
	Type string `json:"type"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
}

func parseVMess(rawURI string) (model.Node, error) {
	encoded := strings.TrimPrefix(rawURI, "vmess://")
	encoded = strings.TrimPrefix(encoded, "VMESS://")
	rawJSON, err := decodeBase64Flexible(encoded)
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid vmess base64: %w", err)
	}

	var v vmessJSON
	if err := json.Unmarshal(rawJSON, &v); err != nil {
		return model.Node{}, fmt.Errorf("invalid vmess JSON: %w", err)
	}
	if v.Add == "" || v.ID == "" {
		return model.Node{}, errors.New("incomplete vmess node")
	}

	port := parseAnyPort(v.Port)
	if port <= 0 || port > 65535 {
		return model.Node{}, errors.New("invalid vmess port")
	}

	return model.Node{
		Name:     fallbackName(v.PS, net.JoinHostPort(v.Add, strconv.Itoa(port))),
		Engine:   model.CoreSingBox,
		Protocol: "vmess",
		Address:  v.Add,
		Port:     port,
		RawURI:   rawURI,
		Params: map[string]string{
			"uuid": v.ID,
			"net":  v.Net,
			"tls":  v.TLS,
			"sni":  v.SNI,
			"host": v.Host,
			"path": v.Path,
		},
	}, nil
}

func parseShadowsocks(rawURI string) (model.Node, error) {
	u, err := url.Parse(rawURI)
	if err == nil && u.Hostname() != "" && u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port <= 0 || port > 65535 {
			return model.Node{}, errors.New("invalid ss port")
		}
		return model.Node{
			Name:     fallbackName(u.Fragment, u.Host),
			Engine:   model.CoreSingBox,
			Protocol: "shadowsocks",
			Address:  u.Hostname(),
			Port:     port,
			RawURI:   rawURI,
		}, nil
	}

	// Fallback for legacy ss://<base64(method:password@host:port)>#name
	body := strings.TrimPrefix(rawURI, "ss://")
	fragment := ""
	if idx := strings.IndexByte(body, '#'); idx >= 0 {
		fragment, _ = url.QueryUnescape(body[idx+1:])
		body = body[:idx]
	}
	decoded, err := decodeBase64Flexible(body)
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid legacy ss URI: %w", err)
	}
	atIdx := strings.LastIndexByte(string(decoded), '@')
	if atIdx < 0 {
		return model.Node{}, errors.New("missing @ in ss payload")
	}
	hostPort := string(decoded)[atIdx+1:]
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid ss host:port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return model.Node{}, errors.New("invalid ss port")
	}
	return model.Node{
		Name:     fallbackName(fragment, hostPort),
		Engine:   model.CoreSingBox,
		Protocol: "shadowsocks",
		Address:  host,
		Port:     port,
		RawURI:   rawURI,
	}, nil
}

func decodeBase64Flexible(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	if rem := len(s) % 4; rem != 0 {
		s += strings.Repeat("=", 4-rem)
	}
	return base64.StdEncoding.DecodeString(s)
}

func parseAnyPort(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(val))
		return n
	default:
		return 0
	}
}

func parsePortOrDefault(portStr string, fallback int) int {
	n, err := strconv.Atoi(portStr)
	if err != nil || n <= 0 || n > 65535 {
		return fallback
	}
	return n
}

func fallbackName(primary, alt string) string {
	if s := strings.TrimSpace(primary); s != "" {
		return s
	}
	return alt
}
