package probe

import (
	"context"
	"errors"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

type fakeDialer struct {
	latencies map[string]int
}

func (f fakeDialer) ProbeGenerate204(_ context.Context, node model.Node, _ string) (int, error) {
	ms, ok := f.latencies[node.ID]
	if !ok || ms <= 0 {
		return 0, errors.New("timeout")
	}
	return ms, nil
}

func TestRankNodes(t *testing.T) {
	nodes := []model.Node{
		{ID: "de-slow", Region: "DE", Engine: model.CoreSingBox, Protocol: "hysteria2", Weight: 0.8},
		{ID: "de-fast", Region: "DE", Engine: model.CoreSingBox, Protocol: "hysteria2", Weight: 0.9},
		{ID: "nl-dead", Region: "NL", Engine: model.CoreSingBox, Protocol: "tuic", Weight: 0.9},
		{ID: "nl-xray", Region: "NL", Engine: model.CoreXray, Protocol: "vless", Weight: 0.85},
	}

	dialer := fakeDialer{
		latencies: map[string]int{
			"de-slow": 250,
			"de-fast": 40,
			"nl-xray": 75,
		},
	}

	ranked := RankNodes(context.Background(), nodes, model.EngineModeAuto, model.CountryPreference{Auto: true}, dialer)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 healthy nodes, got %d", len(ranked))
	}
	if ranked[0].Node.ID != "de-fast" {
		t.Fatalf("expected de-fast to rank first, got %s", ranked[0].Node.ID)
	}

	deOnly := RankNodes(context.Background(), nodes, model.EngineModeAuto, model.CountryPreference{Auto: false, Code: "DE"}, dialer)
	if len(deOnly) != 2 {
		t.Fatalf("expected 2 DE nodes, got %d", len(deOnly))
	}
}

func TestCountryNameFA(t *testing.T) {
	cases := map[string]string{
		"IT": "ایتالیا",
		"it": "ایتالیا",
		"DE": "آلمان",
		"IR": "ایران",
		"US": "آمریکا",
		"ZZ": "ZZ",
		"":   "نامشخص",
	}
	for code, want := range cases {
		if got := CountryNameFA(code); got != want {
			t.Fatalf("CountryNameFA(%q) = %q, want %q", code, got, want)
		}
	}
}
