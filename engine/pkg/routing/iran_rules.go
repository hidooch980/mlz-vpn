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

// DNSRuleSet builds the Split-DNS configuration section for sing-box.
type DNSConfig struct {
	Servers []DNSServer `json:"servers"`
	Rules   []DNSRule   `json:"rules"`
	Final   string      `json:"final"`
}

type DNSServer struct {
	Tag             string `json:"tag"`
	Address         string `json:"address"`
	Detour          string `json:"detour,omitempty"`
	Strategy        string `json:"strategy,omitempty"`
	AddressResolver string `json:"address_resolver,omitempty"`
}

type DNSRule struct {
	RuleSet      []string `json:"rule_set,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	Domain       []string `json:"domain,omitempty"`
	Outbound     []string `json:"outbound,omitempty"`
	Server       string   `json:"server"`
}

type RouteRule struct {
	Protocol     string   `json:"protocol,omitempty"`
	IPIsPrivate  bool     `json:"ip_is_private,omitempty"`
	RuleSet      []string `json:"rule_set,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	Domain       []string `json:"domain,omitempty"`
	Outbound     string   `json:"outbound"`
}

// BuildIranSplitDNS returns sing-box Split-DNS rules keeping domestic lookups on clean Iranian DNS
// while routing global lookups through encrypted DoH inside the proxy tunnel.
func BuildIranSplitDNS(proxyOutboundTag string) DNSConfig {
	return DNSConfig{
		Servers: []DNSServer{
			{
				Tag:     "dns-remote",
				Address: "https://1.1.1.1/dns-query",
				Detour:  proxyOutboundTag,
			},
			{
				Tag:     "dns-iran-direct",
				Address: "178.22.122.100",
				Detour:  "direct",
			},
		},
		Rules: []DNSRule{
			{
				Outbound: []string{"any"},
				Server:   "dns-iran-direct",
			},
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
		Final: "dns-remote",
	}
}

// BuildIranRouteRules returns ordered sing-box route rules ensuring private LANs
// and Iranian domains/IPs always exit via "direct".
func BuildIranRouteRules(proxyOutboundTag string) []RouteRule {
	return []RouteRule{
		{
			Protocol: "dns",
			Outbound: "dns-out",
		},
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
