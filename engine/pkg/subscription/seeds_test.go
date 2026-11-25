package subscription

import (
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/parser"
)

func TestEmbeddedSeedsParse(t *testing.T) {
	seeds := EmbeddedSeeds()
	if len(seeds) < 3 {
		t.Fatalf("expected a small usable fallback pool, got %d", len(seeds))
	}
	for _, seed := range seeds {
		if seed.Engine != "" && seed.Engine != model.CoreSingBox {
			t.Fatalf("seed %s must be sing-box compatible on Android, got %s", seed.ID, seed.Engine)
		}
		node, err := parser.ParseURI(seed.ConfigURI)
		if err != nil {
			t.Fatalf("seed %s does not parse: %v", seed.ID, err)
		}
		if node.Address == "" || node.Port <= 0 {
			t.Fatalf("seed %s parsed without a usable endpoint", seed.ID)
		}
	}
}
