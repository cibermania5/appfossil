package model

import (
	"fmt"
	"time"
)

// UsageSource describes where an app's last-used date came from, in rough order
// of trustworthiness.
type UsageSource int

const (
	// UsageNone means no last-used date could be determined.
	UsageNone UsageSource = iota
	// UsageKnowledge is from CoreDuet's knowledgeC.db (precise; needs sudo/FDA).
	UsageKnowledge
	// UsageSpotlight is from Spotlight's kMDItemLastUsedDate.
	UsageSpotlight
	// UsageLibrarySignal is inferred from ~/Library activity (containers,
	// saved state, preferences, caches).
	UsageLibrarySignal
	// UsageFileDate is the weakest signal: the bundle/executable mod/ time.
	UsageFileDate
)

// Label returns a short human-readable description of the usage source.
func (u UsageSource) Label() string {
	switch u {
	case UsageKnowledge:
		return "usage history"
	case UsageSpotlight:
		return "Spotlight"
	case UsageLibrarySignal:
		return "Library activity"
	case UsageFileDate:
		return "file date"
	default:
		return "unknown"
	}
}

// Approximate reports whether the source is a heuristic rather than a real
// record of the app being opened.
func (u UsageSource) Approximate() bool {
	return u == UsageLibrarySignal || u == UsageFileDate
}

// Source describes how an application was installed.
type Source int

const (
	// SourceUnknown is the zero value used before classification.
	SourceUnknown Source = iota
	// SourceHomebrew is an app installed via a Homebrew cask.
	SourceHomebrew
	// SourceAppStore is an app installed from the Mac App Store.
	SourceAppStore
	// SourceManual is an app installed manually (dmg, pkg, drag-and-drop).
	SourceManual
	// SourceSystem is an Apple/system-shipped app.
	SourceSystem
)

// AppInfo holds everything we discover about a single installed application.
type AppInfo struct {
	Name     string
	Path     string
	BundleID string
	Version  string

	// LastUsed is when the app was last opened, if known.
	LastUsed *time.Time
	// DaysSinceUsed is the whole days since LastUsed, or -1 when unknown/never.
	DaysSinceUsed int
	// LastUsedApprox is true when LastUsed came from a heuristic fallback
	// rather than a real record of the app being opened.
	LastUsedApprox bool
	// LastUsedSource records which signal produced LastUsed.
	LastUsedSource UsageSource

	SizeBytes int64

	Source Source
	// CaskToken is the Homebrew cask token when Source is SourceHomebrew.
	CaskToken string
}

// SourceLabel returns a short human-readable label for the install source.
func (a AppInfo) SourceLabel() string {
	switch a.Source {
	case SourceHomebrew:
		if a.CaskToken != "" {
			return "brew: " + a.CaskToken
		}
		return "Homebrew"
	case SourceAppStore:
		return "App Store"
	case SourceManual:
		return "Manual"
	case SourceSystem:
		return "System"
	default:
		return "Unknown"
	}
}

// LastUsedLabel renders the last-used date as a compact relative string.
func (a AppInfo) LastUsedLabel() string {
	if a.LastUsed == nil || a.DaysSinceUsed < 0 {
		return "Never"
	}
	var s string
	switch {
	case a.DaysSinceUsed == 0:
		s = "Today"
	case a.DaysSinceUsed == 1:
		s = "Yesterday"
	case a.DaysSinceUsed < 365:
		s = fmt.Sprintf("%dd ago", a.DaysSinceUsed)
	default:
		years := a.DaysSinceUsed / 365
		s = fmt.Sprintf("%dy ago", years)
	}
	if a.LastUsedApprox {
		s = "~" + s
	}
	return s
}

// SizeLabel renders the on-disk size in human-readable units.
func (a AppInfo) SizeLabel() string {
	return HumanSize(a.SizeBytes)
}

// IsStale reports whether the app has not been used within thresholdDays.
// Apps that have never been used (or whose last-used date is unknown) count as
// stale.
func (a AppInfo) IsStale(thresholdDays int) bool {
	if a.LastUsed == nil || a.DaysSinceUsed < 0 {
		return true
	}
	return a.DaysSinceUsed >= thresholdDays
}

// HumanSize converts a byte count into a compact human-readable string.
func HumanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
