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

func TestValidateFeedFreshnessGrace(t *testing.T) {
	now := time.Date(2026, 11, 25, 21, 0, 0, 0, time.UTC)
	payload := &model.FeedPayload{GeneratedAt: now.Add(-2 * time.Hour), TTLSeconds: 300}
	if err := ValidateFeedFreshness(payload, now); err != nil {
		t.Fatalf("a two-hour scheduler delay must remain usable: %v", err)
	}

	payload.GeneratedAt = now.Add(-6 * time.Hour)
	if err := ValidateFeedFreshness(payload, now); err == nil {
		t.Fatal("expected a six-hour-old feed to be rejected")
	}

	payload.GeneratedAt = now.Add(20 * time.Minute)
	if err := ValidateFeedFreshness(payload, now); err == nil {
		t.Fatal("expected a feed from the future to be rejected")
	}

	payload.GeneratedAt = now
	payload.TTLSeconds = int((25 * time.Hour).Seconds())
	if err := ValidateFeedFreshness(payload, now); err == nil {
		t.Fatal("expected an excessive TTL to be rejected")
	}
}
