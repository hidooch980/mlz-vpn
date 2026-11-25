package cores

import (
	"encoding/json"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

func TestAllInCatalogAndConfigBuilders(t *testing.T) {
	cat := AllInCatalog()
	expected := []model.CoreKind{
		model.CoreAmneziaWG, model.CoreSpoofDPI, model.CoreUsque,
		model.CoreLyrebird, model.CoreDNSTT, model.CoreByeDPI,
		model.CoreArti, model.CoreNaiveProxy, model.CorePsiphon,
		model.CoreBrook, model.CoreLantern, model.CoreTrojanGo,
	}
	for _, k := range expected {
		if _, ok := cat[k]; !ok {
			t.Fatalf("expected sidecar %s in All-In catalog", k)
		}
	}

	sbJSON, err := BuildSingBoxConfig(SingBoxBuildOptions{
		TunnelMode:       model.TunnelModeTUN,
		EnableLANShare:   true,
		LANSharePort:     10808,
		LANUsername:      "mlz",
		LANPassword:      "4829",
		ClashAPIPort:     9090,
		RuleSetDir:       "./rules",
		UpstreamSocks:    "127.0.0.1:20808",
		UpstreamUser:     "u",
		UpstreamPassword: "p",
		AutoInterface:    true,
	})
	if err != nil {
		t.Fatalf("BuildSingBoxConfig failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(sbJSON, &parsed); err != nil {
		t.Fatalf("invalid sing-box JSON: %v", err)
	}

	xrayJSON, err := BuildXrayChainConfig(XrayChainOptions{
		ListenAddr:     "127.0.0.1:20808",
		SocksUser:      "u",
		SocksPass:      "p",
		EnableFragment: true,
		Node: model.Node{
			Address:  "de.example.com",
			Port:     443,
			Protocol: "vless",
			Params: map[string]string{
				"uuid":     "11111111-1111-1111-1111-111111111111",
				"type":     "xhttp",
				"security": "reality",
				"sni":      "example.com",
				"pbk":      "pubkey",
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildXrayChainConfig failed: %v", err)
	}
	if err := json.Unmarshal(xrayJSON, &parsed); err != nil {
		t.Fatalf("invalid xray JSON: %v", err)
	}
}
