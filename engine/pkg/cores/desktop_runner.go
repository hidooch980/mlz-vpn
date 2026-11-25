package cores

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/model"
)

// DesktopRunner manages real sing-box and xray subprocesses on Windows/Desktop,
// keeping sing-box as the sole TUN/Split-DNS owner and xray as an optional SOCKS5-chained core.
type DesktopRunner struct {
	mu        sync.Mutex
	binDir    string
	configDir string
	singBox   *coreProcess
	xray      *coreProcess
}

// NewDesktopRunner initializes a process-backed runner storing generated configs under configDir
// and looking for core binaries (sing-box, xray) inside binDir (or alongside executable / PATH).
func NewDesktopRunner(binDir, configDir string) *DesktopRunner {
	return &DesktopRunner{
		binDir:    binDir,
		configDir: configDir,
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type coreProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	log  lockedBuffer
	mu   sync.Mutex
	err  error
}

func startCoreProcess(ctx context.Context, bin string, args []string) (*coreProcess, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	p := &coreProcess{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout = &p.log
	cmd.Stderr = &p.log
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

func (p *coreProcess) stop() {
	if p == nil {
		return
	}
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
	}
}

func (p *coreProcess) failure() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	err := p.err
	p.mu.Unlock()
	if err == nil {
		select {
		case <-p.done:
			return p.log.String()
		default:
			return ""
		}
	}
	msg := p.log.String()
	if msg == "" {
		return err.Error()
	}
	return err.Error() + ": " + msg
}

// StartSingBox writes active sing-box JSON and starts the sing-box run subprocess.
func (r *DesktopRunner) StartSingBox(ctx context.Context, singBoxJSON []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.singBox != nil {
		r.singBox.stop()
		r.singBox = nil
	}
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
	proc, err := startCoreProcess(ctx, bin, []string{"run", "-c", cfgPath})
	if err != nil {
		return fmt.Errorf("failed to start sing-box: %w", err)
	}
	r.singBox = proc
	return nil
}

// StopSingBox terminates the managed sing-box subprocess if running.
func (r *DesktopRunner) StopSingBox() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.singBox != nil {
		r.singBox.stop()
		r.singBox = nil
	}
	return nil
}

// SingBoxFailure returns captured stderr/stdout when the managed sing-box process
// exited unexpectedly (useful for self-healing diagnostics).
func (r *DesktopRunner) SingBoxFailure() string {
	r.mu.Lock()
	proc := r.singBox
	r.mu.Unlock()
	return proc.failure()
}

// StartXray writes active xray JSON and starts the xray run subprocess.
func (r *DesktopRunner) StartXray(ctx context.Context, xrayJSON []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.xray != nil {
		r.xray.stop()
		r.xray = nil
	}
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
	proc, err := startCoreProcess(ctx, bin, []string{"run", "-c", cfgPath})
	if err != nil {
		return fmt.Errorf("failed to start xray-core: %w", err)
	}
	r.xray = proc
	return nil
}

// StopXray terminates the managed xray subprocess if running.
func (r *DesktopRunner) StopXray() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.xray != nil {
		r.xray.stop()
		r.xray = nil
	}
	return nil
}

// ProbeNodeDelays starts a temporary sing-box instance exposing every candidate
// as a distinct outbound, then measures true end-to-end delay through the Clash
// API. It is used by the orchestrator to rank servers by real proxy latency.
func (r *DesktopRunner) ProbeNodeDelays(ctx context.Context, nodes []model.Node) (map[string]int, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes to probe")
	}
	bin, err := r.resolveCoreBinary("sing-box")
	if err != nil {
		return nil, err
	}
	mixedPort, err := AllocateLoopbackPortInt()
	if err != nil {
		return nil, err
	}
	clashPort, err := AllocateLoopbackPortInt()
	if err != nil {
		return nil, err
	}
	secret := RandomCredential()
	data, tags, err := BuildSingBoxProbeConfig(nodes, mixedPort, clashPort, secret)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(r.configDir, 0o755); err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(r.configDir, "probe-sing-box.json")
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		return nil, err
	}
	proc, err := startCoreProcess(ctx, bin, []string{"run", "-c", cfgPath})
	if err != nil {
		return nil, err
	}
	defer proc.stop()

	client := NewClashClient(fmt.Sprintf("http://127.0.0.1:%d", clashPort), secret)
	deadline := time.NewTimer(4 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
readyLoop:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-proc.done:
			return nil, fmt.Errorf("probe sing-box exited: %s", proc.failure())
		case <-deadline.C:
			return nil, fmt.Errorf("probe sing-box did not become ready")
		case <-ticker.C:
			probeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			err := client.Ready(probeCtx)
			cancel()
			if err == nil {
				break readyLoop
			}
		}
	}

	delays := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, node := range nodes {
		tag := tags[i]
		if tag == "" || node.ID == "" {
			continue
		}
		wg.Add(1)
		go func(node model.Node, tag string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			ms, err := client.TestDelay(probeCtx, tag, "https://www.gstatic.com/generate_204", 4*time.Second)
			if err != nil || ms <= 0 {
				return
			}
			mu.Lock()
			delays[node.ID] = ms
			mu.Unlock()
		}(node, tag)
	}
	wg.Wait()
	return delays, nil
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
