// Command feedgen aggregates public proxy sources, deduplicates nodes,
// signs feed.v1.json with Ed25519, and outputs the 5-minute feed bundle.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/parser"
)

type canonicalFeed struct {
	Schema      string           `json:"schema"`
	Sequence    int64            `json:"sequence"`
	GeneratedAt time.Time        `json:"generated_at"`
	TTLSeconds  int              `json:"ttl_seconds"`
	Mirrors     []string         `json:"mirrors,omitempty"`
	Nodes       []model.FeedNode `json:"nodes"`
}

// BuildAndSignFeed normalizes parsed nodes and optionally signs the canonical JSON with Ed25519.
func BuildAndSignFeed(nodes []model.Node, privKey ed25519.PrivateKey, now time.Time) model.FeedPayload {
	feedNodes := make([]model.FeedNode, 0, len(nodes))
	for i, n := range nodes {
		feedNodes = append(feedNodes, model.FeedNode{
			ID:        fmt.Sprintf("node-%03d", i+1),
			Engine:    n.Engine,
			Protocol:  n.Protocol,
			Region:    n.Region,
			Weight:    0.85,
			ConfigURI: n.RawURI,
		})
	}

	canon := canonicalFeed{
		Schema:      "mlz.feed.v1",
		Sequence:    now.UTC().Unix(),
		GeneratedAt: now.UTC(),
		TTLSeconds:  300,
		Mirrors: []string{
			"https://cdn.jsdelivr.net/gh/hidooch980/mlz-vpn@main/feeds/feed.v1.json",
			"https://raw.githubusercontent.com/hidooch980/mlz-vpn/main/feeds/feed.v1.json",
		},
		Nodes: feedNodes,
	}

	var sigB64 string
	if len(privKey) == ed25519.PrivateKeySize {
		rawCanon, _ := json.Marshal(canon)
		sigB64 = base64.StdEncoding.EncodeToString(ed25519.Sign(privKey, rawCanon))
	}

	return model.FeedPayload{
		Schema:      canon.Schema,
		Sequence:    canon.Sequence,
		GeneratedAt: canon.GeneratedAt,
		TTLSeconds:  canon.TTLSeconds,
		Mirrors:     canon.Mirrors,
		Nodes:       canon.Nodes,
		Signature:   sigB64,
	}
}

func main() {
	sourcesFlag := flag.String("sources", "", "comma-separated URLs of upstream open-source proxy lists")
	outPath := flag.String("out", "feed.v1.json", "output signed feed path")
	flag.Parse()

	privB64 := strings.TrimSpace(os.Getenv("MLZ_FEED_PRIVKEY"))
	var privKey ed25519.PrivateKey
	if privB64 != "" {
		raw, err := base64.StdEncoding.DecodeString(privB64)
		if err != nil || len(raw) != ed25519.PrivateKeySize {
			fmt.Fprintf(os.Stderr, "invalid MLZ_FEED_PRIVKEY\n")
			os.Exit(1)
		}
		privKey = ed25519.PrivateKey(raw)
	}

	var allNodes []model.Node
	seen := make(map[string]struct{})
	client := &http.Client{Timeout: 10 * time.Second}

	for _, src := range strings.Split(*sourcesFlag, ",") {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		resp, err := client.Get(src)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()

		for _, n := range parser.ParseSubscriptionText(string(body)) {
			key := fmt.Sprintf("%s:%d:%s", n.Address, n.Port, n.Protocol)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			allNodes = append(allNodes, n)
			if len(allNodes) >= 120 {
				break
			}
		}
	}

	final := BuildAndSignFeed(allNodes, privKey, time.Now().UTC())
	encoded, err := json.MarshalIndent(final, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal failed: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outPath, encoded, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
		os.Exit(1)
	}
}
