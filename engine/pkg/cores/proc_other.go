//go:build !windows

package cores

import "os/exec"

func hideWindow(_ *exec.Cmd) {}
