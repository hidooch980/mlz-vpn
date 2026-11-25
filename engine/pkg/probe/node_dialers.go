package probe

import (
	"context"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

// TCPDialer measures real TCP reachability/latency for a node endpoint. It does
// not authenticate the proxy protocol, but it filters dead hosts and is used as
// a fallback when a protocol-aware local core prober is unavailable.
type TCPDialer struct {
	Timeout time.Duration
}

func (d TCPDialer) ProbeGenerate204(ctx context.Context, node model.Node, _ string) (int, error) {
	if node.Address == "" || node.Port <= 0 {
		return 0, errors.New("invalid node endpoint")
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = ProbeTimeout
	}
	dialer := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(node.Address, strconv.Itoa(node.Port)))
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	ms := int(time.Since(start).Milliseconds())
	if ms <= 0 {
		ms = 1
	}
	return ms, nil
}

// MeasuredDialer prefers protocol-aware delays measured by a temporary local
// core and falls back to the supplied dialer for nodes without a sample.
type MeasuredDialer struct {
	Delays   map[string]int
	Fallback NodeDialer
}

func (d MeasuredDialer) ProbeGenerate204(ctx context.Context, node model.Node, probeURL string) (int, error) {
	if ms, ok := d.Delays[node.ID]; ok && ms > 0 {
		return ms, nil
	}
	if d.Fallback == nil {
		return 0, errors.New("no delay sample and no fallback dialer")
	}
	return d.Fallback.ProbeGenerate204(ctx, node, probeURL)
}
