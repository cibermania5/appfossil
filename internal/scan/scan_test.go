package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cibermania5/appfossil/internal/model"
)

func TestParseAppArtifact(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"plain string", `["Obsidian.app"]`, []string{"Obsidian.app"}},
		{"with subdir", `["nested/Foo.app"]`, []string{"Foo.app"}},
		{"target object", `[{"target": "/Applications/Bar.app"}]`, []string{"Bar.app"}},
		{"mixed", `["A.app", {"target": "/Applications/B.app"}]`, []string{"A.app", "B.app"}},
		{"not an array", `"oops"`, nil},
	}
	for _, c := range cases {
		got := parseAppArtifact(json.RawMessage(c.raw))
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

func TestBrewMapLookup(t *testing.T) {
	m := &brewMap{appToToken: map[string]string{"obsidian.app": "obsidian"}}

	if tok, ok := m.lookup("Obsidian.app"); !ok || tok != "obsidian" {
		t.Errorf("case-insensitive lookup failed: %q %v", tok, ok)
	}
	if _, ok := m.lookup("Unknown.app"); ok {
		t.Error("expected miss for unknown app")
	}

	var nilMap *brewMap
	if _, ok := nilMap.lookup("x.app"); ok {
		t.Error("nil brewMap lookup should miss")
	}
}

func TestClassify(t *testing.T) {
	m := &brewMap{appToToken: map[string]string{"obsidian.app": "obsidian"}}

	// App Store: bundle with a MAS receipt.
	masBundle := filepath.Join(t.TempDir(), "Paid.app")
	mustMkdir(t, filepath.Join(masBundle, "Contents", "_MASReceipt"))
	mustWrite(t, filepath.Join(masBundle, "Contents", "_MASReceipt", "receipt"), "x")

	cases := []struct {
		name      string
		app       model.AppInfo
		wantSrc   model.Source
		wantToken string
	}{
		{"system by path", model.AppInfo{Path: "/System/Applications/Notes.app"}, model.SourceSystem, ""},
		{"spoofed apple bundle id", model.AppInfo{Path: "/Applications/Notes.app", BundleID: "com.apple.Notes"}, model.SourceManual, ""},
		{"app store", model.AppInfo{Path: masBundle}, model.SourceAppStore, ""},
		{"homebrew", model.AppInfo{Path: "/Applications/Obsidian.app", BundleID: "md.obsidian"}, model.SourceHomebrew, "obsidian"},
		{"manual", model.AppInfo{Path: "/Applications/Random.app", BundleID: "com.random"}, model.SourceManual, ""},
	}
	for _, c := range cases {
		src, tok := m.classify(&c.app)
		if src != c.wantSrc || tok != c.wantToken {
			t.Errorf("%s: classify = (%v, %q), want (%v, %q)", c.name, src, tok, c.wantSrc, c.wantToken)
		}
	}
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a"), "12345")          // 5 bytes
	mustMkdir(t, filepath.Join(dir, "sub"))                 //
	mustWrite(t, filepath.Join(dir, "sub", "b"), "67890ab") // 7 bytes

	if got := dirSize(dir); got != 12 {
		t.Errorf("dirSize = %d, want 12", got)
	}
}

func TestDirSizeSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	mustWrite(t, target, "12345")
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if got := dirSize(dir); got != 5 {
		t.Errorf("dirSize = %d, want 5", got)
	}
}

func TestReadLastUsedPrefersKnowledge(t *testing.T) {
	when := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	udb := &usageDB{byBundle: map[string]time.Time{"md.obsidian": when}}

	app := model.AppInfo{Path: "/Applications/Obsidian.app", BundleID: "md.obsidian"}
	readLastUsed(&app, udb)

	if app.LastUsedSource != model.UsageKnowledge {
		t.Errorf("source = %v, want UsageKnowledge", app.LastUsedSource)
	}
	if app.LastUsedApprox {
		t.Error("knowledge-derived date should not be approximate")
	}
	if app.LastUsed == nil || !app.LastUsed.Equal(when) {
		t.Errorf("LastUsed = %v, want %v", app.LastUsed, when)
	}
	if app.DaysSinceUsed < 0 {
		t.Error("DaysSinceUsed should be set")
	}
}

func TestSafeBundleIDRejectsTraversal(t *testing.T) {
	cases := map[string]bool{
		"com.example.App":      true,
		"com.example-app.App2": true,
		"":                     false,
		"../Library":           false,
		"com.example/../../x":  false,
		"/private/var/db":      false,
		`com.example\evil`:     false,
		"com..example":         false,
	}
	for bundleID, want := range cases {
		if got := safeBundleID(bundleID); got != want {
			t.Errorf("safeBundleID(%q) = %v, want %v", bundleID, got, want)
		}
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
