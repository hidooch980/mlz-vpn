package cores

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

// DesktopRunner manages real sing-box and xray subprocesses on Windows/Desktop,
// keeping sing-box as the sole TUN/Split-DNS owner and xray as an optional SOCKS5-chained core.
type DesktopRunner struct {
	mu         sync.Mutex
	binDir     string
	configDir  string
	singBoxCmd *exec.Cmd
	xrayCmd    *exec.Cmd
}

// NewDesktopRunner initializes a process-backed runner storing generated configs under configDir
// and looking for core binaries (sing-box, xray) inside binDir (or alongside executable / PATH).
func NewDesktopRunner(binDir, configDir string) *DesktopRunner {
	return &DesktopRunner{
		binDir:    binDir,
		configDir: configDir,
	}
}

// StartSingBox writes active sing-box JSON and starts `sing-box run -c <path>`.
func (r *DesktopRunner) StartSingBox(ctx context.Context, singBoxJSON []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_ = r.stopCmd(&r.singBoxCmd)
	if err := os.MkdirAll(r.configDir, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(r.configDir, "sing-box.json")
	if err := os.WriteFile(cfgPath, singBoxJSON, 0o600); err != nil {
		return err
	}

	bin, err := r.resolveCoreBinary("sing-box")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "run", "-c", cfgPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sing-box: %w", err)
	}
	r.singBoxCmd = cmd
	return nil
}

// StopSingBox terminates the managed sing-box subprocess if running.
func (r *DesktopRunner) StopSingBox() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopCmd(&r.singBoxCmd)
}

// StartXray writes active xray JSON and starts `xray run -c <path>`.
func (r *DesktopRunner) StartXray(ctx context.Context, xrayJSON []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_ = r.stopCmd(&r.xrayCmd)
	if err := os.MkdirAll(r.configDir, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(r.configDir, "xray.json")
	if err := os.WriteFile(cfgPath, xrayJSON, 0o600); err != nil {
		return err
	}

	bin, err := r.resolveCoreBinary("xray")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "run", "-c", cfgPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start xray-core: %w", err)
	}
	r.xrayCmd = cmd
	return nil
}

// StopXray terminates the managed xray subprocess if running.
func (r *DesktopRunner) StopXray() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopCmd(&r.xrayCmd)
}

func (r *DesktopRunner) resolveCoreBinary(base string) (string, error) {
	name := base
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	candidates := []string{
		filepath.Join(r.binDir, name),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), name),
			filepath.Join(filepath.Dir(exe), "cores", name),
		)
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	if fromPath, err := exec.LookPath(name); err == nil {
		return fromPath, nil
	}
	return "", fmt.Errorf("core binary %q not found in %s or PATH", name, r.binDir)
}

func (r *DesktopRunner) stopCmd(slot **exec.Cmd) error {
	if *slot != nil && (*slot).Process != nil {
		_ = (*slot).Process.Kill()
		_, _ = (*slot).Process.Wait()
	}
	*slot = nil
	return nil
}
