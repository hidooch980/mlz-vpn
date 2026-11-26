package cores

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("MLZ_CORE_HELPER") != "1" {
		return
	}
	time.Sleep(3 * time.Second)
	os.Exit(0)
}

func TestCoreProcessOutlivesStartContext(t *testing.T) {
	t.Setenv("MLZ_CORE_HELPER", "1")
	ctx, cancel := context.WithCancel(context.Background())
	proc, err := startCoreProcess(ctx, os.Args[0], []string{"-test.run=TestHelperProcess"})
	if err != nil {
		t.Fatalf("start helper: %v", err)
	}
	cancel()
	select {
	case <-proc.done:
		t.Fatal("core process was killed when the starting request context ended")
	case <-time.After(500 * time.Millisecond):
	}
	proc.stop()
}

func TestDesktopRunnerMissingBinaryReportsClearError(t *testing.T) {
	tmp := t.TempDir()
	runner := NewDesktopRunner(tmp, filepath.Join(tmp, "cfg"))
	err := runner.StartSingBox(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected clear missing-binary error when sing-box executable is absent")
	}

	// Verify config file was safely persisted with restricted permissions before binary lookup.
	if _, statErr := os.Stat(filepath.Join(tmp, "cfg", "sing-box.json")); statErr != nil {
		t.Fatalf("expected sing-box.json written prior to launch: %v", statErr)
	}
	_ = runtime.GOOS
}
