package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cibermania5/appfossil/internal/model"
)

func tPtr(t time.Time) *time.Time { return &t }

func sampleApps() []model.AppInfo {
	now := time.Now()
	return []model.AppInfo{
		{Name: "Zoom", Source: model.SourceManual, LastUsed: tPtr(now), DaysSinceUsed: 5, SizeBytes: 100},
		{Name: "Obsidian", Source: model.SourceHomebrew, CaskToken: "obsidian", LastUsed: tPtr(now), DaysSinceUsed: 200, SizeBytes: 500},
		{Name: "Xcode", Source: model.SourceAppStore, LastUsed: nil, DaysSinceUsed: -1, SizeBytes: 9000},
		{Name: "Safari", Source: model.SourceSystem, LastUsed: tPtr(now), DaysSinceUsed: 1, SizeBytes: 50},
	}
}

func names(apps []model.AppInfo) []string {
	out := make([]string, len(apps))
	for i, a := range apps {
		out[i] = a.Name
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortApps(t *testing.T) {
	// Stale: never-used first, then by days desc.
	apps := sampleApps()
	sortApps(apps, sortStale)
	if got, want := names(apps), []string{"Xcode", "Obsidian", "Zoom", "Safari"}; !eq(got, want) {
		t.Errorf("sortStale = %v, want %v", got, want)
	}

	apps = sampleApps()
	sortApps(apps, sortSize)
	if got, want := names(apps), []string{"Xcode", "Obsidian", "Zoom", "Safari"}; !eq(got, want) {
		t.Errorf("sortSize = %v, want %v", got, want)
	}

	apps = sampleApps()
	sortApps(apps, sortName)
	if got, want := names(apps), []string{"Obsidian", "Safari", "Xcode", "Zoom"}; !eq(got, want) {
		t.Errorf("sortName = %v, want %v", got, want)
	}
}

func TestApplyFilters(t *testing.T) {
	apps := sampleApps()

	// Source filter: brew only.
	got := applyFilters(apps, sortName, filterBrew, false, 90, "")
	if want := []string{"Obsidian"}; !eq(names(got), want) {
		t.Errorf("filterBrew = %v, want %v", names(got), want)
	}

	// Stale-only at threshold 90: Obsidian (200) and Xcode (never).
	got = applyFilters(apps, sortName, filterAll, true, 90, "")
	if want := []string{"Obsidian", "Xcode"}; !eq(names(got), want) {
		t.Errorf("staleOnly = %v, want %v", names(got), want)
	}

	// Name query is case-insensitive and substring.
	got = applyFilters(apps, sortName, filterAll, false, 90, "AR")
	if want := []string{"Safari"}; !eq(names(got), want) {
		t.Errorf("query = %v, want %v", names(got), want)
	}
}

func TestSourceFilterMatches(t *testing.T) {
	a := model.AppInfo{Source: model.SourceHomebrew}
	if !filterBrew.matches(a) || filterManual.matches(a) || !filterAll.matches(a) {
		t.Errorf("source filter matching is wrong for %v", a.Source)
	}
}

func TestFit(t *testing.T) {
	cases := []struct {
		s    string
		w    int
		want string
	}{
		{"App", 5, "App  "},
		{"hello", 3, "he…"},
		{"hi", 2, "hi"},
		{"", 0, ""},
		{"toolong", 1, "…"},
	}
	for _, c := range cases {
		got := fit(c.s, c.w)
		if got != c.want {
			t.Errorf("fit(%q, %d) = %q, want %q", c.s, c.w, got, c.want)
		}
		if c.w > 0 && lipgloss.Width(got) != c.w {
			t.Errorf("fit(%q, %d) width = %d, want %d", c.s, c.w, lipgloss.Width(got), c.w)
		}
	}
}

func TestColWidths(t *testing.T) {
	name, last, size, source := colWidths(80)
	if last != 12 || size != 10 {
		t.Errorf("fixed columns wrong: last=%d size=%d", last, size)
	}
	if total := name + last + size + source + 3; total != 80 {
		t.Errorf("columns + gaps = %d, want 80", total)
	}
	if name < 8 {
		t.Errorf("name column too small: %d", name)
	}

	// Narrow terminal: name must not go below the floor.
	nName, _, _, _ := colWidths(40)
	if nName < 8 {
		t.Errorf("narrow name column too small: %d", nName)
	}
}

func TestSortModeString(t *testing.T) {
	if sortStale.String() != "stale" || sortSize.String() != "size" || sortName.String() != "name" {
		t.Error("sortMode.String mismatch")
	}
}
