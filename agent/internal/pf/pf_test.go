package pf

import (
	"net"
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
		LocalInterfaces: []string{"em0"},
		RemoteNetworks:  []string{"192.168.50.0/24"},
	})
	if !strings.Contains(rules, "pass in quick on em0 inet from {10.10.0.0/16}") {
		t.Fatalf("missing source- and interface-scoped pass rule:\n%s", rules)
	}
	if strings.Contains(rules, "from any") {
		t.Fatalf("filter rule must not allow any source:\n%s", rules)
	}
	if !strings.Contains(rules, "pass out quick on pangolin0") {
		t.Fatalf("missing tunnel egress rule:\n%s", rules)
	}
}

func TestInterfacesForNetworks(t *testing.T) {
	_, lan, _ := net.ParseCIDR("192.168.1.15/24")
	_, vlan, _ := net.ParseCIDR("10.10.5.10/16")
	entries := []ifaceNetwork{
		{name: "vtnet1", network: lan},
		{name: "vlan10", network: vlan},
	}
	got := interfacesForNetworks([]string{"192.168.1.0/24"}, entries)
	if len(got) != 1 || got[0] != "vtnet1" {
		t.Fatalf("unexpected interface mapping: %v", got)
	}
	got = interfacesForNetworks([]string{"10.10.0.0/16", "192.168.1.0/24"}, entries)
	if len(got) != 2 || got[0] != "vlan10" || got[1] != "vtnet1" {
		t.Fatalf("unexpected multi-interface mapping: %v", got)
	}
}

func TestPruneRemoteRoutesRejectsOtherLocalNetworks(t *testing.T) {
	remote := []string{
		"10.20.0.0/16",
		"192.168.1.0/24",
		"192.168.1.128/25",
		"10.0.0.0/8",
	}
	blocked := []string{"192.168.1.0/24", "10.10.0.0/16"}
	got := pruneRemoteRoutes(remote, blocked)
	if len(got) != 1 || got[0] != "10.20.0.0/16" {
		t.Fatalf("unexpected pruned routes: %v", got)
	}
}

func TestValidateCIDRsRejectsDangerousValues(t *testing.T) {
	for _, cidr := range []string{
		"0.0.0.0/0",
		"0.0.0.0/1",
		"128.0.0.0/1",
		"10.0.0.0/7",
		"10.0.0.0/8\nnat on tun0",
		"::/0",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"0.0.0.0/8",
		"192.0.2.0/24",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"255.255.255.255/32",
	} {
		if err := validateCIDRs([]string{cidr}); err == nil {
			t.Fatalf("expected %q to be rejected", cidr)
		}
	}
	for _, cidr := range []string{"10.0.0.0/24", "100.64.0.0/10", "172.16.0.0/12", "192.168.1.0/24", "8.8.8.0/24"} {
		if err := validateCIDRs([]string{cidr}); err != nil {
			t.Fatalf("valid CIDR %q rejected: %v", cidr, err)
		}
	}
}

func TestEmptyRemoteRoutesNeverBecomeAny(t *testing.T) {
	cfg := Config{
		TunnelInterface: "pangolin0",
		TunnelIP:        "100.64.0.2",
		LocalNetworks:   []string{"10.10.0.0/16"},
	}
	for name, rules := range map[string]string{
		"nat":    natRules(cfg),
		"filter": filterRules(cfg),
	} {
		if strings.Contains(rules, "any") {
			t.Fatalf("%s rule with empty destinations must not contain any:\n%s", name, rules)
		}
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
