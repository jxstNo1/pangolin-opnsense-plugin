package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestDNSRecordsCustomPath(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, "client")
	if err != nil {
		t.Fatal(err)
	}
	s.Init("test")
	custom := filepath.Join(dir, "custom-dns.json")
	s.SetDNSRecordsPath(custom)
	if err := s.WriteDNSRecords([]DNSEntry{{Domain: "a.example.com", IP: "100.64.0.2", Type: "A"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatalf("custom DNS snapshot missing: %v", err)
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

func TestTouchKeepsStatusFresh(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, "client")
	s.Init("test")
	first := s.lastWrite
	if err := s.Touch(time.Hour); err != nil {
		t.Fatal(err)
	}
	if !s.lastWrite.Equal(first) {
		t.Fatal("Touch must not rewrite inside the interval")
	}
	if err := s.Touch(0); err != nil {
		t.Fatal(err)
	}
	if !s.lastWrite.After(first) {
		t.Fatal("Touch(0) must refresh the status file")
	}
}

func TestSubnetRouterSnapshotIsCopy(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, "client")
	s.Init("test")
	sr := &SubnetRouterStatus{
		Enabled:       true,
		Backend:       "pf",
		LocalNetworks: []string{"10.10.0.0/16"},
		Applied:       true,
	}
	if _, _, err := s.SetSubnetRouter(sr); err != nil {
		t.Fatal(err)
	}
	sr.Applied = false
	sr.LocalNetworks[0] = "0.0.0.0/0"
	snapshot := s.Snapshot()
	if snapshot.SubnetRouter == nil || !snapshot.SubnetRouter.Applied {
		t.Fatalf("snapshot changed through caller pointer: %+v", snapshot.SubnetRouter)
	}
	if snapshot.SubnetRouter.LocalNetworks[0] != "10.10.0.0/16" {
		t.Fatalf("snapshot shares LocalNetworks slice: %+v", snapshot.SubnetRouter.LocalNetworks)
	}
}

func TestWriteDNSRecordsEmptyReplacesStale(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, "client")
	s.Init("test")
	if err := s.WriteDNSRecords([]DNSEntry{{Domain: "stale.example.com", IP: "100.64.0.2", Type: "A"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDNSRecords(nil); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadDNSRecords()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("stale records survived empty snapshot: %+v", got)
	}
	raw, err := os.ReadFile(s.DNSRecordsPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "null\n" {
		t.Fatalf("empty snapshot must be a JSON array, got %q", raw)
	}
}

func TestFailedStatusWriteIsRetried(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, "client")
	s.Init("test")
	statusPath := s.StatusPath()
	if err := os.Remove(statusPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(statusPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetConnected(true, "100.64.0.2", "pangolin0", []string{"10.20.0.0/16"}); err == nil {
		t.Fatal("expected status write to fail while path is a directory")
	}
	if err := os.Remove(statusPath); err != nil {
		t.Fatal(err)
	}
	if err := s.Touch(0); err != nil {
		t.Fatalf("dirty status was not retried: %v", err)
	}
	st, err := ReadStatus(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Connected || st.TunnelIP != "100.64.0.2" {
		t.Fatalf("retried status is incomplete: %+v", st)
	}
}
