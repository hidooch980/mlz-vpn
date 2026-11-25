package ipc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/orchestrator"
)

type noopRunner struct{}

func (noopRunner) StartSingBox(_ context.Context, _ []byte) error { return nil }
func (noopRunner) StopSingBox() error                             { return nil }
func (noopRunner) StartXray(_ context.Context, _ []byte) error    { return nil }
func (noopRunner) StopXray() error                                { return nil }

func TestIPCAuthAndEndpoints(t *testing.T) {
	t.Setenv("MLZ_ALLOW_LOOPBACK_UPDATE", "1")
	orch := orchestrator.New(orchestrator.Config{CacheDir: t.TempDir()}, noopRunner{})
	srv := NewServer(orch, "secret-token-123", nil)

	reqNoAuth := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	recNoAuth := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(recNoAuth, reqNoAuth)
	if recNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", recNoAuth.Code)
	}

	reqAuth := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	reqAuth.Header.Set("Authorization", "Bearer secret-token-123")
	recAuth := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid Bearer token, got %d", recAuth.Code)
	}

	// Test LAN toggle endpoint
	lanBody := bytes.NewBufferString(`{"enabled":true,"use_pin":true}`)
	reqLAN := httptest.NewRequest(http.MethodPost, "/v1/lan", lanBody)
	reqLAN.Header.Set("Authorization", "Bearer secret-token-123")
	recLAN := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(recLAN, reqLAN)
	if recLAN.Code != http.StatusOK {
		t.Fatalf("expected 200 for /v1/lan, got %d", recLAN.Code)
	}
	var lanResp struct {
		Enabled bool   `json:"enabled"`
		PIN     string `json:"pin"`
	}
	if err := json.Unmarshal(recLAN.Body.Bytes(), &lanResp); err != nil || !lanResp.Enabled || len(lanResp.PIN) != 4 {
		t.Fatalf("unexpected /v1/lan response: %s", recLAN.Body.String())
	}

	// Test SHA-256 verified /v1/download-update endpoint
	assetBytes := []byte("fake-update-archive")
	sum := sha256.Sum256(assetBytes)
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(assetBytes)
	}))
	defer dlSrv.Close()

	dlPayload, _ := json.Marshal(map[string]string{
		"url":       dlSrv.URL + "/upd.zip",
		"sha256":    hex.EncodeToString(sum[:]),
		"file_name": "upd.zip",
	})
	reqDL := httptest.NewRequest(http.MethodPost, "/v1/download-update", bytes.NewReader(dlPayload))
	reqDL.Header.Set("Authorization", "Bearer secret-token-123")
	recDL := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(recDL, reqDL)
	if recDL.Code != http.StatusOK {
		t.Fatalf("expected 200 for /v1/download-update, got %d (%s)", recDL.Code, recDL.Body.String())
	}
	var dlResp map[string]string
	_ = json.Unmarshal(recDL.Body.Bytes(), &dlResp)
	if _, err := os.Stat(dlResp["path"]); err != nil {
		t.Fatalf("expected downloaded update file on disk: %v", err)
	}
}
