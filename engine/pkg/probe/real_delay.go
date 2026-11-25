// Package probe implements true end-to-end Real Delay testing (HTTP 204 over active proxy),
// bounded concurrent worker pools, smart server scoring, and clickable IP health verification.
package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

const (
	MaxProbeConcurrency = 6
	DefaultProbeURL     = "https://www.gstatic.com/generate_204"
	FallbackProbeURL    = "https://cp.cloudflare.com/generate_204"
	ProbeTimeout        = 4 * time.Second
)

// NodeDialer measures real end-to-end latency through a candidate proxy node.
type NodeDialer interface {
	ProbeGenerate204(ctx context.Context, node model.Node, probeURL string) (realDelayMS int, err error)
}

// DirectDialer measures HTTP 204 latency directly (used for diagnostics and tests).
type DirectDialer struct{}

func (DirectDialer) ProbeGenerate204(ctx context.Context, _ model.Node, probeURL string) (int, error) {
	if probeURL == "" {
		probeURL = DefaultProbeURL
	}
	client := &http.Client{Timeout: ProbeTimeout}
	// Warmup request absorbs TLS/DNS setup; second request measures steady RTT.
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if err != nil {
			return 0, err
		}
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
		}
		if attempt == 1 {
			return int(time.Since(start).Milliseconds()), nil
		}
	}
	return 0, nil
}

// ProbeResult captures a measured node's real delay and computed smart score.
type ProbeResult struct {
	Node        model.Node
	RealDelayMS int
	SuccessRate float64
	FinalScore  float64
	Err         error
}

// RankNodes tests candidate nodes using a bounded worker pool (<= 6 concurrent probes)
// and sorts reachable nodes according to the user's selected mode and country preference.
func RankNodes(ctx context.Context, nodes []model.Node, mode model.EngineMode, country model.CountryPreference, dialer NodeDialer) []ProbeResult {
	candidates := filterCandidates(nodes, mode, country)
	if len(candidates) == 0 {
		return nil
	}

	sem := make(chan struct{}, MaxProbeConcurrency)
	results := make([]ProbeResult, len(candidates))
	var wg sync.WaitGroup

	for i, n := range candidates {
		wg.Add(1)
		go func(idx int, node model.Node) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = ProbeResult{Node: node, Err: ctx.Err()}
				return
			}

			probeCtx, cancel := context.WithTimeout(ctx, ProbeTimeout)
			defer cancel()

			ms, err := dialer.ProbeGenerate204(probeCtx, node, DefaultProbeURL)
			if err != nil {
				ms, err = dialer.ProbeGenerate204(probeCtx, node, FallbackProbeURL)
			}
			if err != nil || ms <= 0 {
				results[idx] = ProbeResult{Node: node, Err: err}
				return
			}

			score := computeScore(node, ms, mode)
			node.RealDelay = ms
			node.Score = score
			results[idx] = ProbeResult{
				Node:        node,
				RealDelayMS: ms,
				SuccessRate: 1.0,
				FinalScore:  score,
			}
		}(i, n)
	}
	wg.Wait()

	healthy := make([]ProbeResult, 0, len(results))
	for _, r := range results {
		if r.Err == nil && r.RealDelayMS > 0 {
			healthy = append(healthy, r)
		}
	}
	sort.Slice(healthy, func(i, j int) bool {
		return healthy[i].FinalScore > healthy[j].FinalScore
	})
	return healthy
}

func filterCandidates(nodes []model.Node, mode model.EngineMode, country model.CountryPreference) []model.Node {
	filtered := make([]model.Node, 0, len(nodes))
	for _, n := range nodes {
		if !country.Auto && country.Code != "" && !strings.EqualFold(n.Region, country.Code) {
			continue
		}
		if mode == model.EngineModeXrayAntiDPI && n.Engine != model.CoreXray {
			if n.Protocol != "vless" && n.Protocol != "trojan" {
				continue
			}
		}
		filtered = append(filtered, n)
	}
	return filtered
}

func computeScore(node model.Node, delayMS int, mode model.EngineMode) float64 {
	normLatency := 1.0 - float64(delayMS)/1500.0
	if normLatency < 0 {
		normLatency = 0
	}
	weight := node.Weight
	if weight <= 0 || weight > 1 {
		weight = 0.75
	}

	switch mode {
	case model.EngineModeSingBoxFast:
		return 0.65*normLatency + 0.35*weight
	case model.EngineModeXrayAntiDPI:
		bonus := 0.10
		if node.Engine == model.CoreXray {
			bonus = 0.25
		}
		return 0.45*normLatency + 0.30*weight + bonus
	default:
		return 0.55*normLatency + 0.45*weight
	}
}

// RunIPHealthCheck executes the user-triggered check when clicking the IP rectangle.
func RunIPHealthCheck(ctx context.Context, proxyHTTPPort int) (model.IPTestResult, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	if proxyHTTPPort > 0 {
		proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyHTTPPort))
		if err != nil {
			return model.IPTestResult{}, err
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

	info, fetchDuration, err := fetchPublicIPInfo(ctx, client)
	if err != nil {
		return model.IPTestResult{}, err
	}

	delayMS := int(fetchDuration.Milliseconds())
	iranOK := checkIranDirect(ctx)

	return model.IPTestResult{
		PublicIP:     info.IP,
		CountryCode:  info.Country,
		CountryName:  CountryNameFA(info.Country),
		RealDelayMS:  delayMS,
		DNSProtected: proxyHTTPPort > 0,
		IranDirectOK: iranOK,
		TestedAt:     time.Now().Unix(),
	}, nil
}

func fetchPublicIPInfo(ctx context.Context, client *http.Client) (struct {
	IP      string
	Country string
}, time.Duration, error) {
	type result = struct {
		IP      string
		Country string
	}
	// Several independent endpoints are tried because ipinfo.io and ipify are
	// intermittently blocked from Iranian ISPs and from some exit nodes.
	endpoints := []string{
		"https://ipwho.is/",
		"https://api.myip.com",
		"https://ipapi.co/json/",
		"https://ipinfo.io/json",
		"https://api.ipify.org?format=json",
		"https://ifconfig.me/all.json",
	}
	var lastErr error
	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "MLZ-VPN/1.0")
		requestStart := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("IP endpoint %s returned HTTP %d", endpoint, resp.StatusCode)
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			lastErr = err
			continue
		}
		out := result{}
		for _, key := range []string{"ip", "ip_addr", "query"} {
			if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
				out.IP = strings.TrimSpace(value)
				break
			}
		}
		for _, key := range []string{"country_code", "cc", "country"} {
			if value, ok := raw[key].(string); ok {
				candidate := strings.ToUpper(strings.TrimSpace(value))
				if len(candidate) == 2 {
					out.Country = candidate
					break
				}
			}
		}
		if out.IP != "" {
			return out, time.Since(requestStart), nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no public IP endpoint returned a usable response")
	}
	return result{}, 0, lastErr
}

func checkIranDirect(ctx context.Context) bool {
	directClient := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://dns.shecan.ir", nil)
	if err != nil {
		return false
	}
	resp, err := directClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// CountryNameFA maps common ISO codes to Persian display names.
func CountryNameFA(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "DE":
		return "آلمان"
	case "NL":
		return "هلند"
	case "FI":
		return "فنلاند"
	case "FR":
		return "فرانسه"
	case "CH":
		return "سوئیس"
	case "GB":
		return "انگلیس"
	case "US":
		return "آمریکا"
	case "TR":
		return "ترکیه"
	case "AE":
		return "امارات"
	case "IR":
		return "ایران"
	default:
		if code == "" {
			return "نامشخص"
		}
		return strings.ToUpper(code)
	}
}
