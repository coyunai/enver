package buildinfo

// These values are overridden with -ldflags at release time.
var (
	Version = "0.0.1"
	Commit  = "none"
	Date    = "unknown"
)
