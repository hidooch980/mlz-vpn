// Package routing generates sing-box Split-DNS and direct-bypass rules
// so Iranian domains (.ir, banks, local services) and LAN traffic stay direct.
package routing

// IranDirectSuffixes lists core domestic suffixes bypassed directly before GeoSite rules.
var IranDirectSuffixes = []string{
	".ir",
	".gov.ir",
	".ac.ir",
	".co.ir",
	".sch.ir",
	".org.ir",
	".net.ir",
	".id.ir",
}

// IranPriorityDomains covers critical domestic banking, payment, and daily platforms
// that may use non-.ir TLDs or domestic CDNs.
var IranPriorityDomains = []string{
	"shaparak.ir",
	"shepa.com",
	"zarinpal.com",
	"snapp.ir",
	"tapsi.ir",
	"digikala.com",
	"divar.ir",
	"aparat.com",
	"filimo.com",
	"telewebion.com",
	"bale.ai",
	"eitaa.com",
	"rubika.ir",
}

// DNSConfig matches the sing-box 1.12+ DNS schema. Servers use the typed
// (type/server/server_port) form; the legacy address-string form was removed
// in sing-box 1.14.
type DNSConfig struct {
	Servers  []DNSServer `json:"servers"`
	Rules    []DNSRule   `json:"rules,omitempty"`
	Final    string      `json:"final,omitempty"`
	Strategy string      `json:"strategy,omitempty"`
}

type DNSServer struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Server     string `json:"server,omitempty"`
	ServerPort int    `json:"server_port,omitempty"`
	Path       string `json:"path,omitempty"`
	Detour     string `json:"detour,omitempty"`
	Insecure   bool   `json:"insecure,omitempty"`
}

type DNSRule struct {
	RuleSet      []string `json:"rule_set,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	Domain       []string `json:"domain,omitempty"`
	Server       string   `json:"server"`
}

// RouteRule covers both matchers and sing-box 1.11+ rule actions. Action is
// empty for ordinary route-to-outbound rules.
type RouteRule struct {
	Action       string   `json:"action,omitempty"`
	Protocol     string   `json:"protocol,omitempty"`
	Port         int      `json:"port,omitempty"`
	IPIsPrivate  bool     `json:"ip_is_private,omitempty"`
	RuleSet      []string `json:"rule_set,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	Domain       []string `json:"domain,omitempty"`
	Outbound     string   `json:"outbound,omitempty"`
}

// BuildIranSplitDNS returns Split-DNS rules keeping domestic lookups on clean
// Iranian DNS while routing global lookups through encrypted DoH inside the proxy.
func BuildIranSplitDNS(proxyOutboundTag string) DNSConfig {
	return DNSConfig{
		Servers: []DNSServer{
			{
				Type:   "https",
				Tag:    "dns-remote",
				Server: "1.1.1.1",
				Path:   "/dns-query",
				Detour: proxyOutboundTag,
			},
			{
				Type:   "udp",
				Tag:    "dns-iran-direct",
				Server: "178.22.122.100",
				Detour: "direct",
			},
		},
		Rules: []DNSRule{
			{
				DomainSuffix: IranDirectSuffixes,
				Domain:       IranPriorityDomains,
				Server:       "dns-iran-direct",
			},
			{
				RuleSet: []string{"geosite-ir"},
				Server:  "dns-iran-direct",
			},
		},
		Final:    "dns-remote",
		Strategy: "prefer_ipv4",
	}
}

// BuildIranRouteRules returns ordered sing-box route rules ensuring private LANs
// and Iranian domains/IPs always exit via direct.
func BuildIranRouteRules(proxyOutboundTag string) []RouteRule {
	return []RouteRule{
		{Action: "sniff"},
		{Action: "hijack-dns", Protocol: "dns"},
		{Action: "hijack-dns", Port: 53},
		{
			IPIsPrivate: true,
			Outbound:    "direct",
		},
		{
			DomainSuffix: IranDirectSuffixes,
			Domain:       IranPriorityDomains,
			Outbound:     "direct",
		},
		{
			RuleSet:  []string{"geosite-ir", "geoip-ir"},
			Outbound: "direct",
		},
		{
			Outbound: proxyOutboundTag,
		},
	}
}
