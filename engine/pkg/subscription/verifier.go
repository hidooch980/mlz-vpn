// Package subscription manages the 5-minute signed feed lifecycle,
// Ed25519 authenticity verification, and battery-aware refresh intervals.
package subscription

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

const (
	DefaultRefreshInterval      = 5 * time.Minute
	BatterySaverRefreshInterval = 15 * time.Minute
	ExpectedFeedSchema          = "mlz.feed.v1"
)

var (
	ErrInvalidSchema    = errors.New("unexpected feed schema")
	ErrInvalidSignature = errors.New("feed Ed25519 signature verification failed")
	ErrInvalidPublicKey = errors.New("invalid Ed25519 public key")
)

type canonicalFeed struct {
	Schema      string           `json:"schema"`
	Sequence    int64            `json:"sequence"`
	GeneratedAt time.Time        `json:"generated_at"`
	TTLSeconds  int              `json:"ttl_seconds"`
	Mirrors     []string         `json:"mirrors,omitempty"`
	Nodes       []model.FeedNode `json:"nodes"`
}

// VerifyFeedSignature validates that payload was signed by the trusted Ed25519 private key.
func VerifyFeedSignature(pubKeyBase64 string, payload *model.FeedPayload) error {
	if payload == nil {
		return errors.New("nil feed payload")
	}
	if payload.Schema != ExpectedFeedSchema {
		return fmt.Errorf("%w: %s", ErrInvalidSchema, payload.Schema)
	}

	pubBytes, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return ErrInvalidPublicKey
	}

	sigBytes, err := base64.StdEncoding.DecodeString(payload.Signature)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}

	canonical := canonicalFeed{
		Schema:      payload.Schema,
		Sequence:    payload.Sequence,
		GeneratedAt: payload.GeneratedAt.UTC(),
		TTLSeconds:  payload.TTLSeconds,
		Mirrors:     payload.Mirrors,
		Nodes:       payload.Nodes,
	}
	msg, err := json.Marshal(canonical)
	if err != nil {
		return fmt.Errorf("marshal canonical feed: %w", err)
	}

	if !ed25519.Verify(ed25519.PublicKey(pubBytes), msg, sigBytes) {
		return ErrInvalidSignature
	}
	return nil
}

// RefreshInterval returns 5 minutes normally, or 15 minutes when mobile battery saver is active.
func RefreshInterval(batterySaver bool) time.Duration {
	if batterySaver {
		return BatterySaverRefreshInterval
	}
	return DefaultRefreshInterval
}

// ShouldRefresh reports whether the cached feed has exceeded its refresh window.
func ShouldRefresh(lastFetched time.Time, now time.Time, batterySaver bool) bool {
	if lastFetched.IsZero() {
		return true
	}
	return now.Sub(lastFetched) >= RefreshInterval(batterySaver)
}
