package model

import (
	"testing"
	"time"
)

func TestHumanSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{5 * 1024 * 1024 * 1024, "5.0 GB"},
	}
	for _, c := range cases {
		if got := HumanSize(c.in); got != c.want {
			t.Errorf("HumanSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestLastUsedLabel(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		app  AppInfo
		want string
	}{
		{"never", AppInfo{LastUsed: nil, DaysSinceUsed: -1}, "Never"},
		{"today", AppInfo{LastUsed: ptrTime(now), DaysSinceUsed: 0}, "Today"},
		{"yesterday", AppInfo{LastUsed: ptrTime(now), DaysSinceUsed: 1}, "Yesterday"},
		{"days", AppInfo{LastUsed: ptrTime(now), DaysSinceUsed: 42}, "42d ago"},
		{"years", AppInfo{LastUsed: ptrTime(now), DaysSinceUsed: 800}, "2y ago"},
		{"approx", AppInfo{LastUsed: ptrTime(now), DaysSinceUsed: 42, LastUsedApprox: true}, "~42d ago"},
	}
	for _, c := range cases {
		if got := c.app.LastUsedLabel(); got != c.want {
			t.Errorf("%s: LastUsedLabel() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSourceLabel(t *testing.T) {
	cases := []struct {
		app  AppInfo
		want string
	}{
		{AppInfo{Source: SourceHomebrew, CaskToken: "obsidian"}, "brew: obsidian"},
		{AppInfo{Source: SourceHomebrew}, "Homebrew"},
		{AppInfo{Source: SourceAppStore}, "App Store"},
		{AppInfo{Source: SourceManual}, "Manual"},
		{AppInfo{Source: SourceSystem}, "System"},
		{AppInfo{Source: SourceUnknown}, "Unknown"},
	}
	for _, c := range cases {
		if got := c.app.SourceLabel(); got != c.want {
			t.Errorf("SourceLabel(%v) = %q, want %q", c.app.Source, got, c.want)
		}
	}
}

func TestUsageSource(t *testing.T) {
	cases := []struct {
		src    UsageSource
		label  string
		approx bool
	}{
		{UsageKnowledge, "usage history", false},
		{UsageSpotlight, "Spotlight", false},
		{UsageLibrarySignal, "Library activity", true},
		{UsageFileDate, "file date", true},
		{UsageNone, "unknown", false},
	}
	for _, c := range cases {
		if got := c.src.Label(); got != c.label {
			t.Errorf("%v.Label() = %q, want %q", c.src, got, c.label)
		}
		if got := c.src.Approximate(); got != c.approx {
			t.Errorf("%v.Approximate() = %v, want %v", c.src, got, c.approx)
		}
	}
}

func TestIsStale(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		app       AppInfo
		threshold int
		want      bool
	}{
		{"never is stale", AppInfo{LastUsed: nil, DaysSinceUsed: -1}, 90, true},
		{"fresh not stale", AppInfo{LastUsed: ptrTime(now), DaysSinceUsed: 10}, 90, false},
		{"exactly threshold is stale", AppInfo{LastUsed: ptrTime(now), DaysSinceUsed: 90}, 90, true},
		{"over threshold is stale", AppInfo{LastUsed: ptrTime(now), DaysSinceUsed: 365}, 90, true},
	}
	for _, c := range cases {
		if got := c.app.IsStale(c.threshold); got != c.want {
			t.Errorf("%s: IsStale(%d) = %v, want %v", c.name, c.threshold, got, c.want)
		}
	}
}
