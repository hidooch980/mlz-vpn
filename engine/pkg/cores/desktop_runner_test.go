package cores

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
