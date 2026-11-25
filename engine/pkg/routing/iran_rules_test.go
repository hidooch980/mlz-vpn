package routing

import "testing"

func TestBuildIranRules(t *testing.T) {
	dns := BuildIranSplitDNS("proxy-out")
	if dns.Final != "dns-remote" || len(dns.Servers) != 2 || len(dns.Rules) < 2 {
		t.Fatalf("unexpected split-dns config: %+v", dns)
	}

	routes := BuildIranRouteRules("proxy-out")
	if len(routes) < 5 {
		t.Fatalf("expected at least 5 route rules, got %d", len(routes))
	}
	privateDirect := false
	for _, rule := range routes {
		if rule.IPIsPrivate && rule.Outbound == "direct" {
			privateDirect = true
			break
		}
	}
	if !privateDirect {
		t.Fatalf("expected a private-IP direct bypass rule: %+v", routes)
	}
	if routes[len(routes)-1].Outbound != "proxy-out" {
		t.Fatalf("expected final fallback rule to route through proxy-out")
	}
}
