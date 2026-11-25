package probe

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

// SocksProxyDialer implements real end-to-end HTTP 204 probing through any local SOCKS5 proxy
// (sing-box test inbound, Xray SOCKS5 chain, or any All-In Sidecar) using pure Go stdlib.
type SocksProxyDialer struct {
	SocksAddr string
	Username  string
	Password  string
}

func (s SocksProxyDialer) ProbeGenerate204(ctx context.Context, _ model.Node, probeURL string) (int, error) {
	if probeURL == "" {
		probeURL = DefaultProbeURL
	}
	u, err := url.Parse(probeURL)
	if err != nil {
		return 0, err
	}
	host := u.Hostname()
	port := 443
	if u.Scheme == "http" {
		port = 80
	}
	if p := u.Port(); p != "" {
		if parsed, convErr := strconv.Atoi(p); convErr == nil {
			port = parsed
		}
	}

	transport := &http.Transport{
		DialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			return dialSOCKS5(dialCtx, s.SocksAddr, s.Username, s.Password, host, port)
		},
		TLSClientConfig: &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport, Timeout: ProbeTimeout}
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
			return 0, fmt.Errorf("unexpected proxy 204 status: %d", resp.StatusCode)
		}
		if attempt == 1 {
			return int(time.Since(start).Milliseconds()), nil
		}
	}
	return 0, nil
}

// dialSOCKS5 performs an RFC 1928 / RFC 1929 SOCKS5 CONNECT handshake over stdlib net.Conn.
func dialSOCKS5(ctx context.Context, socksAddr, user, pass, targetHost string, targetPort int) (net.Conn, error) {
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", socksAddr)
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	methods := []byte{0x05, 0x01, 0x00}
	if user != "" {
		methods = []byte{0x05, 0x02, 0x00, 0x02}
	}
	if _, err := conn.Write(methods); err != nil {
		_ = conn.Close()
		return nil, err
	}

	var reply [2]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if reply[0] != 0x05 {
		_ = conn.Close()
		return nil, errors.New("invalid SOCKS5 greeting version")
	}
	if reply[1] == 0x02 {
		if len(user) > 255 || len(pass) > 255 {
			_ = conn.Close()
			return nil, errors.New("SOCKS5 credentials too long")
		}
		authReq := make([]byte, 0, 3+len(user)+len(pass))
		authReq = append(authReq, 0x01, byte(len(user)))
		authReq = append(authReq, user...)
		authReq = append(authReq, byte(len(pass)))
		authReq = append(authReq, pass...)
		if _, err := conn.Write(authReq); err != nil {
			_ = conn.Close()
			return nil, err
		}
		var authReply [2]byte
		if _, err := io.ReadFull(conn, authReply[:]); err != nil || authReply[1] != 0x00 {
			_ = conn.Close()
			return nil, errors.New("SOCKS5 auth rejected")
		}
	} else if reply[1] != 0x00 {
		_ = conn.Close()
		return nil, errors.New("SOCKS5 no acceptable auth method")
	}

	if len(targetHost) > 255 {
		_ = conn.Close()
		return nil, errors.New("hostname too long for SOCKS5")
	}
	req := make([]byte, 0, 7+len(targetHost))
	req = append(req, 0x05, 0x01, 0x00, 0x03, byte(len(targetHost)))
	req = append(req, targetHost...)
	var portBuf [2]byte
	binary.BigEndian.PutUint16(portBuf[:], uint16(targetPort))
	req = append(req, portBuf[:]...)

	if _, err := conn.Write(req); err != nil {
		_ = conn.Close()
		return nil, err
	}

	var hdr [4]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if hdr[1] != 0x00 {
		_ = conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect failed with code %d", hdr[1])
	}

	var addrLen int
	switch hdr[3] {
	case 0x01:
		addrLen = 4
	case 0x04:
		addrLen = 16
	case 0x03:
		var ln [1]byte
		if _, err := io.ReadFull(conn, ln[:]); err != nil {
			_ = conn.Close()
			return nil, err
		}
		addrLen = int(ln[0])
	default:
		_ = conn.Close()
		return nil, errors.New("unknown SOCKS5 ATYP")
	}
	discard := make([]byte, addrLen+2)
	if _, err := io.ReadFull(conn, discard); err != nil {
		_ = conn.Close()
		return nil, err
	}

	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}
