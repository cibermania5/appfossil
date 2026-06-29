package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cibermania5/appfossil/internal/model"
	"github.com/cibermania5/appfossil/internal/scan"
)

func tp(t time.Time) *time.Time { return &t }

func mdApps() []model.AppInfo {
	now := time.Now()
	return []model.AppInfo{
		{Name: "Obsidian", BundleID: "md.obsidian", Source: model.SourceHomebrew, CaskToken: "obsidian",
			LastUsed: tp(now), DaysSinceUsed: 200, SizeBytes: 500, LastUsedApprox: true},
		{Name: "Fresh|App", BundleID: "com.fresh", Source: model.SourceManual,
			LastUsed: tp(now), DaysSinceUsed: 3, SizeBytes: 100},
		{Name: "NeverOpened", Path: "/Applications/NeverOpened.app", BundleID: "", Source: model.SourceAppStore,
			LastUsed: nil, DaysSinceUsed: -1, SizeBytes: 1000},
	}
}

func TestBuildMarkdown(t *testing.T) {
	md := buildMarkdown(mdApps(), 90, scan.Diagnostics{KnowledgeReadable: true})

	wantContains := []string{
		"# appfossil report",
		"## Summary",
		"- Apps scanned: **3**",
		"- Stale (not used in 90+ days): **2**", // Obsidian (200) + NeverOpened
		"## Stale apps (not used in 90+ days)",
		"## All apps",
		"| App | Last Used | Days Idle | Size | Source | Date From | Bundle ID |",
		"brew: obsidian",
		"Fresh\\|App", // pipe escaped
		"approximate", // approx footnote present
		"Privacy: this report contains local app paths",
		"## How to remove these apps",
		"**Warning:** appfossil never removes anything itself",
		"brew uninstall --cask obsidian",
	}
	for _, w := range wantContains {
		if !strings.Contains(md, w) {
			t.Errorf("markdown missing %q\n---\n%s", w, md)
		}
	}

	// The fresh app must not appear in the stale section, but must be in All apps.
	staleSection := md[strings.Index(md, "## Stale apps"):strings.Index(md, "## All apps")]
	if strings.Contains(staleSection, "Fresh") {
		t.Errorf("fresh app leaked into stale section:\n%s", staleSection)
	}
}

func TestBuildMarkdownNoStale(t *testing.T) {
	now := time.Now()
	apps := []model.AppInfo{
		{Name: "Daily", Source: model.SourceManual, LastUsed: tp(now), DaysSinceUsed: 1, SizeBytes: 10},
	}
	md := buildMarkdown(apps, 90, scan.Diagnostics{KnowledgeReadable: true})
	if !strings.Contains(md, "Nice and tidy") {
		t.Errorf("expected empty-stale message, got:\n%s", md)
	}
	if strings.Contains(md, "## How to remove these apps") {
		t.Error("removal section should be omitted when there are no stale apps")
	}
}

func TestRemovalCommand(t *testing.T) {
	cases := []struct {
		name   string
		app    model.AppInfo
		want   string
		wantOK bool
	}{
		{
			name:   "homebrew cask",
			app:    model.AppInfo{Name: "Obsidian", Source: model.SourceHomebrew, CaskToken: "obsidian"},
			want:   "brew uninstall --cask obsidian  # Obsidian",
			wantOK: true,
		},
		{
			name:   "homebrew without token",
			app:    model.AppInfo{Name: "Foo", Path: "/Applications/Foo.app", Source: model.SourceHomebrew},
			want:   `mv "/Applications/Foo.app" ~/.Trash/  # Foo`,
			wantOK: true,
		},
		{
			name:   "manual",
			app:    model.AppInfo{Name: "Bar", Path: "/Applications/Bar.app", Source: model.SourceManual},
			want:   `mv "/Applications/Bar.app" ~/.Trash/  # Bar`,
			wantOK: true,
		},
		{
			name:   "app store",
			app:    model.AppInfo{Name: "Pages", Path: "/Applications/Pages.app", Source: model.SourceAppStore},
			want:   `mv "/Applications/Pages.app" ~/.Trash/  # Pages`,
			wantOK: true,
		},
		{
			name:   "system",
			app:    model.AppInfo{Name: "Notes", Path: "/System/Applications/Notes.app", Source: model.SourceSystem},
			wantOK: false,
		},
		{
			name:   "unknown",
			app:    model.AppInfo{Name: "X", Source: model.SourceUnknown},
			wantOK: false,
		},
	}
	for _, c := range cases {
		got, ok := removalCommand(c.app)
		if ok != c.wantOK {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.wantOK)
		}
		if c.wantOK && got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestWriteMarkdownFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	if err := writeMarkdown(path, mdApps(), 90, scan.Diagnostics{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# appfossil report") {
		t.Errorf("file does not start with report heading: %q", string(data)[:30])
	}
}

func TestWriteMarkdownRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	link := filepath.Join(dir, "report.md")
	if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if err := writeMarkdown(link, mdApps(), 90, scan.Diagnostics{}); err == nil {
		t.Fatal("expected symlink write to fail")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("symlink target was overwritten: %q", data)
	}
}

func TestMarkdownCellEscapesActiveContent(t *testing.T) {
	input := `Bad|<script>*_[link](x)_*`
	got := mdCell(input)
	for _, disallowed := range []string{"<script>", "*_", "[link](x)", "Bad|"} {
		if strings.Contains(got, disallowed) {
			t.Fatalf("mdCell(%q) = %q, still contains %q", input, got, disallowed)
		}
	}
	if !strings.Contains(got, `Bad\|`) {
		t.Fatalf("mdCell(%q) = %q, expected escaped pipe", input, got)
	}
}

func TestPrintJSONRemoveCommand(t *testing.T) {
	apps := []model.AppInfo{
		{
			Name:      "Obsidian",
			Path:      "/Applications/Obsidian.app",
			Source:    model.SourceHomebrew,
			CaskToken: "obsidian",
		},
		{
			Name:   "Notes",
			Path:   "/System/Applications/Notes.app",
			Source: model.SourceSystem,
		},
	}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	printJSON(apps, 90)
	w.Close()
	os.Stdout = old

	var out []jsonApp
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d apps, want 2", len(out))
	}
	if out[0].RemoveCommand != "brew uninstall --cask obsidian  # Obsidian" {
		t.Errorf("brew app remove_command = %q", out[0].RemoveCommand)
	}
	if out[1].RemoveCommand != "" {
		t.Errorf("system app should omit remove_command, got %q", out[1].RemoveCommand)
	}
	raw, err := json.Marshal(out[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "remove_command") {
		t.Errorf("system app JSON should omit remove_command: %s", raw)
	}
}
