package cores

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

func TestBuildSingBoxConfigProtocols(t *testing.T) {
	tests := []struct {
		name string
		node model.Node
		tun  bool
	}{
		{
			name: "vless-reality-grpc",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "vless", Params: map[string]string{
				"uuid": "11111111-1111-1111-1111-111111111111", "security": "reality", "sni": "www.microsoft.com",
				"pbk": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0", "sid": "0123", "fp": "chrome",
				"type": "grpc", "servicename": "grpcsvc", "flow": "xtls-rprx-vision",
			}},
		},
		{
			name: "vless-ws-tls",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "vless", Params: map[string]string{
				"uuid": "11111111-1111-1111-1111-111111111111", "security": "tls", "sni": "cdn.example.com",
				"type": "ws", "path": "/ws", "host": "cdn.example.com", "alpn": "h2,http/1.1",
			}},
		},
		{
			name: "vmess-ws",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "vmess", Params: map[string]string{
				"uuid": "11111111-1111-1111-1111-111111111111", "security": "auto", "alterid": "0",
				"tls": "tls", "sni": "cdn.example.com", "type": "ws", "path": "/vm", "host": "cdn.example.com",
			}},
		},
		{
			name: "trojan-ws",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "trojan", Params: map[string]string{
				"password": "p@ss:word", "sni": "de.example.com", "type": "ws", "path": "/tr",
			}},
		},
		{
			name: "shadowsocks",
			node: model.Node{Address: "de.example.com", Port: 8388, Protocol: "shadowsocks", Params: map[string]string{
				"method": "aes-256-gcm", "password": "secret",
			}},
		},
		{
			name: "hysteria2",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "hysteria2", Params: map[string]string{
				"password": "secret", "sni": "de.example.com", "insecure": "1", "obfs": "salamander", "obfs-password": "obfs",
			}},
		},
		{
			name: "tuic",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "tuic", Params: map[string]string{
				"uuid": "11111111-1111-1111-1111-111111111111", "password": "secret", "sni": "de.example.com",
				"congestion_control": "bbr", "udp_relay_mode": "native",
			}},
		},
		{
			name: "tun",
			tun:  true,
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "vless", Params: map[string]string{
				"uuid": "11111111-1111-1111-1111-111111111111", "security": "tls", "sni": "de.example.com",
			}},
		},
	}

	bin := os.Getenv("SINGBOX_BIN")
	dir := t.TempDir()
	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			opts := SingBoxBuildOptions{
				TunnelMode:     model.TunnelModeProxy,
				LocalMixedPort: 10809,
				ClashAPIPort:   19090,
				ClashSecret:    "test",
				PrimaryNode:    tc.node,
				AutoInterface:  true,
				EnableIPv6:     true,
			}
			if tc.tun {
				opts.TunnelMode = model.TunnelModeTUN
			}
			data, err := BuildSingBoxConfig(opts)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("json: %v", err)
			}
			if bin == "" {
				return
			}
			path := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, "check", "-c", path)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sing-box check failed: %v\n%s\n%s", err, out, data)
			}
		})
	}
}

func TestBuildSingBoxProbeConfig(t *testing.T) {
	nodes := []model.Node{
		{ID: "a", Address: "de.example.com", Port: 443, Protocol: "vless", Params: map[string]string{
			"uuid": "11111111-1111-1111-1111-111111111111", "security": "tls", "sni": "de.example.com",
		}},
		{ID: "b", Address: "de.example.com", Port: 8388, Protocol: "shadowsocks", Params: map[string]string{
			"method": "aes-256-gcm", "password": "secret",
		}},
		{ID: "x", Address: "de.example.com", Port: 443, Protocol: "vless", Params: map[string]string{
			"uuid": "11111111-1111-1111-1111-111111111111", "type": "xhttp",
		}},
	}
	data, tags, err := BuildSingBoxProbeConfig(nodes, 10809, 19090, "secret")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if tags[0] == "" || tags[1] == "" || tags[2] != "" {
		t.Fatalf("unexpected tags: %v", tags)
	}
	bin := os.Getenv("SINGBOX_BIN")
	if bin == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "probe.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "check", "-c", path).CombinedOutput()
	if err != nil {
		t.Fatalf("probe check failed: %v\n%s\n%s", err, out, data)
	}
}

func TestBuildSingBoxRejectsXHTTP(t *testing.T) {
	_, err := BuildSingBoxConfig(SingBoxBuildOptions{
		TunnelMode: model.TunnelModeProxy,
		PrimaryNode: model.Node{Address: "de.example.com", Port: 443, Protocol: "vless", Params: map[string]string{
			"uuid": "11111111-1111-1111-1111-111111111111", "type": "xhttp", "security": "tls", "sni": "de.example.com",
		}},
	})
	if err == nil {
		t.Fatal("expected xhttp to be rejected by sing-box builder")
	}
}
