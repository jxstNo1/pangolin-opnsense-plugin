package version

// These values are overridden with -ldflags at build time. Keep the defaults
// useful for local `go run` builds.
var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)
