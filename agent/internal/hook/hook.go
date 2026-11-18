// Package hook invokes the plugin-provided state-change hook without ever
// passing secrets or shell-interpreting arguments.
package hook

import (
	"context"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

const debounce = 750 * time.Millisecond

type Hook struct {
	path       string
	statePath  string
	mode       string
	events     chan string
	done       chan struct{}
	closeOnce  sync.Once
}

func New(path, statePath, mode string) *Hook {
	h := &Hook{
		path:      path,
		statePath: statePath,
		mode:      mode,
		events:    make(chan string, 32),
		done:      make(chan struct{}),
	}
	if path != "" {
		go h.loop()
	} else {
		close(h.done)
	}
	return h
}

// Notify queues an event. It never blocks: status updates must not stall the
// tunnel because a web UI action is slow.
func (h *Hook) Notify(event string) {
	if h.path == "" {
		return
	}
	select {
	case h.events <- event:
	default:
		// Drop the oldest event to keep the newest one; the hook always
		// reads the current state file, so coalescing is safe.
		select {
		case <-h.events:
		default:
		}
		select {
		case h.events <- event:
		default:
		}
	}
}

func (h *Hook) Close() {
	if h.path == "" {
		return
	}
	h.closeOnce.Do(func() {
		close(h.events)
		<-h.done
	})
}

func (h *Hook) loop() {
	defer close(h.done)
	for {
		event, ok := <-h.events
		if !ok {
			return
		}
		// Coalesce a burst of changes (connect + routes + DNS) into one
		// plugin sync.
		timer := time.NewTimer(debounce)
	wait:
		for {
			select {
			case next, ok := <-h.events:
				if !ok {
					timer.Stop()
					h.run(event)
					return
				}
				event = next
			case <-timer.C:
				break wait
			}
		}
		h.run(event)
	}
}

func (h *Hook) run(event string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, h.path, event)
	cmd.Env = append(os.Environ(),
		"PANGOLIN_EVENT="+event,
		"PANGOLIN_MODE="+h.mode,
		"PANGOLIN_STATE_FILE="+h.statePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("hook %s %s failed: %v: %s", h.path, event, err, string(out))
		return
	}
	if len(out) > 0 {
		log.Printf("hook %s: %s", event, string(out))
	}
}
