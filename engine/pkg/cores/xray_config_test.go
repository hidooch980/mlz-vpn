package cores

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

func TestBuildXrayChainConfigProtocols(t *testing.T) {
	tests := []struct {
		name string
		node model.Node
	}{
		{
			name: "vless-xhttp-reality",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "vless", Params: map[string]string{
				"uuid": "11111111-1111-1111-1111-111111111111", "security": "reality", "sni": "www.microsoft.com",
				"pbk": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0", "sid": "0123", "fp": "chrome",
				"type": "xhttp", "path": "/x", "host": "cdn.example.com", "mode": "auto",
			}},
		},
		{
			name: "vless-ws-tls-fragment",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "vless", Params: map[string]string{
				"uuid": "11111111-1111-1111-1111-111111111111", "security": "tls", "sni": "cdn.example.com",
				"type": "ws", "path": "/ws", "host": "cdn.example.com", "alpn": "h2,http/1.1",
			}},
		},
		{
			name: "vmess-grpc",
			node: model.Node{Address: "de.example.com", Port: 443, Protocol: "vmess", Params: map[string]string{
				"uuid": "11111111-1111-1111-1111-111111111111", "security": "auto", "alterid": "0",
				"tls": "tls", "sni": "cdn.example.com", "type": "grpc", "servicename": "svc",
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
	}

	bin := os.Getenv("XRAY_BIN")
	dir := t.TempDir()
	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			data, err := BuildXrayChainConfig(XrayChainOptions{
				ListenAddr:     "127.0.0.1:20808",
				SocksUser:      "u",
				SocksPass:      "p",
				Node:           tc.node,
				EnableFragment: true,
			})
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
			cmd := exec.Command(bin, "run", "-test", "-c", path)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("xray test failed: %v\n%s\n%s", err, out, data)
			}
		})
	}
}
