//go:build windows

package orchestrator

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

const internetSettingsKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type proxySnapshot struct {
	mu          sync.Mutex
	active      bool
	enable      string
	server      string
	override    string
	hadEnable   bool
	hadServer   bool
	hadOverride bool
}

var systemProxyState proxySnapshot

// queryRegistryValue reads a REG_SZ/REG_DWORD value; ok is false when absent.
func queryRegistryValue(name string) (string, bool) {
	out, err := exec.Command("reg", "query", internetSettingsKey, "/v", name).Output()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, name) || !strings.Contains(line, "REG_") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		return fields[len(fields)-1], true
	}
	return "", false
}

func setRegistryValue(name, typ, value string) error {
	return exec.Command("reg", "add", internetSettingsKey, "/v", name, "/t", typ, "/d", value, "/f").Run()
}

func deleteRegistryValue(name string) error {
	return exec.Command("reg", "delete", internetSettingsKey, "/v", name, "/f").Run()
}

// ApplySystemProxy configures Windows per-user Internet Settings for non-elevated
// Proxy mode. The previous values are captured exactly once so Disconnect can
// restore the user's own proxy configuration instead of blindly disabling it.
func ApplySystemProxy(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	systemProxyState.mu.Lock()
	defer systemProxyState.mu.Unlock()

	if !systemProxyState.active {
		systemProxyState.enable, systemProxyState.hadEnable = queryRegistryValue("ProxyEnable")
		systemProxyState.server, systemProxyState.hadServer = queryRegistryValue("ProxyServer")
		systemProxyState.override, systemProxyState.hadOverride = queryRegistryValue("ProxyOverride")
		systemProxyState.active = true
	}

	if err := setRegistryValue("ProxyEnable", "REG_DWORD", "1"); err != nil {
		restoreProxyLocked()
		return err
	}
	if err := setRegistryValue("ProxyServer", "REG_SZ", addr); err != nil {
		restoreProxyLocked()
		return err
	}
	if err := setRegistryValue("ProxyOverride", "REG_SZ", "localhost;127.*;10.*;172.16.*;192.168.*;*.ir;<local>"); err != nil {
		restoreProxyLocked()
		return err
	}
	return nil
}

// ClearSystemProxy restores the exact Windows proxy values captured by ApplySystemProxy.
func ClearSystemProxy() error {
	systemProxyState.mu.Lock()
	defer systemProxyState.mu.Unlock()
	if !systemProxyState.active {
		return nil
	}
	return restoreProxyLocked()
}

func restoreProxyLocked() error {
	var firstErr error
	restore := func(name, typ, value string, had bool) {
		var err error
		if had {
			err = setRegistryValue(name, typ, value)
		} else {
			_ = deleteRegistryValue(name)
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	restore("ProxyEnable", "REG_DWORD", systemProxyState.enable, systemProxyState.hadEnable)
	restore("ProxyServer", "REG_SZ", systemProxyState.server, systemProxyState.hadServer)
	restore("ProxyOverride", "REG_SZ", systemProxyState.override, systemProxyState.hadOverride)
	systemProxyState.active = false
	return firstErr
}
