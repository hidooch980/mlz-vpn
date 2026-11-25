//go:build !windows

package orchestrator

// ApplySystemProxy is a no-op outside Windows (Android uses VpnService TUN directly).
func ApplySystemProxy(_ int) error { return nil }

// ClearSystemProxy is a no-op outside Windows.
func ClearSystemProxy() error { return nil }
