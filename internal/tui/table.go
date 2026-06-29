package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cibermania5/appfossil/internal/model"
)

// sortMode controls how the app list is ordered.
type sortMode int

const (
	sortStale sortMode = iota // least-recently-used first
	sortSize                  // largest first
	sortName                  // alphabetical
)

func (s sortMode) String() string {
	switch s {
	case sortSize:
		return "size"
	case sortName:
		return "name"
	default:
		return "stale"
	}
}

// sourceFilter cycles through "all" plus each install source.
type sourceFilter int

const (
	filterAll sourceFilter = iota
	filterManual
	filterBrew
	filterAppStore
	filterSystem
)

func (f sourceFilter) String() string {
	switch f {
	case filterManual:
		return "manual"
	case filterBrew:
		return "brew"
	case filterAppStore:
		return "app store"
	case filterSystem:
		return "system"
	default:
		return "all"
	}
}

func (f sourceFilter) matches(a model.AppInfo) bool {
	switch f {
	case filterManual:
		return a.Source == model.SourceManual
	case filterBrew:
		return a.Source == model.SourceHomebrew
	case filterAppStore:
		return a.Source == model.SourceAppStore
	case filterSystem:
		return a.Source == model.SourceSystem
	default:
		return true
	}
}

// applyFilters returns the apps matching the current filters, in sort order.
func applyFilters(apps []model.AppInfo, sm sortMode, sf sourceFilter, staleOnly bool, threshold int, query string) []model.AppInfo {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]model.AppInfo, 0, len(apps))
	for _, a := range apps {
		if !sf.matches(a) {
			continue
		}
		if staleOnly && !a.IsStale(threshold) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(a.Name), q) {
			continue
		}
		out = append(out, a)
	}
	sortApps(out, sm)
	return out
}

// sortApps orders apps in place according to the sort mode.
func sortApps(apps []model.AppInfo, sm sortMode) {
	switch sm {
	case sortSize:
		sort.SliceStable(apps, func(i, j int) bool {
			return apps[i].SizeBytes > apps[j].SizeBytes
		})
	case sortName:
		sort.SliceStable(apps, func(i, j int) bool {
			return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
		})
	default:
		scan := func(a model.AppInfo) int {
			if a.LastUsed == nil || a.DaysSinceUsed < 0 {
				return 1 << 30
			}
			return a.DaysSinceUsed
		}
		sort.SliceStable(apps, func(i, j int) bool {
			ai, aj := scan(apps[i]), scan(apps[j])
			if ai != aj {
				return ai > aj
			}
			return apps[i].SizeBytes > apps[j].SizeBytes
		})
	}
}

// fit pads or truncates s to exactly w display cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s + strings.Repeat(" ", w-lipgloss.Width(s))
	}
	if w == 1 {
		return "…"
	}
	// Truncate by runes until it fits, leaving room for the ellipsis.
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > w {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// colWidths computes column widths for the given total table width.
// Order: Name (flex), Last Used, Size, Source.
func colWidths(total int) (name, last, size, source int) {
	last, size, source = 12, 10, 26
	const gaps = 3 // single space between 4 columns
	name = total - last - size - source - gaps
	if name < 16 {
		// Shrink the source column before letting name get too small.
		source = max(12, source-(16-name))
		name = total - last - size - source - gaps
	}
	if name < 8 {
		name = 8
	}
	return
}
