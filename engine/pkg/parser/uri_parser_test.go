package parser

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

func TestParseURIs(t *testing.T) {
	vless := "vless://11111111-1111-1111-1111-111111111111@de.example.com:443?type=xhttp&security=reality&sni=cdn.example.com&pbk=abc&sid=12#DE-Xray"
	node, err := ParseURI(vless)
	if err != nil {
		t.Fatalf("unexpected error parsing vless xhttp: %v", err)
	}
	if node.Engine != model.CoreXray || node.Port != 443 || node.Name != "DE-Xray" {
		t.Fatalf("unexpected parsed vless node: %+v", node)
	}
	if node.Params["uuid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("vless uuid lost: %+v", node.Params)
	}

	hy2 := "hysteria2://secret@nl.example.com:8443?sni=example.com#NL-HY2"
	hyNode, err := ParseURI(hy2)
	if err != nil {
		t.Fatalf("unexpected error parsing hy2: %v", err)
	}
	if hyNode.Engine != model.CoreSingBox || hyNode.Protocol != "hysteria2" {
		t.Fatalf("unexpected parsed hy2 node: %+v", hyNode)
	}
	if hyNode.Params["password"] != "secret" {
		t.Fatalf("hy2 password lost: %+v", hyNode.Params)
	}

	vmessRaw := `{"v":"2","ps":"VMess-Test","add":"fi.example.com","port":"443","id":"22222222-2222-2222-2222-222222222222","net":"ws","tls":"tls","sni":"fi.example.com","path":"/ws"}`
	vmessURI := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessRaw))
	vmNode, err := ParseURI(vmessURI)
	if err != nil {
		t.Fatalf("unexpected error parsing vmess: %v", err)
	}
	if vmNode.Protocol != "vmess" || vmNode.Address != "fi.example.com" || vmNode.Port != 443 {
		t.Fatalf("unexpected parsed vmess node: %+v", vmNode)
	}
	if vmNode.Params["uuid"] != "22222222-2222-2222-2222-222222222222" || vmNode.Params["path"] != "/ws" {
		t.Fatalf("vmess params lost: %+v", vmNode.Params)
	}

	tuic := "tuic://33333333-3333-3333-3333-333333333333:pass@tu.example.com:443?congestion_control=bbr&alpn=h3#TU"
	tuicNode, err := ParseURI(tuic)
	if err != nil {
		t.Fatalf("unexpected error parsing tuic: %v", err)
	}
	if tuicNode.Params["uuid"] == "" || tuicNode.Params["password"] != "pass" {
		t.Fatalf("tuic credentials lost: %+v", tuicNode.Params)
	}

	trojan := "trojan://p%3Ass%40word@tr.example.com:443?sni=tr.example.com#TR"
	trojanNode, err := ParseURI(trojan)
	if err != nil {
		t.Fatalf("unexpected error parsing trojan: %v", err)
	}
	if trojanNode.Params["password"] != "p:ss@word" {
		t.Fatalf("trojan password not decoded: %+v", trojanNode.Params)
	}

	sub := base64.StdEncoding.EncodeToString([]byte(vless + "\n" + hy2))
	list := ParseSubscriptionText(sub)
	if len(list) != 2 {
		t.Fatalf("expected 2 nodes from base64 subscription, got %d", len(list))
	}
}

func TestParseShadowsocksCredentials(t *testing.T) {
	userInfo := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:p@ss:word"))
	sip002 := "ss://" + userInfo + "@ss.example.com:8388#SS-SIP002"
	node, err := ParseURI(sip002)
	if err != nil {
		t.Fatalf("unexpected error parsing SIP002 ss: %v", err)
	}
	if node.Params["method"] != "aes-256-gcm" || node.Params["password"] != "p@ss:word" {
		t.Fatalf("SIP002 credentials lost: %+v", node.Params)
	}

	plain := "ss://chacha20-ietf-poly1305:secret@ss2.example.com:8389#SS-Plain"
	plainNode, err := ParseURI(plain)
	if err != nil {
		t.Fatalf("unexpected error parsing plain ss: %v", err)
	}
	if plainNode.Params["method"] != "chacha20-ietf-poly1305" || plainNode.Params["password"] != "secret" {
		t.Fatalf("plain credentials lost: %+v", plainNode.Params)
	}

	legacyPayload := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:legacy@pass@ss3.example.com:8390"))
	legacy := "ss://" + legacyPayload + "#SS-Legacy"
	legacyNode, err := ParseURI(legacy)
	if err != nil {
		t.Fatalf("unexpected error parsing legacy ss: %v", err)
	}
	if legacyNode.Params["password"] != "legacy@pass" {
		t.Fatalf("legacy password lost: %+v", legacyNode.Params)
	}
}

func TestParseRejectsMissingCredentials(t *testing.T) {
	for _, uri := range []string{
		"vless://@de.example.com:443",
		"trojan://@tr.example.com:443",
		"ss://@ss.example.com:8388",
		"tuic://uuid@tu.example.com:443",
	} {
		if _, err := ParseURI(uri); err == nil {
			t.Fatalf("expected error for missing credentials: %s", uri)
		}
	}
}

func TestParseSubscriptionFiltersInvalid(t *testing.T) {
	text := strings.Join([]string{
		"vless://11111111-1111-1111-1111-111111111111@a.example.com:443#A",
		"ss://broken",
		"trojan://pass@b.example.com:443#B",
	}, "\n")
	nodes := ParseSubscriptionText(text)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 valid nodes, got %d", len(nodes))
	}
}
