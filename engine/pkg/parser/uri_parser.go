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
			params[k] = strings.ToLower(strings.TrimSpace(v[0]))
		}
	}

	protocol := scheme
	if scheme == "hy2" {
		protocol = "hysteria2"
	}

	// Credentials live in the URI userinfo and were previously dropped entirely,
	// producing outbounds without UUID/password. Keep them in Params so both core
	// builders and tests can consume the same normalized representation.
	if u.User != nil {
		username := u.User.Username()
		password, hasPassword := u.User.Password()
		switch protocol {
		case "vless":
			if username == "" {
				return model.Node{}, errors.New("vless requires a UUID")
			}
			params["uuid"] = username
		case "trojan", "hysteria2":
			if !hasPassword && username == "" {
				return model.Node{}, errors.New(protocol + " requires a password")
			}
			if hasPassword {
				// Standard form is scheme://password@host. A colon is legal inside
				// the password, so preserve the complete userinfo.
				params["password"] = username + ":" + password
			} else {
				params["password"] = username
			}
		case "tuic":
			if username == "" || !hasPassword {
				return model.Node{}, errors.New("tuic requires uuid:password userinfo")
			}
			params["uuid"] = username
			params["password"] = password
		}
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
	V        any    `json:"v"`
	PS       string `json:"ps"`
	Add      string `json:"add"`
	Port     any    `json:"port"`
	ID       string `json:"id"`
	Net      string `json:"net"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Path     string `json:"path"`
	TLS      string `json:"tls"`
	SNI      string `json:"sni"`
	Security string `json:"scy"`
}

func parseVMess(rawURI string) (model.Node, error) {
	encoded := rawURI[len("vmess://"):]
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

	engine := model.CoreSingBox
	if strings.EqualFold(v.Net, "xhttp") || strings.EqualFold(v.Net, "splithttp") {
		engine = model.CoreXray
	}
	security := v.Security
	if security == "" {
		security = "auto"
	}
	return model.Node{
		Name:     fallbackName(v.PS, net.JoinHostPort(v.Add, strconv.Itoa(port))),
		Engine:   engine,
		Protocol: "vmess",
		Address:  v.Add,
		Port:     port,
		RawURI:   rawURI,
		Params: map[string]string{
			"uuid":     v.ID,
			"net":      v.Net,
			"type":     v.Net,
			"tls":      v.TLS,
			"sni":      v.SNI,
			"host":     v.Host,
			"path":     v.Path,
			"security": security,
		},
	}, nil
}

func parseShadowsocks(rawURI string) (model.Node, error) {
	// SIP002: ss://BASE64(method:password)@host:port?plugin=...#name
	// Also accept the unencoded method:password@host:port form.
	if u, err := url.Parse(rawURI); err == nil && u.Hostname() != "" && u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port <= 0 || port > 65535 {
			return model.Node{}, errors.New("invalid ss port")
		}
		method, password, err := decodeSSUserInfo(u)
		if err != nil {
			return model.Node{}, err
		}
		params := map[string]string{"method": method, "password": password}
		for k, v := range u.Query() {
			if len(v) > 0 {
				params[strings.ToLower(k)] = v[0]
			}
		}
		return model.Node{
			Name:     fallbackName(u.Fragment, u.Host),
			Engine:   model.CoreSingBox,
			Protocol: "shadowsocks",
			Address:  u.Hostname(),
			Port:     port,
			RawURI:   rawURI,
			Params:   params,
		}, nil
	}

	// Fallback for legacy ss://<base64(method:password@host:port)>#name
	body := rawURI[len("ss://"):]
	fragment := ""
	if idx := strings.IndexByte(body, '#'); idx >= 0 {
		fragment, _ = url.QueryUnescape(body[idx+1:])
		body = body[:idx]
	}
	query := url.Values{}
	if idx := strings.IndexByte(body, '?'); idx >= 0 {
		query, _ = url.ParseQuery(body[idx+1:])
		body = body[:idx]
	}
	decoded, err := decodeBase64Flexible(body)
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid legacy ss URI: %w", err)
	}
	payload := string(decoded)
	atIdx := strings.LastIndexByte(payload, '@')
	if atIdx < 0 {
		return model.Node{}, errors.New("missing @ in ss payload")
	}
	creds := payload[:atIdx]
	hostPort := payload[atIdx+1:]
	method, password, ok := strings.Cut(creds, ":")
	if !ok || method == "" || password == "" {
		return model.Node{}, errors.New("invalid ss credentials")
	}
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid ss host:port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return model.Node{}, errors.New("invalid ss port")
	}
	params := map[string]string{
		"method":   strings.ToLower(strings.TrimSpace(method)),
		"password": password,
	}
	for k, v := range query {
		if len(v) > 0 {
			params[strings.ToLower(k)] = v[0]
		}
	}
	return model.Node{
		Name:     fallbackName(fragment, hostPort),
		Engine:   model.CoreSingBox,
		Protocol: "shadowsocks",
		Address:  host,
		Port:     port,
		RawURI:   rawURI,
		Params:   params,
	}, nil
}

func decodeSSUserInfo(u *url.URL) (string, string, error) {
	if u.User == nil {
		return "", "", errors.New("ss URI is missing credentials")
	}
	username := u.User.Username()
	if password, ok := u.User.Password(); ok && username != "" {
		return strings.ToLower(username), password, nil
	}
	decoded, err := decodeBase64Flexible(username)
	if err != nil {
		return "", "", fmt.Errorf("invalid base64 ss credentials: %w", err)
	}
	method, password, ok := strings.Cut(string(decoded), ":")
	if !ok || method == "" || password == "" {
		return "", "", errors.New("invalid ss credentials")
	}
	return strings.ToLower(strings.TrimSpace(method)), password, nil
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
