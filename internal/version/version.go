package version

import (
	"fmt"
	"os/exec"
	"strings"
)

// Set at link time via -ldflags; "dev" triggers a git describe fallback.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

func init() {
	if Version != "dev" {
		return
	}
	out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
	if err != nil {
		return
	}
	Version = strings.TrimSpace(string(out))
	if hash, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		Commit = strings.TrimSpace(string(hash))
	}
}

// Short returns the version string without build metadata.
func Short() string {
	return Version
}

// Full returns version with optional commit and build date.
func Full() string {
	if Commit == "" && Date == "" {
		return Version
	}
	var b strings.Builder
	b.WriteString(Version)
	if Commit != "" || Date != "" {
		b.WriteString(" (")
		if Commit != "" {
			b.WriteString(Commit)
		}
		if Date != "" {
			if Commit != "" {
				b.WriteString(", ")
			}
			b.WriteString(Date)
		}
		b.WriteString(")")
	}
	return b.String()
}

// Line returns a single-line label suitable for CLI or UI headers.
func Line() string {
	return fmt.Sprintf("appfossil %s", Full())
}
