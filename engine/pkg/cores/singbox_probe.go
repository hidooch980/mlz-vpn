package cores

import (
	"encoding/json"
	"fmt"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

// BuildSingBoxProbeConfig builds a temporary sing-box configuration that exposes
// every protocol-compatible candidate as a separate outbound plus a local Clash
// API. The caller runs it briefly and queries /proxies/{tag}/delay to obtain
// true end-to-end Real Delay values before committing to a node.
//
// Returned tags align with nodes; an empty tag means the node cannot be probed
// by sing-box (for example an XHTTP-only node that requires Xray-core).
func BuildSingBoxProbeConfig(nodes []model.Node, mixedPort, clashPort int, secret string) ([]byte, []string, error) {
	tags := make([]string, len(nodes))
	outbounds := make([]map[string]any, 0, len(nodes)+1)
	for i, node := range nodes {
		tag := fmt.Sprintf("probe-%d", i)
		ob, err := buildSingBoxNodeOutbound(tag, node)
		if err != nil {
			continue
		}
		delete(ob, "domain_resolver")
		outbounds = append(outbounds, ob)
		tags[i] = tag
	}
	if len(outbounds) == 0 {
		return nil, tags, fmt.Errorf("no probe-compatible nodes")
	}
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": "direct"})

	cfg := map[string]any{
		"log": map[string]any{"level": "error", "timestamp": true},
		"dns": map[string]any{
			"servers": []map[string]any{{"type": "local", "tag": "dns-local"}},
			"final":   "dns-local",
		},
		"inbounds": []map[string]any{
			{
				"type":        "mixed",
				"tag":         "probe-in",
				"listen":      "127.0.0.1",
				"listen_port": mixedPort,
			},
		},
		"outbounds": outbounds,
		"route": map[string]any{
			"final":                   "direct",
			"default_domain_resolver": "dns-local",
		},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": fmt.Sprintf("127.0.0.1:%d", clashPort),
				"secret":              secret,
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, tags, err
	}
	return data, tags, nil
}
