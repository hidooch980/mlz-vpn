package routing

import "testing"

func TestBuildIranRules(t *testing.T) {
	dns := BuildIranSplitDNS("proxy-out")
	if dns.Final != "dns-remote" || len(dns.Servers) != 2 || len(dns.Rules) < 3 {
		t.Fatalf("unexpected split-dns config: %+v", dns)
	}

	routes := BuildIranRouteRules("proxy-out")
	if len(routes) < 5 {
		t.Fatalf("expected at least 5 route rules, got %d", len(routes))
	}
	if !routes[1].IPIsPrivate || routes[1].Outbound != "direct" {
		t.Fatalf("expected private IP bypass near top of rules: %+v", routes[1])
	}
	if routes[len(routes)-1].Outbound != "proxy-out" {
		t.Fatalf("expected final fallback rule to route through proxy-out")
	}
}
