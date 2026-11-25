package parser

import (
	"encoding/base64"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

func TestParseURIs(t *testing.T) {
	vless := "vless://11111111-1111-1111-1111-111111111111@de.example.com:443?type=xhttp&security=reality#DE-Xray"
	node, err := ParseURI(vless)
	if err != nil {
		t.Fatalf("unexpected error parsing vless xhttp: %v", err)
	}
	if node.Engine != model.CoreXray || node.Port != 443 || node.Name != "DE-Xray" {
		t.Fatalf("unexpected parsed vless node: %+v", node)
	}

	hy2 := "hysteria2://secret@nl.example.com:8443?sni=example.com#NL-HY2"
	hyNode, err := ParseURI(hy2)
	if err != nil {
		t.Fatalf("unexpected error parsing hy2: %v", err)
	}
	if hyNode.Engine != model.CoreSingBox || hyNode.Protocol != "hysteria2" {
		t.Fatalf("unexpected parsed hy2 node: %+v", hyNode)
	}

	vmessRaw := `{"v":"2","ps":"VMess-Test","add":"fi.example.com","port":"443","id":"22222222-2222-2222-2222-222222222222","net":"ws","tls":"tls"}`
	vmessURI := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessRaw))
	vmNode, err := ParseURI(vmessURI)
	if err != nil {
		t.Fatalf("unexpected error parsing vmess: %v", err)
	}
	if vmNode.Protocol != "vmess" || vmNode.Address != "fi.example.com" || vmNode.Port != 443 {
		t.Fatalf("unexpected parsed vmess node: %+v", vmNode)
	}

	sub := base64.StdEncoding.EncodeToString([]byte(vless + "\n" + hy2))
	list := ParseSubscriptionText(sub)
	if len(list) != 2 {
		t.Fatalf("expected 2 nodes from base64 subscription, got %d", len(list))
	}
}
