//go:build !windows

package lan

// OpenFirewallPort is a no-op on Android (VpnService allows LAN binding directly when configured).
func OpenFirewallPort(_ int) error { return nil }

// CloseFirewallPort is a no-op on non-Windows platforms.
func CloseFirewallPort() error { return nil }
