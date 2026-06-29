package scan

import (
	"testing"
	"time"
)

func TestMacAbsoluteToTime(t *testing.T) {
	// Mac absolute time 0 == 2001-01-01T00:00:00Z.
	got := macAbsoluteToTime(0).UTC()
	want := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("macAbsoluteToTime(0) = %v, want %v", got, want)
	}

	// A known later instant: 700000000s after 2001 epoch.
	got2 := macAbsoluteToTime(700000000).UTC()
	want2 := time.Unix(700000000+macAbsoluteEpoch, 0).UTC()
	if !got2.Equal(want2) {
		t.Errorf("macAbsoluteToTime(700000000) = %v, want %v", got2, want2)
	}
}

func TestParseUsageRows(t *testing.T) {
	// Bundle ids can contain no pipes; values are mac-absolute seconds.
	// "md.obsidian" appears twice; the newer (larger) value must win.
	data := "" +
		"com.apple.Safari|700000000\n" +
		"md.obsidian|600000000\n" +
		"md.obsidian|650000000\n" +
		"\n" +
		"broken-line\n" +
		"bad.time|notanumber\n" +
		"zero.value|0\n"

	got := parseUsageRows(data)

	if len(got) != 2 {
		t.Fatalf("expected 2 bundles, got %d: %v", len(got), got)
	}
	if _, ok := got["com.apple.Safari"]; !ok {
		t.Error("missing com.apple.Safari")
	}
	ob, ok := got["md.obsidian"]
	if !ok {
		t.Fatal("missing md.obsidian")
	}
	if want := macAbsoluteToTime(650000000); !ob.Equal(want) {
		t.Errorf("md.obsidian = %v, want newest %v", ob, want)
	}
	if _, ok := got["zero.value"]; ok {
		t.Error("zero/invalid timestamps should be skipped")
	}
}

func TestUsageDBLookup(t *testing.T) {
	now := time.Now()
	db := &usageDB{byBundle: map[string]time.Time{"md.obsidian": now}}

	if _, ok := db.lookup(""); ok {
		t.Error("empty bundle id should miss")
	}
	if _, ok := db.lookup("unknown"); ok {
		t.Error("unknown bundle id should miss")
	}
	if got, ok := db.lookup("md.obsidian"); !ok || !got.Equal(now) {
		t.Errorf("lookup = %v %v, want %v true", got, ok, now)
	}

	var nilDB *usageDB
	if _, ok := nilDB.lookup("md.obsidian"); ok {
		t.Error("nil usageDB lookup should miss")
	}
}
