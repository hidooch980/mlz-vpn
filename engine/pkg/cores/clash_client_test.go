package cores

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClashClientTrafficAndDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/traffic":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"up":1200,"down":48000}`)
			_, _ = fmt.Fprintln(w, `{"up":2400,"down":96000}`)
		case "/proxies/proxy-out/delay":
			_, _ = w.Write([]byte(`{"delay":43}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClashClient(srv.URL, "tok")
	ms, err := client.TestDelay(context.Background(), "proxy-out", "https://www.gstatic.com/generate_204", 3*time.Second)
	if err != nil || ms != 43 {
		t.Fatalf("expected 43ms delay, got ms=%d err=%v", ms, err)
	}

	var samples int
	var lastDown int64
	_ = client.StreamTraffic(context.Background(), func(_, down int64) {
		samples++
		lastDown = down
	})
	if samples != 2 || lastDown != 96000 {
		t.Fatalf("expected 2 traffic samples ending with 96000, got samples=%d lastDown=%d", samples, lastDown)
	}
}
