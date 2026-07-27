package cloudflare

import "testing"

func hostnames(rules []IngressRule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.Hostname)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNormalizeIngressAddsCatchAll(t *testing.T) {
	got := NormalizeIngress(nil)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Hostname != "" || got[0].Service != CatchAllService {
		t.Fatalf("catch-all = %+v", got[0])
	}
}

func TestNormalizeIngressKeepsOneCatchAllLast(t *testing.T) {
	in := []IngressRule{
		{Service: "http_status:404"},
		{Hostname: "a.example.com", Service: "http://localhost:1"},
		{Service: "http_status:500"},
		{Hostname: "b.example.com", Service: "http://localhost:2"},
	}
	got := NormalizeIngress(in)

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(got), got)
	}
	if !equalStrings(hostnames(got), []string{"a.example.com", "b.example.com", ""}) {
		t.Fatalf("order = %v", hostnames(got))
	}
	// The first catch-all found wins, and it must be last.
	if got[2].Service != "http_status:404" {
		t.Fatalf("catch-all service = %q", got[2].Service)
	}
}

func TestNormalizeIngressFixesEmptyCatchAllService(t *testing.T) {
	got := NormalizeIngress([]IngressRule{{Service: "  "}})
	if got[len(got)-1].Service != CatchAllService {
		t.Fatalf("service = %q, want %q", got[len(got)-1].Service, CatchAllService)
	}
}

func TestNormalizeIngressKeepsPathOnlyRules(t *testing.T) {
	in := []IngressRule{{Path: "/health", Service: "http://localhost:9"}}
	got := NormalizeIngress(in)
	if len(got) != 2 || got[0].Path != "/health" {
		t.Fatalf("path rule dropped: %+v", got)
	}
}

func TestUpsertIngressInsertsBeforeCatchAll(t *testing.T) {
	got := UpsertIngress(nil, "crm.demo.example.com", "http://localhost:3000")
	if !equalStrings(hostnames(got), []string{"crm.demo.example.com", ""}) {
		t.Fatalf("order = %v", hostnames(got))
	}
	if got[len(got)-1].Service != CatchAllService {
		t.Fatal("catch-all is not last")
	}
}

func TestUpsertIngressReplacesExisting(t *testing.T) {
	rules := UpsertIngress(nil, "crm.demo.example.com", "http://localhost:3000")
	rules = UpsertIngress(rules, "api.demo.example.com", "http://localhost:8080")
	rules = UpsertIngress(rules, "CRM.demo.example.com", "http://localhost:5000")

	if len(rules) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(rules), rules)
	}
	rule, ok := FindIngress(rules, "crm.demo.example.com")
	if !ok {
		t.Fatal("FindIngress did not find the rule")
	}
	if rule.Service != "http://localhost:5000" {
		t.Fatalf("service = %q, want the replacement", rule.Service)
	}
	if rules[len(rules)-1].Hostname != "" {
		t.Fatal("catch-all is not last after upserts")
	}
}

func TestRemoveIngress(t *testing.T) {
	rules := UpsertIngress(nil, "a.example.com", "http://localhost:1")
	rules = UpsertIngress(rules, "b.example.com", "http://localhost:2")

	rules = RemoveIngress(rules, "A.example.com")
	if _, ok := FindIngress(rules, "a.example.com"); ok {
		t.Fatal("rule was not removed (case-insensitive match failed)")
	}
	if !equalStrings(hostnames(rules), []string{"b.example.com", ""}) {
		t.Fatalf("order = %v", hostnames(rules))
	}

	// Removing everything still leaves a valid config.
	rules = RemoveIngress(rules, "b.example.com")
	if len(rules) != 1 || rules[0].Service != CatchAllService {
		t.Fatalf("config invalid after removing everything: %+v", rules)
	}

	// Removing something that is not there is a no-op.
	same := RemoveIngress(rules, "nope.example.com")
	if len(same) != 1 {
		t.Fatalf("len = %d, want 1", len(same))
	}
}

func TestFindIngressIgnoresCatchAll(t *testing.T) {
	rules := NormalizeIngress(nil)
	if _, ok := FindIngress(rules, ""); ok {
		t.Fatal("FindIngress matched the catch-all rule")
	}
}

func TestHostnameRules(t *testing.T) {
	rules := UpsertIngress(nil, "a.example.com", "http://localhost:1")
	got := HostnameRules(rules)
	if len(got) != 1 || got[0].Hostname != "a.example.com" {
		t.Fatalf("HostnameRules = %+v", got)
	}
}

func TestTunnelHelpers(t *testing.T) {
	tun := &Tunnel{Connections: []Connection{
		{ColoName: "AMS"},
		{ColoName: "AMS"},
		{ColoName: "FRA"},
		{ColoName: "CDG", IsPendingReconnect: true},
	}}
	if got := tun.ActiveConnections(); got != 3 {
		t.Fatalf("ActiveConnections() = %d, want 3", got)
	}
	if got := tun.Colos(); !equalStrings(got, []string{"AMS", "FRA", "CDG"}) {
		t.Fatalf("Colos() = %v", got)
	}
}

func TestTunnelCNAMETarget(t *testing.T) {
	if got := TunnelCNAMETarget("abc"); got != "abc.cfargotunnel.com" {
		t.Fatalf("TunnelCNAMETarget = %q", got)
	}
	if !IsTunnelTarget("abc.cfargotunnel.com") {
		t.Fatal("IsTunnelTarget = false for a tunnel target")
	}
	if !IsTunnelTarget("ABC.CFARGOTUNNEL.COM.") {
		t.Fatal("IsTunnelTarget is not case/dot tolerant")
	}
	if IsTunnelTarget("example.com") {
		t.Fatal("IsTunnelTarget = true for a normal CNAME")
	}
}
