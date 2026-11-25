package subscription

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

func TestVerifyFeedSignatureAndRefresh(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	now := time.Date(2026, 11, 25, 18, 30, 0, 0, time.UTC)
	canon := canonicalFeed{
		Schema:      ExpectedFeedSchema,
		Sequence:    1,
		GeneratedAt: now,
		TTLSeconds:  300,
		Nodes: []model.FeedNode{
			{ID: "n1", Engine: model.CoreSingBox, Protocol: "hysteria2", Region: "NL", Weight: 0.9, ConfigURI: "hysteria2://a@b:443"},
		},
	}
	msg, _ := json.Marshal(canon)
	sig := ed25519.Sign(priv, msg)

	payload := &model.FeedPayload{
		Schema:      canon.Schema,
		Sequence:    canon.Sequence,
		GeneratedAt: canon.GeneratedAt,
		TTLSeconds:  canon.TTLSeconds,
		Nodes:       canon.Nodes,
		Signature:   base64.StdEncoding.EncodeToString(sig),
	}

	if err := VerifyFeedSignature(pubB64, payload); err != nil {
		t.Fatalf("expected valid signature, got: %v", err)
	}

	payload.Nodes[0].Region = "US"
	if err := VerifyFeedSignature(pubB64, payload); err == nil {
		t.Fatal("expected tampered payload to fail signature verification")
	}

	if ShouldRefresh(now, now.Add(4*time.Minute), false) {
		t.Fatal("should not refresh before 5 minutes")
	}
	if !ShouldRefresh(now, now.Add(5*time.Minute), false) {
		t.Fatal("should refresh at 5 minutes")
	}
	if ShouldRefresh(now, now.Add(10*time.Minute), true) {
		t.Fatal("battery saver should wait 15 minutes")
	}
}
