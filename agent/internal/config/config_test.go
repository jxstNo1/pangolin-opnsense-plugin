package config

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func validClient() *Config {
	c := Default()
	c.Mode = ModeClient
	c.Client.Endpoint = "https://pangolin.example.com"
	c.Client.ID = "client-id"
	c.Client.Secret = "client-secret"
	c.SubnetRouter.Enabled = true
	c.SubnetRouter.LocalNetworks = []string{"10.10.0.0/16"}
	c.DNS.Enabled = true
	c.DNS.OutputFile = path.Join(c.StateDir, DefaultDNSOutput)
	return c
}

func TestValidateClientOK(t *testing.T) {
	if err := validClient().Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestRejectsDefaultRoute(t *testing.T) {
	c := validClient()
	c.SubnetRouter.LocalNetworks = []string{"0.0.0.0/0"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected default route to be rejected")
	}
}

func TestRejectsBroadPrefix(t *testing.T) {
	c := validClient()
	c.SubnetRouter.LocalNetworks = []string{"10.0.0.0/7"}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "too broad") {
		t.Fatalf("expected broad prefix rejection, got %v", err)
	}
}

func TestRejectsDNSOverride(t *testing.T) {
	c := validClient()
	c.Client.OverrideDNS = true
	if err := c.Validate(); err == nil {
		t.Fatal("expected overrideDns=true to be rejected on OPNsense")
	}
}

func TestRejectsHTTP(t *testing.T) {
	c := validClient()
	c.Client.Endpoint = "http://pangolin.example.com"
	if err := c.Validate(); err == nil {
		t.Fatal("expected plain http to be rejected")
	}
	c.AllowInsecureHTTP = true
	if err := c.Validate(); err != nil {
		t.Fatalf("expected http to be allowed with explicit override: %v", err)
	}
}

func TestRejectsInvalidInterface(t *testing.T) {
	c := validClient()
	c.Client.InterfaceName = "this-name-is-too-long"
	if err := c.Validate(); err == nil {
		t.Fatal("expected invalid interface name to be rejected")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.json")
	doc := map[string]any{
		"mode":     ModeClient,
		"stateDir": dir,
		"client": map[string]any{
			"endpoint":      "https://pangolin.example.com",
			"id":            "id",
			"secret":        "secret",
			"interfaceName": "pangolin0",
			"unexpected":    true,
		},
	}
	data, _ := json.Marshal(doc)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.json")
	cfg := validClient()
	cfg.StateDir = dir
	cfg.DNS.OutputFile = filepath.Join(dir, DefaultDNSOutput)
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Client.InterfaceName != DefaultClientIf {
		t.Fatalf("expected default interface, got %q", got.Client.InterfaceName)
	}
	if got.StatePath() != filepath.Join(dir, "client-status.json") {
		t.Fatalf("unexpected status path %s", got.StatePath())
	}
}

func TestForwardDomainValidation(t *testing.T) {
	c := validClient()
	c.DNS.ForwardDomains = []string{"internal.example.com"}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid forward domain: %v", err)
	}
	c.DNS.ForwardDomains = []string{"bad domain"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected invalid forward domain to be rejected")
	}
}

func TestRemoteNetworkAllowList(t *testing.T) {
	c := validClient()
	c.SubnetRouter.AllowedRemoteNetworks = []string{"192.168.50.0/24"}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid remote allow-list: %v", err)
	}
	c.SubnetRouter.AllowedRemoteNetworks = []string{"0.0.0.0/0"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected default remote route to be rejected")
	}
	c.SubnetRouter.AllowedRemoteNetworks = []string{"10.10.128.0/17"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected remote/local overlap to be rejected")
	}
}
