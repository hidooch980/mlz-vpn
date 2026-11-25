// Package lan discovers local hotspot / LAN interface addresses, generates PINs
// for shared proxy access, and manages temporary Windows Firewall rules.
package lan

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"strings"
)

// ShareInfo summarizes active LAN/Hotspot proxy settings for display and QR generation.
type ShareInfo struct {
	Enabled         bool     `json:"enabled"`
	PrimaryIP       string   `json:"primary_ip"`
	CandidateIPs    []string `json:"candidate_ips"`
	Port            int      `json:"port"`
	Username        string   `json:"username,omitempty"`
	PIN             string   `json:"pin,omitempty"`
	FirewallManaged bool     `json:"firewall_managed"`
}

// DetectLANAddresses discovers active private IPv4 addresses, prioritizing Windows Hotspot (192.168.137.1)
// and Android Hotspot (192.168.43.1) ranges first.
func DetectLANAddresses() (primary string, all []string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "192.168.1.1", nil
	}

	var priority []string
	var others []string

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipNet.IP.To4()
			if v4 == nil || !v4.IsPrivate() {
				continue
			}
			ipStr := v4.String()
			if strings.HasPrefix(ipStr, "192.168.137.") || strings.HasPrefix(ipStr, "192.168.43.") {
				priority = append(priority, ipStr)
			} else {
				others = append(others, ipStr)
			}
		}
	}

	all = append(priority, others...)
	if len(all) > 0 {
		return all[0], all
	}
	return "192.168.137.1", []string{"192.168.137.1"}
}

// GeneratePIN creates a 4-digit random numeric PIN for optional LAN proxy authentication.
func GeneratePIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return "4829"
	}
	return fmt.Sprintf("%04d", 1000+n.Int64())
}
