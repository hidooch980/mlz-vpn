//go:build android

package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/experimental/libbox"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/orchestrator"
)

// androidRunner embeds sing-box's libbox runtime in the gomobile AAR. Kotlin
// establishes the VpnService TUN, passes its fd here, and libbox consumes the
// fd while protecting every proxy socket through SocketProtector.
var (
	androidSetupOnce sync.Once
	androidSetupErr  error
)

type androidRunner struct {
	cacheDir    string
	tunFD       int32
	protector   SocketProtector
	networkJSON string

	mu     sync.Mutex
	server *libbox.CommandServer
	plat   *androidPlatform
}

// networkSnapshot is produced by Kotlin from ConnectivityManager and
// NetworkInterface. Go cannot enumerate Android interfaces through netlink:
// SELinux denies route dumps to untrusted apps, which used to abort startup.
type networkSnapshot struct {
	Default      string                     `json:"default"`
	DefaultIndex int32                      `json:"default_index"`
	Interfaces   []networkInterfaceSnapshot `json:"interfaces"`
}

type networkInterfaceSnapshot struct {
	Name       string   `json:"name"`
	Index      int32    `json:"index"`
	MTU        int32    `json:"mtu"`
	Flags      int32    `json:"flags"`
	Addresses  []string `json:"addresses"`
	DNSServers []string `json:"dns"`
	Type       int32    `json:"type"`
	Metered    bool     `json:"metered"`
}

func newAndroidRunner(cacheDir string, tunFD int32, protector SocketProtector, networkJSON string) orchestrator.CoreRunner {
	return &androidRunner{cacheDir: cacheDir, tunFD: tunFD, protector: protector, networkJSON: networkJSON}
}

func parseNetworkSnapshot(raw string) networkSnapshot {
	var snapshot networkSnapshot
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &snapshot)
	}
	return snapshot
}

func (a *androidRunner) ensureServer() error {
	if a.server != nil {
		return nil
	}
	a.plat = &androidPlatform{tunFD: a.tunFD, protector: a.protector, snapshot: parseNetworkSnapshot(a.networkJSON)}
	cache := filepath.Join(a.cacheDir, "libbox")
	_ = os.MkdirAll(cache, 0o700)
	androidSetupOnce.Do(func() {
		androidSetupErr = libbox.Setup(&libbox.SetupOptions{
			BasePath:        cache,
			WorkingPath:     cache,
			TempPath:        cache,
			FixAndroidStack: true,
			AppVersion:      "1.0.2",
		})
	})
	if androidSetupErr != nil {
		return androidSetupErr
	}
	server, err := libbox.NewCommandServer(&noopCommandHandler{}, a.plat)
	if err != nil {
		return err
	}
	if err := server.Start(); err != nil {
		server.Close()
		return err
	}
	a.server = server
	return nil
}

func (a *androidRunner) updateNetworkJSON(raw string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.networkJSON = raw
	if a.plat == nil {
		return
	}
	snapshot := parseNetworkSnapshot(raw)
	a.plat.mu.Lock()
	a.plat.snapshot = snapshot
	a.plat.mu.Unlock()
}

func (a *androidRunner) StartSingBox(_ context.Context, config []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.ensureServer(); err != nil {
		return err
	}
	if err := a.server.StartOrReloadService(string(config), &libbox.OverrideOptions{}); err != nil {
		// A failed start can leave the command server and its previous service
		// holding the mixed inbound port and cache file. Tear it down so a retry
		// starts from a clean runtime instead of failing forever with EADDRINUSE.
		_ = a.server.CloseService()
		a.server.Close()
		a.server = nil
		a.plat.releaseTunFDs()
		return err
	}
	return nil
}

func (a *androidRunner) StopSingBox() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server != nil {
		// Close only stops the command gRPC server; CloseService releases the
		// actual sing-box instance, inbound listeners, TUN fd and cache-file lock.
		_ = a.server.CloseService()
		a.server.Close()
		a.server = nil
	}
	if a.plat != nil {
		a.plat.releaseTunFDs()
	}
	return nil
}

// StartXray is intentionally unsupported: Android ships only the sing-box
// libbox runtime. The orchestrator falls back to sing-box or reports the error.
func (a *androidRunner) StartXray(_ context.Context, _ []byte) error {
	return errors.New("xray dataplane is not bundled on Android; use sing-box")
}

func (a *androidRunner) StopXray() error { return nil }

type androidPlatform struct {
	tunFD     int32
	protector SocketProtector
	snapshot  networkSnapshot

	mu           sync.Mutex
	openedMu     sync.Mutex
	openedTunFDs []int
	myTunName    string
	interfaces   []net.Interface
	monitorStop  chan struct{}
}

func (p *androidPlatform) LocalDNSTransport() libbox.LocalDNSTransport { return nil }

func (p *androidPlatform) UsePlatformAutoDetectInterfaceControl() bool { return true }

func (p *androidPlatform) AutoDetectInterfaceControl(fd int32) error {
	if p.protector == nil || !p.protector.Protect(fd) {
		return errors.New("VpnService.protect failed")
	}
	return nil
}

func (p *androidPlatform) OpenTun(_ libbox.TunOptions) (int32, error) {
	// libbox duplicates the descriptor internally before handing it to the TUN
	// stack, but it never closes the descriptor returned here. Duplicate the
	// VpnService fd so libbox can own its copy, track our copy, and release all
	// of them on StopSingBox. Without that release the kernel keeps tun0 alive
	// after VpnService.stop and the next connect blackholes traffic into the
	// stale interface.
	fd, err := syscall.Dup(int(p.tunFD))
	if err != nil {
		return 0, err
	}
	p.openedMu.Lock()
	p.openedTunFDs = append(p.openedTunFDs, fd)
	p.openedMu.Unlock()
	return int32(fd), nil
}

func (p *androidPlatform) releaseTunFDs() {
	p.openedMu.Lock()
	fds := p.openedTunFDs
	p.openedTunFDs = nil
	p.openedMu.Unlock()
	for _, fd := range fds {
		_ = syscall.Close(fd)
	}
}

func (p *androidPlatform) FindConnectionOwner(int32, string, int32, string, int32) (*libbox.ConnectionOwner, error) {
	return nil, errors.New("connection owner lookup is not available")
}

func (p *androidPlatform) StartDefaultInterfaceMonitor(listener libbox.InterfaceUpdateListener) error {
	p.mu.Lock()
	if p.monitorStop != nil {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	iface, err := p.pickDefaultInterface()
	if err != nil {
		return err
	}

	p.mu.Lock()
	if p.monitorStop != nil {
		p.mu.Unlock()
		return nil
	}
	stop := make(chan struct{})
	p.monitorStop = stop
	p.mu.Unlock()
	listener.UpdateDefaultInterface(iface.Name, int32(iface.Index), false, false)

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		current := iface.Name
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				next, err := p.pickDefaultInterface()
				if err == nil && next.Name != current {
					current = next.Name
					listener.UpdateDefaultInterface(next.Name, int32(next.Index), false, false)
				}
			}
		}
	}()
	return nil
}

func (p *androidPlatform) CloseDefaultInterfaceMonitor(libbox.InterfaceUpdateListener) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.monitorStop != nil {
		close(p.monitorStop)
		p.monitorStop = nil
	}
	return nil
}

func (p *androidPlatform) pickDefaultInterface() (net.Interface, error) {
	p.mu.Lock()
	snapshot := p.snapshot
	myTun := p.myTunName
	p.mu.Unlock()
	var fallback *networkInterfaceSnapshot
	for i := range snapshot.Interfaces {
		iface := &snapshot.Interfaces[i]
		if iface.Name == "" || iface.Name == myTun || iface.Flags&int32(net.FlagUp) == 0 || iface.Flags&int32(net.FlagLoopback) != 0 {
			continue
		}
		if iface.Name == snapshot.Default || (snapshot.DefaultIndex != 0 && iface.Index == snapshot.DefaultIndex) {
			return net.Interface{Index: int(iface.Index), MTU: int(iface.MTU), Name: iface.Name, Flags: net.Flags(iface.Flags)}, nil
		}
		if fallback == nil && len(iface.Addresses) > 0 {
			fallback = iface
		}
	}
	if fallback != nil {
		return net.Interface{Index: int(fallback.Index), MTU: int(fallback.MTU), Name: fallback.Name, Flags: net.Flags(fallback.Flags)}, nil
	}
	return net.Interface{}, errors.New("no default network interface")
}

func (p *androidPlatform) GetInterfaces() (libbox.NetworkInterfaceIterator, error) {
	p.mu.Lock()
	snapshot := p.snapshot
	myTun := p.myTunName
	p.mu.Unlock()
	items := make([]*libbox.NetworkInterface, 0, len(snapshot.Interfaces))
	for _, iface := range snapshot.Interfaces {
		if iface.Name == "" || iface.Flags&int32(net.FlagLoopback) != 0 || iface.Name == myTun {
			continue
		}
		items = append(items, &libbox.NetworkInterface{
			Index:     iface.Index,
			MTU:       iface.MTU,
			Name:      iface.Name,
			Addresses: &stringList{values: iface.Addresses},
			Flags:     iface.Flags,
			Type:      iface.Type,
			DNSServer: &stringList{values: iface.DNSServers},
			Gateway:   &stringList{},
			Metered:   iface.Metered,
		})
	}
	return &networkInterfaceList{values: items}, nil
}

func (p *androidPlatform) UnderNetworkExtension() bool                              { return false }
func (p *androidPlatform) IncludeAllNetworks() bool                                 { return false }
func (p *androidPlatform) ReadWIFIState() *libbox.WIFIState                         { return nil }
func (p *androidPlatform) ClearDNSCache()                                           {}
func (p *androidPlatform) SendNotification(*libbox.Notification) error              { return nil }
func (p *androidPlatform) CancelNotification(string, int32) error                   { return nil }
func (p *androidPlatform) StartNeighborMonitor(libbox.NeighborUpdateListener) error { return nil }
func (p *androidPlatform) CloseNeighborMonitor(libbox.NeighborUpdateListener) error { return nil }

func (p *androidPlatform) RegisterMyInterface(name string) {
	p.mu.Lock()
	p.myTunName = name
	p.mu.Unlock()
}

func (p *androidPlatform) UsePlatformShell() bool { return false }
func (p *androidPlatform) CheckPlatformShell() error {
	return errors.New("platform shell is unavailable")
}
func (p *androidPlatform) OpenShellSession(*libbox.PlatformUser, string, libbox.StringIterator, string, int32, int32) (libbox.ShellSession, error) {
	return nil, errors.New("platform shell is unavailable")
}
func (p *androidPlatform) LookupUser(string) (*libbox.PlatformUser, error) {
	return nil, errors.New("platform user lookup is unavailable")
}
func (p *androidPlatform) LookupSFTPServer() (string, error) {
	return "", errors.New("sftp is unavailable")
}
func (p *androidPlatform) ReadSystemSSHHostKey() (string, error) {
	return "", errors.New("ssh host key is unavailable")
}
func (p *androidPlatform) TailscaleHostname() string { return "" }
func (p *androidPlatform) UsePlatformBridge() bool   { return false }
func (p *androidPlatform) CreateBridge(*libbox.BridgeOptions) (libbox.BridgeSession, error) {
	return nil, errors.New("platform bridge is unavailable")
}

type stringList struct {
	values []string
}

func (s *stringList) Len() int32    { return int32(len(s.values)) }
func (s *stringList) HasNext() bool { return len(s.values) > 0 }
func (s *stringList) Next() string {
	if len(s.values) == 0 {
		return ""
	}
	v := s.values[0]
	s.values = s.values[1:]
	return v
}

type networkInterfaceList struct {
	values []*libbox.NetworkInterface
}

func (n *networkInterfaceList) HasNext() bool { return len(n.values) > 0 }
func (n *networkInterfaceList) Next() *libbox.NetworkInterface {
	if len(n.values) == 0 {
		return nil
	}
	v := n.values[0]
	n.values = n.values[1:]
	return v
}

type noopCommandHandler struct{}

func (noopCommandHandler) ServiceStop() error   { return nil }
func (noopCommandHandler) ServiceReload() error { return nil }
func (noopCommandHandler) GetSystemProxyStatus() (*libbox.SystemProxyStatus, error) {
	return &libbox.SystemProxyStatus{Available: false, Enabled: false}, nil
}
func (noopCommandHandler) SetSystemProxyEnabled(bool) error { return nil }
func (noopCommandHandler) TriggerNativeCrash() error        { return nil }
func (noopCommandHandler) WriteDebugMessage(string)         {}
func (noopCommandHandler) ConnectSSHAgent() (int32, error) {
	return 0, errors.New("ssh agent is unavailable")
}
