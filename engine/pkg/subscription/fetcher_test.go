package subscription

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

func signedFeed(t *testing.T, priv ed25519.PrivateKey, seq int64, generated time.Time, ttl int) []byte {
	t.Helper()
	canon := canonicalFeed{
		Schema:      ExpectedFeedSchema,
		Sequence:    seq,
		GeneratedAt: generated.UTC(),
		TTLSeconds:  ttl,
		Nodes: []model.FeedNode{
			{ID: "n1", Engine: model.CoreSingBox, Protocol: "hysteria2", Region: "NL", ConfigURI: "hysteria2://a@b:443"},
		},
	}
	msg, err := json.Marshal(canon)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, msg)
	payload := model.FeedPayload{
		Schema:      canon.Schema,
		Sequence:    canon.Sequence,
		GeneratedAt: canon.GeneratedAt,
		TTLSeconds:  canon.TTLSeconds,
		Nodes:       canon.Nodes,
		Signature:   base64.StdEncoding.EncodeToString(sig),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestFetcherRejectsRollbackAndExpiredFeed(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	body := signedFeed(t, priv, 10, now, 300)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := FeedSourceConfig{
		PublicKeyBase64: base64.StdEncoding.EncodeToString(pub),
		FeedURLs:        []string{srv.URL},
		StatePath:       filepath.Join(t.TempDir(), "seq"),
	}
	f := NewFetcher(cfg)
	updated, err := f.SyncIfDue(context.Background(), true, false, 0)
	if err != nil || !updated {
		t.Fatalf("initial sync: updated=%v err=%v", updated, err)
	}
	nodes, _ := f.CachedNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected accepted node, got %d", len(nodes))
	}

	body = signedFeed(t, priv, 9, now, 300)
	if _, err := f.SyncIfDue(context.Background(), true, false, 0); err == nil {
		t.Fatal("expected rollback rejection")
	}
	nodes, _ = f.CachedNodes()
	if len(nodes) != 1 {
		t.Fatal("rollback must not discard the accepted feed")
	}

	body = signedFeed(t, priv, 11, now.Add(-6*time.Hour), 300)
	if _, err := f.SyncIfDue(context.Background(), true, false, 0); err == nil {
		t.Fatal("expected expired feed rejection")
	}

	body = signedFeed(t, priv, 12, now, 300)
	f.cfg.PublicKeyBase64 = ""
	if _, err := f.SyncIfDue(context.Background(), true, false, 0); err == nil {
		t.Fatal("expected unsigned/unpinned feed rejection")
	}
}
