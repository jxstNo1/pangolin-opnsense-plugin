// Package logging configures the standard logger for the agent. The agent's
// own messages and the newt/olm logger both go to stderr; rc.d redirects
// stderr to syslog, and on FreeBSD we additionally write to syslog directly so
// the messages survive a closed stdout.
package logging

import (
	"log"
	"strings"
)

func Init(level string) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)
	log.SetOutput(platformWriter())
	log.SetPrefix("[os-pangolin] ")
	// The standard log package has no levels. Keep the configured level
	// available for callers that need to decide whether to emit debug lines.
	current = strings.ToLower(level)
}

var current = "info"

func DebugEnabled() bool {
	return current == "debug"
}
