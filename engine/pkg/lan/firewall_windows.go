//go:build windows

package lan

import (
	"fmt"
	"os/exec"
)

const firewallRuleName = "MLZ VPN LAN Sharing Proxy"

// OpenFirewallPort opens TCP traffic on port for private/hotspot networks during an elevated TUN session.
func OpenFirewallPort(port int) error {
	_ = CloseFirewallPort()
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+firewallRuleName,
		"dir=in",
		"action=allow",
		"protocol=TCP",
		fmt.Sprintf("localport=%d", port),
		"profile=private,public",
	)
	return cmd.Run()
}

// CloseFirewallPort removes the temporary LAN sharing rule on disconnect/shutdown.
func CloseFirewallPort() error {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+firewallRuleName,
	)
	return cmd.Run()
}
