// Package ipc exposes a token-authenticated localhost control API used by the Flutter UI
// to communicate with both unprivileged Proxy and elevated UAC TUN mlz-engine processes on Windows.
package ipc

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/orchestrator"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/updater"
)

// Server wraps the orchestrator behind a constant-time Bearer-authenticated HTTP/JSON API.
type Server struct {
	orch       *orchestrator.Orchestrator
	token      string
	httpServer *http.Server
	shutdownFn func()
}

// NewServer constructs an authenticated local control API server.
func NewServer(orch *orchestrator.Orchestrator, bearerToken string, onShutdown func()) *Server {
	s := &Server{
		orch:       orch,
		token:      bearerToken,
		shutdownFn: onShutdown,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status", s.auth(s.handleStatus))
	mux.HandleFunc("/v1/connect", s.auth(s.handleConnect))
	mux.HandleFunc("/v1/disconnect", s.auth(s.handleDisconnect))
	mux.HandleFunc("/v1/test-ip", s.auth(s.handleTestIP))
	mux.HandleFunc("/v1/diagnostics", s.auth(s.handleDiagnostics))
	mux.HandleFunc("/v1/lan", s.auth(s.handleLAN))
	mux.HandleFunc("/v1/sync", s.auth(s.handleSync))
	mux.HandleFunc("/v1/updates", s.auth(s.handleCheckUpdate))
	mux.HandleFunc("/v1/download-update", s.auth(s.handleDownloadUpdate))
	mux.HandleFunc("/v1/shutdown", s.auth(s.handleShutdown))

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

// ListenAndServe binds strictly to loopback 127.0.0.1 and serves requests until closed.
func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.httpServer.Serve(ln)
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			hdr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(hdr), []byte(s.token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.orch.Snapshot())
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TunnelMode   model.TunnelMode        `json:"tunnel_mode"`
		EngineMode   model.EngineMode        `json:"engine_mode"`
		Country      model.CountryPreference `json:"country"`
		BatterySaver bool                    `json:"battery_saver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.orch.Connect(r.Context(), orchestrator.ConnectRequest{
		TunnelMode:   req.TunnelMode,
		EngineMode:   req.EngineMode,
		Country:      req.Country,
		BatterySaver: req.BatterySaver,
	}); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "status": s.orch.Snapshot()})
		return
	}
	writeJSON(w, http.StatusOK, s.orch.Snapshot())
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = s.orch.Disconnect()
	writeJSON(w, http.StatusOK, s.orch.Snapshot())
}

func (s *Server) handleTestIP(w http.ResponseWriter, r *http.Request) {
	res, err := s.orch.TestIPHealth(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	report := s.orch.RunSmartDiagnostics(r.Context())
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleLAN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusOK, s.orch.LANInfo())
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
		UsePIN  bool `json:"use_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := s.orch.SetLANSharing(r.Context(), req.Enabled, req.UsePIN)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	updated, err := s.orch.SyncFeed(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"updated": updated, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": updated, "status": s.orch.Snapshot()})
}

func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	res, err := s.orch.CheckAppUpdate(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDownloadUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL      string `json:"url"`
		SHA256   string `json:"sha256"`
		FileName string `json:"file_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path, err := s.orch.DownloadAppUpdate(r.Context(), updater.AssetInfo{URL: req.URL, SHA256: req.SHA256}, req.FileName)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (s *Server) handleShutdown(w http.ResponseWriter, _ *http.Request) {
	_ = s.orch.Disconnect()
	writeJSON(w, http.StatusOK, map[string]bool{"closing": true})
	if s.shutdownFn != nil {
		go func() {
			time.Sleep(100 * time.Millisecond)
			s.shutdownFn()
		}()
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
