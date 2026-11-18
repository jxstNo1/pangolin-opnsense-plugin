package agent

import (
	"testing"
	"time"

	"github.com/fosrl/newt/network"
	olmdns "github.com/fosrl/olm/dns"
)

func TestIncludedRoutes(t *testing.T) {
	routes := includedRoutes([]network.IPv4Route{
		{DestinationAddress: "10.20.0.0", SubnetMask: "255.255.0.0"},
		{DestinationAddress: "0.0.0.0", SubnetMask: "0.0.0.0", IsDefault: true},
		{DestinationAddress: "192.168.1.0", SubnetMask: "255.255.255.0"},
	})
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %v", routes)
	}
	if routes[0] != "10.20.0.0/16" || routes[1] != "192.168.1.0/24" {
		t.Fatalf("unexpected routes: %v", routes)
	}
}

func TestSnapshotDNSSorted(t *testing.T) {
	store := olmdns.NewDNSRecordStore()
	if err := store.AddRecord("b.example.com", []byte{100, 64, 0, 8}, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.AddRecord("a.example.com", []byte{100, 64, 0, 7}, 1); err != nil {
		t.Fatal(err)
	}
	recs := snapshotDNS(store.Snapshot())
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %+v", recs)
	}
	if recs[0].Domain != "a.example.com" || recs[1].Domain != "b.example.com" {
		t.Fatalf("records not sorted: %+v", recs)
	}
}

func TestMustDuration(t *testing.T) {
	if got := mustDuration("", 15*time.Second); got != 15*time.Second {
		t.Fatalf("unexpected fallback: %v", got)
	}
	if got := mustDuration("30s", 15*time.Second); got != 30*time.Second {
		t.Fatalf("unexpected parsed duration: %v", got)
	}
}

func TestSameRoutesIgnoresOrder(t *testing.T) {
	if !sameRoutes([]string{"10.20.0.0/16", "192.168.50.0/24"}, []string{"192.168.50.0/24", "10.20.0.0/16"}) {
		t.Fatal("route order must not count as a change")
	}
	if sameRoutes([]string{"10.20.0.0/16"}, []string{"10.20.0.0/16", "192.168.50.0/24"}) {
		t.Fatal("added route not detected")
	}
	if sameRoutes([]string{"10.20.0.0/16", "192.168.50.0/24"}, []string{"10.20.0.0/16"}) {
		t.Fatal("removed route not detected")
	}
}
