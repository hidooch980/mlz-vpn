package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/parser"
)

// FeedSourceConfig defines the signed GitHub/CDN mirrors, trusted Ed25519 public key,
// and embedded emergency seed URIs used when offline or under heavy cold-start censorship.
type FeedSourceConfig struct {
	PublicKeyBase64 string
	FeedURLs        []string
	EmbeddedSeeds   []model.FeedNode
	// StatePath persists the last accepted signed sequence across restarts.
	StatePath string
}

// Fetcher maintains cached verified nodes and refreshes every 5 minutes (15 min in battery saver).
type Fetcher struct {
	mu           sync.RWMutex
	cfg          FeedSourceConfig
	lastETag     string
	lastSequence int64
	lastFetched  time.Time
	cachedFeed   *model.FeedPayload
	cachedNodes  []model.Node
	activeClient *http.Client
}

// NewFetcher constructs a 5-minute signed feed fetcher.
func NewFetcher(cfg FeedSourceConfig) *Fetcher {
	f := &Fetcher{
		cfg:          cfg,
		activeClient: &http.Client{Timeout: 8 * time.Second},
	}
	if cfg.StatePath != "" {
		if raw, err := os.ReadFile(cfg.StatePath); err == nil {
			if seq, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil && seq > 0 {
				f.lastSequence = seq
			}
		}
	}
	return f
}

// CachedNodes returns currently verified nodes (or embedded seeds before first online sync).
func (f *Fetcher) CachedNodes() ([]model.Node, time.Time) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.cachedNodes) > 0 {
		out := make([]model.Node, len(f.cachedNodes))
		copy(out, f.cachedNodes)
		return out, f.lastFetched
	}
	return convertFeedNodes(f.cfg.EmbeddedSeeds), f.lastFetched
}

// SyncIfDue refreshes the signed feed when the 5-minute (or 15-minute battery) window elapses.
// localHTTPProxyPort > 0 routes the fetch through the active tunnel; 0 falls back to direct mirrors.
func (f *Fetcher) SyncIfDue(ctx context.Context, force bool, batterySaver bool, localHTTPProxyPort int) (updated bool, err error) {
	f.mu.RLock()
	last := f.lastFetched
	etag := f.lastETag
	f.mu.RUnlock()

	if !force && !ShouldRefresh(last, time.Now(), batterySaver) {
		return false, nil
	}

	client := f.clientForPort(localHTTPProxyPort)
	var lastErr error
	for _, mirrorURL := range f.cfg.FeedURLs {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, mirrorURL, nil)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}

		resp, doErr := client.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}

		if resp.StatusCode == http.StatusNotModified {
			resp.Body.Close()
			f.mu.Lock()
			f.lastFetched = time.Now()
			f.mu.Unlock()
			return false, nil
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("mirror %s HTTP %d", mirrorURL, resp.StatusCode)
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		newETag := resp.Header.Get("ETag")
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		var payload model.FeedPayload
		if jsonErr := json.Unmarshal(body, &payload); jsonErr != nil {
			lastErr = jsonErr
			continue
		}

		// Fail closed: an unsigned feed is never accepted, even if configuration
		// accidentally omits the pinned Ed25519 public key.
		if sigErr := VerifyFeedSignature(f.cfg.PublicKeyBase64, &payload); sigErr != nil {
			lastErr = sigErr
			continue
		}
		if freshErr := ValidateFeedFreshness(&payload, time.Now()); freshErr != nil {
			lastErr = freshErr
			continue
		}

		nodes := convertFeedNodes(payload.Nodes)
		if len(nodes) == 0 {
			lastErr = errors.New("verified feed contained 0 valid nodes")
			continue
		}

		f.mu.Lock()
		if f.lastSequence > 0 && payload.Sequence <= f.lastSequence {
			f.mu.Unlock()
			lastErr = fmt.Errorf("%w: got %d, have %d", ErrFeedRollback, payload.Sequence, f.lastSequence)
			continue
		}
		f.cachedFeed = &payload
		f.cachedNodes = nodes
		f.lastETag = newETag
		f.lastSequence = payload.Sequence
		f.lastFetched = time.Now()
		statePath := f.cfg.StatePath
		f.mu.Unlock()
		if statePath != "" {
			_ = writeSequence(statePath, payload.Sequence)
		}
		return true, nil
	}

	return false, lastErr
}

func writeSequence(path string, seq int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.FormatInt(seq, 10)), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (f *Fetcher) clientForPort(proxyPort int) *http.Client {
	if proxyPort <= 0 {
		return f.activeClient
	}
	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
	if err != nil {
		return f.activeClient
	}
	return &http.Client{
		Timeout:   8 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(u)},
	}
}

func convertFeedNodes(feedNodes []model.FeedNode) []model.Node {
	out := make([]model.Node, 0, len(feedNodes))
	for _, fn := range feedNodes {
		parsed, err := parser.ParseURI(fn.ConfigURI)
		if err != nil {
			continue
		}
		if fn.ID != "" {
			parsed.ID = fn.ID
		}
		if fn.Engine != "" {
			parsed.Engine = fn.Engine
		}
		if fn.Protocol != "" {
			parsed.Protocol = fn.Protocol
		}
		if fn.Region != "" {
			parsed.Region = fn.Region
		}
		parsed.Tags = fn.Tags
		parsed.Weight = fn.Weight
		out = append(out, parsed)
	}
	return out
}
