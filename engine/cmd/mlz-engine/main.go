// Command mlz-engine runs the MLZ VPN Go orchestrator on Windows (and Desktop)
// in either unprivileged Proxy mode or on-demand UAC-elevated TUN mode.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/cores"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/ipc"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/orchestrator"
	"github.com/mlz-vpn/mlz-vpn/engine/pkg/subscription"
)

// version and buildNumber are overridden at build time with -ldflags -X.
var (
	version     = "1.0.3"
	buildNumber = "103"

	// MLZ release signing public key (same Ed25519 identity as the signed feed).
	updatePublicKey = "6ZcGl3tK1+MmN/ix3C14i//iC8kyBWt7vagvP7NfWm8="
)

func main() {
	modeFlag := flag.String("mode", "proxy", "tunnel mode: proxy | tun")
	ipcPort := flag.Int("ipc-port", 47890, "loopback IPC control port")
	instancePort := flag.Int("instance-port", 47891, "loopback single-instance lock port")
	token := flag.String("token", "", "bearer auth token for IPC API")
	cacheDir := flag.String("cache-dir", defaultCacheDir(), "directory for config cache and sidecars")
	ruleDir := flag.String("ruleset-dir", "./rules", "directory containing geosite-ir.srs and geoip-ir.srs")
	flag.Parse()

	elevated := *modeFlag == string(model.TunnelModeTUN)
	runner := cores.NewDesktopRunner(filepath.Join(*cacheDir, "cores"), filepath.Join(*cacheDir, "configs"))

	build, err := strconv.Atoi(buildNumber)
	if err != nil || build <= 0 {
		build = 100
	}

	orch := orchestrator.New(orchestrator.Config{
		CurrentBuild:    build,
		CurrentVersion:  version,
		UpdatePublicKey: updatePublicKey,
		CacheDir:        *cacheDir,
		RuleSetDir:      *ruleDir,
		IsElevated:      elevated,
		LocalMixedPort:  10809,
		LANSharePort:    10808,
		ClashAPIPort:    9090,
		FeedSource: subscription.FeedSourceConfig{
			PublicKeyBase64: "6ZcGl3tK1+MmN/ix3C14i//iC8kyBWt7vagvP7NfWm8=",
			EmbeddedSeeds:   subscription.EmbeddedSeeds(),
			FeedURLs: []string{
				"https://raw.githubusercontent.com/hidooch980/mlz-vpn/main/feeds/feed.v1.json",
				"https://cdn.jsdelivr.net/gh/hidooch980/mlz-vpn@main/feeds/feed.v1.json",
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

	// One engine instance per machine: a stale proxy/TUN engine would otherwise
	// fight over sing-box's mixed and Clash API ports.
	var lockLn net.Listener
	for attempt := 0; attempt < 50; attempt++ {
		lockLn, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *instancePort))
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lockLn == nil {
		log.Printf("another mlz-engine instance is already running: %v", err)
		return
	}
	defer lockLn.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", *ipcPort)
	ipcLn, listenErr := net.Listen("tcp", addr)
	if listenErr != nil {
		log.Printf("cannot bind IPC listener on %s: %v", addr, listenErr)
		return
	}
	srv := ipc.NewServer(orch, *token, cancel)

	go func() {
		log.Printf("mlz-engine listening on http://%s (mode=%s, version=%s)", addr, *modeFlag, version)
		if err := srv.Serve(ipcLn); err != nil && err != http.ErrServerClosed {
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
