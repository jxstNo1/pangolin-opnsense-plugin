// Package ha implements the CARP active/passive gate used by the agent.
//
// It deliberately fails closed: if CARP state cannot be read, the tunnel is
// not started. A standby node therefore never races the master for the same
// Pangolin identity or installs duplicate NAT/routing state.
package ha

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

type State string

const (
	Master State = "MASTER"
	Backup State = "BACKUP"
	Init   State = "INIT"
	Unknown State = "UNKNOWN"
)

// ReadState parses `ifconfig <iface>` for a CARP line. FreeBSD prints:
//
//	carp: MASTER vhid 1 advbase 1 advskew 0
func ReadState(iface string) (State, error) {
	if iface == "" {
		return Unknown, fmt.Errorf("CARP interface is empty")
	}
	out, err := exec.Command("/sbin/ifconfig", iface).CombinedOutput()
	if err != nil {
		return Unknown, fmt.Errorf("ifconfig %s: %w: %s", iface, err, strings.TrimSpace(string(out)))
	}
	return parseState(string(out), iface)
}

func parseState(output, iface string) (State, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "carp:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			switch strings.ToUpper(fields[1]) {
			case "MASTER":
				return Master, nil
			case "BACKUP":
				return Backup, nil
			case "INIT":
				return Init, nil
			}
		}
	}
	return Unknown, fmt.Errorf("interface %s has no CARP state", iface)
}

// Wait blocks until the CARP interface reaches want. onState is called for
// every observed state (including the initial one) so status/hooks reflect
// why the tunnel is not running.
func Wait(ctx context.Context, iface, want string, onState func(State, string)) error {
	want = strings.ToUpper(strings.TrimSpace(want))
	if want != string(Master) && want != string(Backup) {
		return fmt.Errorf("invalid required CARP state %q", want)
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var last State
	for {
		state, err := ReadState(iface)
		note := ""
		if err != nil {
			state = Unknown
			note = err.Error()
		}
		if state != last || note != "" {
			log.Printf("ha: CARP %s is %s (want %s)", iface, state, want)
			if onState != nil {
				onState(state, note)
			}
			last = state
		}
		if state == State(want) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Monitor cancels the tunnel when the node stops being active. The caller
// passes the cancel function of the run context; returning from the function
// is not enough because newt/olm run in goroutines.
func Monitor(ctx context.Context, iface, want string, cancel context.CancelFunc, onState func(State, string)) {
	want = strings.ToUpper(strings.TrimSpace(want))
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		state, err := ReadState(iface)
		if err != nil {
			failures++
			if failures >= 3 {
				log.Printf("ha: cannot read CARP state for %s: %v; stopping tunnel (fail closed)", iface, err)
				if onState != nil {
					onState(Unknown, err.Error())
				}
				cancel()
				return
			}
			continue
		}
		failures = 0
		if onState != nil {
			onState(state, "")
		}
		if state != State(want) {
			log.Printf("ha: CARP %s is %s, want %s; stopping tunnel", iface, state, want)
			cancel()
			return
		}
	}
}
