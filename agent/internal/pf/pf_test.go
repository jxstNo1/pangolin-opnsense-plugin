package pf

import (
	"strings"
	"testing"
)

func TestNATRulesUseTunnelAddress(t *testing.T) {
	rules := natRules(Config{
		TunnelInterface: "pangolin0",
		TunnelIP:        "100.64.0.2",
		LocalNetworks:   []string{"10.10.0.0/16", "10.20.0.0/16"},
		RemoteNetworks:  []string{"192.168.50.0/24"},
	})
	for _, want := range []string{
		"nat on pangolin0",
		"from {10.10.0.0/16, 10.20.0.0/16}",
		"to {192.168.50.0/24}",
		"-> 100.64.0.2",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("missing %q in rules:\n%s", want, rules)
		}
	}
	if strings.Contains(rules, "0.0.0.0/0") {
		t.Fatalf("default route must never appear:\n%s", rules)
	}
}

func TestFilterRulesDoNotOpenOtherVLANs(t *testing.T) {
	rules := filterRules(Config{
		TunnelInterface: "pangolin0",
		LocalNetworks:   []string{"10.10.0.0/16"},
		RemoteNetworks:  []string{"192.168.50.0/24"},
	})
	if !strings.Contains(rules, "pass in quick inet from {10.10.0.0/16}") {
		t.Fatalf("missing source-scoped pass rule:\n%s", rules)
	}
	if strings.Contains(rules, "from any") {
		t.Fatalf("filter rule must not allow any source:\n%s", rules)
	}
	if !strings.Contains(rules, "pass out quick on pangolin0") {
		t.Fatalf("missing tunnel egress rule:\n%s", rules)
	}
}

func TestValidateCIDRsRejectsDangerousValues(t *testing.T) {
	for _, cidr := range []string{"0.0.0.0/0", "10.0.0.0/8\nnat on tun0", "::/0"} {
		if err := validateCIDRs([]string{cidr}); err == nil {
			t.Fatalf("expected %q to be rejected", cidr)
		}
	}
	if err := validateCIDRs([]string{"10.0.0.0/24"}); err != nil {
		t.Fatalf("valid CIDR rejected: %v", err)
	}
}

func TestFilterRemoteNetworks(t *testing.T) {
	remote := []string{"10.20.0.0/16", "10.20.5.0/24", "192.168.50.0/24"}
	allowed := []string{"10.20.0.0/16"}
	got, err := filterRemoteNetworks(remote, allowed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "10.20.0.0/16" || got[1] != "10.20.5.0/24" {
		t.Fatalf("unexpected filtered routes: %v", got)
	}
	all, err := filterRemoteNetworks(remote, nil)
	if err != nil || len(all) != 3 {
		t.Fatalf("empty allow-list should keep all routes: %v %v", all, err)
	}
}
