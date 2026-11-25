package probe

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestDialSOCKS5Handshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()

		var greet [4]byte
		if _, err := io.ReadFull(c, greet[:]); err != nil {
			return
		}
		// Require username/password auth (0x02)
		_, _ = c.Write([]byte{0x05, 0x02})

		var authHdr [2]byte
		_, _ = io.ReadFull(c, authHdr[:])
		buf := make([]byte, int(authHdr[1])+1)
		_, _ = io.ReadFull(c, buf)
		pass := make([]byte, int(buf[len(buf)-1]))
		_, _ = io.ReadFull(c, pass)
		_, _ = c.Write([]byte{0x01, 0x00})

		var reqHdr [5]byte
		_, _ = io.ReadFull(c, reqHdr[:])
		rest := make([]byte, int(reqHdr[4])+2)
		_, _ = io.ReadFull(c, rest)
		_, _ = c.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 80})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := dialSOCKS5(ctx, ln.Addr().String(), "mlz", "secret", "gstatic.com", 443)
	if err != nil {
		t.Fatalf("dialSOCKS5 failed: %v", err)
	}
	_ = conn.Close()
	<-done
}
