// Command mlz-engine runs the MLZ VPN Go orchestrator on Windows (and Desktop)
// in either unprivileged Proxy mode or on-demand UAC-elevated TUN mode.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/cores"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/ipc"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/orchestrator"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/subscription"
)

func main() {
	modeFlag := flag.String("mode", "proxy", "tunnel mode: proxy | tun")
	ipcPort := flag.Int("ipc-port", 47890, "loopback IPC control port")
	token := flag.String("token", "", "bearer auth token for IPC API")
	cacheDir := flag.String("cache-dir", defaultCacheDir(), "directory for config cache and sidecars")
	ruleDir := flag.String("ruleset-dir", "./rules", "directory containing geosite-ir.srs and geoip-ir.srs")
	flag.Parse()

	elevated := *modeFlag == string(model.TunnelModeTUN)
	runner := cores.NewDesktopRunner(filepath.Join(*cacheDir, "cores"), filepath.Join(*cacheDir, "configs"))

	orch := orchestrator.New(orchestrator.Config{
		CurrentBuild:   100,
		CacheDir:       *cacheDir,
		RuleSetDir:     *ruleDir,
		IsElevated:     elevated,
		LocalMixedPort: 10809,
		LANSharePort:   10808,
		ClashAPIPort:   9090,
		FeedSource: subscription.FeedSourceConfig{
			PublicKeyBase64: "6ZcGl3tK1+MmN/ix3C14i//iC8kyBWt7vagvP7NfWm8=",
			FeedURLs: []string{
				"https://cdn.jsdelivr.net/gh/hidooch980/mlz-vpn@main/feeds/feed.v1.json",
				"https://raw.githubusercontent.com/hidooch980/mlz-vpn/main/feeds/feed.v1.json",
			},
		},
		ManifestURLs: []string{
			"https://github.com/hidooch980/mlz-vpn/releases/latest/download/version.json",
			"https://raw.githubusercontent.com/hidooch980/mlz-vpn/main/version.json",
		},
	}, runner)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go orch.StartBackgroundSync(ctx)

	srv := ipc.NewServer(orch, *token, cancel)
	addr := fmt.Sprintf("127.0.0.1:%d", *ipcPort)

	go func() {
		log.Printf("mlz-engine listening on http://%s (mode=%s)", addr, *modeFlag)
		if err := srv.ListenAndServe(addr); err != nil {
			log.Printf("IPC server stopped: %v", err)
		}
	}()

	<-ctx.Done()
	_ = orch.Disconnect()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
}

func defaultCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "./cache"
	}
	return filepath.Join(dir, "mlz-vpn")
}
