package cores

import (
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

func TestBuildSidecarCommandAllCatalog(t *testing.T) {
	target := model.Node{
		Address: "example.com",
		Port:    443,
		Params: map[string]string{
			"password": "secret",
			"domain":   "t.example.com",
			"pubkey":   "abc123",
		},
	}

	for kind := range AllInCatalog() {
		args, err := BuildSidecarCommand(kind, "127.0.0.1:20801", "u", "p", target)
		if err != nil {
			t.Fatalf("BuildSidecarCommand failed for %s: %v", kind, err)
		}
		if len(args) == 0 {
			t.Fatalf("expected non-empty args for %s", kind)
		}
	}
}
