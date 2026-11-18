package ha

import "testing"

func TestParseState(t *testing.T) {
	cases := []struct {
		out  string
		want State
	}{
		{"em0: flags=8843<UP>\n\tcarp: MASTER vhid 1 advbase 1 advskew 0\n", Master},
		{"em0: flags=8843<UP>\n\tcarp: BACKUP vhid 1 advbase 1 advskew 100\n", Backup},
		{"em0: flags=8843<UP>\n\tcarp: INIT vhid 1 advbase 1 advskew 100\n", Init},
	}
	for _, tc := range cases {
		got, err := parseState(tc.out, "em0")
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}

func TestParseStateUnknown(t *testing.T) {
	if _, err := parseState("em0: flags=8843<UP>", "em0"); err == nil {
		t.Fatal("expected error for interface without CARP state")
	}
}
