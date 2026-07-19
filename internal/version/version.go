// Package version holds build-time metadata injected via -ldflags -X.
// Values are set at compile time (see DEPLOYMENT.md §2); the zero values
// below are what you get from a plain `go run`/`go build` with no flags.
package version

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
