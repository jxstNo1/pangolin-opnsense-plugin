package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-pangolin/os-pangolin/agent/internal/config"
	"github.com/os-pangolin/os-pangolin/agent/internal/ha"
	"github.com/os-pangolin/os-pangolin/agent/internal/hook"
	"github.com/os-pangolin/os-pangolin/agent/internal/state"
	"github.com/os-pangolin/os-pangolin/agent/internal/version"
)

// Run is the common entry point for both service modes. It owns the status
// store, the state-change hook and the CARP active/passive gate, then hands
// off to the site or client runtime.
func Run(ctx context.Context, cfg *config.Config) error {
	store, err := state.New(cfg.StateDir, cfg.Mode)
	if err != nil {
		return err
	}
	store.Init(version.Version)

	h := hook.New(cfg.HookScript, store.StatusPath(), cfg.Mode)
	defer h.Close()
	notify := func(event string) { h.Notify(event) }

	log.Printf("starting os-pangolin agent %s (%s, commit %s)", version.Version, cfg.Mode, version.Commit)

	if cfg.HA != nil && cfg.HA.Enabled {
		log.Printf("ha: waiting for CARP %s on %s", cfg.HA.RequiredState, cfg.HA.Interface)
		err := ha.Wait(ctx, cfg.HA.Interface, cfg.HA.RequiredState, func(s ha.State, note string) {
			active := string(s) == cfg.HA.RequiredState
			_, _, _ = store.SetHA(active, fmt.Sprintf("CARP %s: %s", cfg.HA.Interface, s))
			if !active {
				notify("down")
			}
		})
		if err != nil {
			return fmt.Errorf("wait for CARP active state: %w", err)
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if cfg.HA != nil && cfg.HA.Enabled {
		go ha.Monitor(runCtx, cfg.HA.Interface, cfg.HA.RequiredState, cancel, func(s ha.State, note string) {
			active := string(s) == cfg.HA.RequiredState
			_, _, _ = store.SetHA(active, fmt.Sprintf("CARP %s: %s", cfg.HA.Interface, s))
			if !active {
				notify("down")
			}
		})
	} else {
		_, _, _ = store.SetHA(true, "CARP integration disabled")
	}

	switch cfg.Mode {
	case config.ModeSite:
		err = RunSite(runCtx, cfg, store, notify)
	case config.ModeClient:
		err = RunClient(runCtx, cfg, store, notify)
	default:
		err = fmt.Errorf("unknown mode %q", cfg.Mode)
	}

	if ctx.Err() != nil {
		// Normal SIGTERM shutdown: not an error.
		return nil
	}
	if err != nil {
		_, _, _ = store.SetError(err.Error())
		return err
	}
	return nil
}

// TestEndpoint performs a plain HTTPS reachability check. It intentionally
// sends no Pangolin credentials: the UI's "test" button uses it to verify DNS,
// routing and TLS termination before the operator saves a secret.
func TestEndpoint(ctx context.Context, endpoint string) error {
	req, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return probeEndpoint(req, endpoint)
}
