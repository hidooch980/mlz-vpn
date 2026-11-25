package cores

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClashClient talks to sing-box's local Clash REST API for real delay URLTest
// and live upload/download throughput telemetry.
type ClashClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

// NewClashClient constructs a local Clash API client (e.g. http://127.0.0.1:9090).
func NewClashClient(baseURL, secret string) *ClashClient {
	return &ClashClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		secret:  secret,
		client:  &http.Client{Timeout: 8 * time.Second},
	}
}

// Ready checks that the local Clash API is serving requests.
func (c *ClashClient) Ready(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/version", nil)
	if err != nil {
		return err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clash /version returned HTTP %d", resp.StatusCode)
	}
	return nil
}

type clashTrafficSample struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// StreamTraffic reads live per-second upload/download rates from GET /traffic until ctx is canceled.
func (c *ClashClient) StreamTraffic(ctx context.Context, onSample func(upBPS, downBPS int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/traffic", nil)
	if err != nil {
		return err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	// Streaming connection stays open; disable client-wide timeout and rely on ctx.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clash /traffic returned HTTP %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 4<<20))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var sample clashTrafficSample
		if err := json.Unmarshal([]byte(line), &sample); err != nil {
			continue
		}
		if onSample != nil {
			onSample(sample.Up, sample.Down)
		}
	}
	return scanner.Err()
}

// TestDelay asks sing-box's Clash API to measure real HTTP URLTest delay through proxyTag.
func (c *ClashClient) TestDelay(ctx context.Context, proxyTag, probeURL string, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	endpoint := fmt.Sprintf(
		"%s/proxies/%s/delay?url=%s&timeout=%d",
		c.baseURL,
		url.PathEscape(proxyTag),
		url.QueryEscape(probeURL),
		timeout.Milliseconds(),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("clash delay probe HTTP %d", resp.StatusCode)
	}

	var out struct {
		Delay int `json:"delay"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	if out.Delay <= 0 {
		return 0, fmt.Errorf("invalid delay sample %d", out.Delay)
	}
	return out.Delay, nil
}
