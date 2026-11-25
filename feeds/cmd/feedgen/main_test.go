package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/parser"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/subscription"
)

func TestBuildAndSignFeedRoundtrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	nodes := parser.ParseSubscriptionText("hysteria2://pw@nl.example.com:443#NL-1\nvless://11111111-1111-1111-1111-111111111111@de.example.com:443?type=xhttp#DE-1")
	payload := BuildAndSignFeed(nodes, priv, time.Now())

	if len(payload.Nodes) != 2 {
		t.Fatalf("expected 2 feed nodes, got %d", len(payload.Nodes))
	}
	if err := subscription.VerifyFeedSignature(base64.StdEncoding.EncodeToString(pub), &payload); err != nil {
		t.Fatalf("VerifyFeedSignature failed on generated feed: %v", err)
	}
}
