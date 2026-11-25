//go:build windows

package orchestrator

import (
	"fmt"
	"os/exec"
)

// ApplySystemProxy configures Windows per-user HKCU Internet Settings for non-elevated Proxy mode.
func ApplySystemProxy(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f").Run(); err != nil {
		return err
	}
	if err := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyServer", "/t", "REG_SZ", "/d", addr, "/f").Run(); err != nil {
		return err
	}
	return exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyOverride", "/t", "REG_SZ", "/d", "localhost;127.*;10.*;172.16.*;192.168.*;*.ir;<local>", "/f").Run()
}

// ClearSystemProxy disables the Windows user proxy cleanly on disconnect or shutdown.
func ClearSystemProxy() error {
	return exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f").Run()
}
