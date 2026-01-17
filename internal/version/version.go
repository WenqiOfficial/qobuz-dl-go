// Package version provides build-time version information.
// Values are injected via ldflags during compilation.
package version

import (
	"fmt"
	"runtime"
)

// These variables are set by ldflags at build time.
// Example: go build -ldflags="-X qobuz-dl-go/internal/version.Version=1.0.0"
var (
	// Version is the semantic version (e.g., "1.0.0" or "dev")
	Version = "dev"

	// BuildTime is the build timestamp (e.g., "20260117-1530")
	BuildTime = "unknown"

	// GitCommit is the git commit hash (short)
	GitCommit = "unknown"
)

// Info holds complete version information.
type Info struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
	GitCommit string `json:"gitCommit"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get returns the current version info.
func Get() Info {
	return Info{
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string.
func (i Info) String() string {
	return fmt.Sprintf("qobuz-dl-go %s (%s) built %s with %s",
		i.Version, i.GitCommit, i.BuildTime, i.GoVersion)
}

// Short returns just the version number.
func Short() string {
	return Version
}

// Full returns the complete version string.
func Full() string {
	return Get().String()
}
