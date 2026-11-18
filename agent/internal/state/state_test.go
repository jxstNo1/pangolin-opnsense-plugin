package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, "client")
	if err != nil {
		t.Fatal(err)
	}
	s.Init("test")
	if _, changed, err := s.SetConnected(true, "100.64.0.2", "pangolin0", []string{"10.20.0.0/16"}); err != nil || !changed {
		t.Fatalf("SetConnected: changed=%v err=%v", changed, err)
	}
	st, err := ReadStatus(s.StatusPath())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Connected || st.TunnelIP != "100.64.0.2" {
		t.Fatalf("unexpected status: %+v", st)
	}
}

func TestDNSRecords(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, "client")
	if err != nil {
		t.Fatal(err)
	}
	s.Init("test")
	records := []DNSEntry{
		{Domain: "b.example.com", IP: "100.64.0.7", Type: "A", SiteID: 1},
		{Domain: "a.example.com", IP: "100.64.0.8", Type: "A", SiteID: 2},
	}
	if err := s.WriteDNSRecords(records); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadDNSRecords()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != records[0] || got[1] != records[1] {
		t.Fatalf("unexpected records: %+v", got)
	}
	raw, err := os.ReadFile(s.DNSRecordsPath())
	if err != nil {
		t.Fatal(err)
	}
	var decoded []DNSEntry
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 {
		t.Fatalf("unexpected file content: %s", raw)
	}
}

func TestNoTempFilesLeft(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, "site")
	s.Init("test")
	_, _, _ = s.SetConnected(true, "100.64.0.3", "newt0", nil)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestHealthyRequiresFreshFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, "site")
	if s.Healthy(0) {
		t.Fatal("missing health file must not be healthy")
	}
	if err := os.WriteFile(s.HealthPath(), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.Healthy(0) {
		t.Fatal("fresh health file should be healthy")
	}
}
