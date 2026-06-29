package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cibermania5/appfossil/internal/model"
	"github.com/cibermania5/appfossil/internal/scan"
	"github.com/cibermania5/appfossil/internal/tui"
	"github.com/cibermania5/appfossil/internal/version"
	"github.com/mattn/go-isatty"
)

func main() {
	days := flag.Int("days", 90, "staleness threshold in days")
	includeSystem := flag.Bool("include-system", false, "also scan /System/Applications")
	asJSON := flag.Bool("json", false, "print a JSON report instead of the interactive UI")
	mdPath := flag.String("md", "", "write a Markdown report to this file (use '-' for stdout)")
	staleOnly := flag.Bool("stale-only", false, "only include stale apps in the report output")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Line())
		return
	}

	consoleMode := *asJSON || *mdPath != ""
	interactive := !consoleMode && isatty.IsTerminal(os.Stdout.Fd())

	if interactive {
		if err := tui.Run(tui.Config{ThresholdDays: *days, IncludeSystem: *includeSystem}); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	sc := scan.New(scan.Options{IncludeSystem: *includeSystem})
	apps := sc.Scan()
	diag := sc.Diagnostics()
	if *staleOnly {
		apps = filterStale(apps, *days)
	}

	if *mdPath != "" {
		if err := writeMarkdown(*mdPath, apps, *days, diag); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if *asJSON {
		printJSON(apps, *days)
		return
	}
	printText(apps, *days, diag)
}

// accuracyHint returns a one-line suggestion when usage history is unavailable.
func accuracyHint(diag scan.Diagnostics) string {
	if diag.KnowledgeReadable {
		return ""
	}
	if diag.Elevated {
		return "Note: precise usage history (knowledgeC.db) was unavailable; " +
			"grant your terminal Full Disk Access for best accuracy."
	}
	return "Note: dates are approximate. Re-run with sudo (or grant Full Disk " +
		"Access) to read macOS usage history for precise last-used dates."
}

func usage() {
	fmt.Fprintf(os.Stderr, `appfossil - inspect unused macOS apps

Usage:
  appfossil [flags]

Flags:
  -days N           staleness threshold in days (default 90)
  -include-system   also scan /System/Applications
  -json             print a JSON report instead of the interactive UI
  -md FILE          write a Markdown report to FILE (use '-' for stdout)
  -stale-only       only include stale apps in the report output
  -version          print version and exit

With no flags and a terminal attached, an interactive TUI launches.
When output is piped, or -json/-md is set, a report is produced instead.

Examples:
  appfossil -md report.md            # write a Markdown report
  appfossil -md report.md -days 180  # 6-month threshold
  appfossil -stale-only -md -        # Markdown of stale apps to stdout
`)
}

func filterStale(apps []model.AppInfo, days int) []model.AppInfo {
	out := make([]model.AppInfo, 0, len(apps))
	for _, a := range apps {
		if a.IsStale(days) {
			out = append(out, a)
		}
	}
	return out
}

type jsonApp struct {
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	BundleID       string  `json:"bundle_id"`
	Version        string  `json:"version"`
	Source         string  `json:"source"`
	CaskToken      string  `json:"cask_token,omitempty"`
	LastUsed       *string `json:"last_used"`
	LastUsedAprox  bool    `json:"last_used_approx"`
	LastUsedFrom   string  `json:"last_used_from"`
	DaysSinceUsed  int     `json:"days_since_used"`
	SizeBytes      int64   `json:"size_bytes"`
	Stale          bool    `json:"stale"`
	RemoveCommand  string  `json:"remove_command,omitempty"`
}

func printJSON(apps []model.AppInfo, days int) {
	out := make([]jsonApp, 0, len(apps))
	for _, a := range apps {
		var last *string
		if a.LastUsed != nil {
			s := a.LastUsed.Format("2006-01-02T15:04:05Z07:00")
			last = &s
		}
		entry := jsonApp{
			Name:          a.Name,
			Path:          a.Path,
			BundleID:      a.BundleID,
			Version:       a.Version,
			Source:        a.SourceLabel(),
			CaskToken:     a.CaskToken,
			LastUsed:      last,
			LastUsedAprox: a.LastUsedApprox,
			LastUsedFrom:  a.LastUsedSource.Label(),
			DaysSinceUsed: a.DaysSinceUsed,
			SizeBytes:     a.SizeBytes,
			Stale:         a.IsStale(days),
		}
		if cmd, ok := removalCommand(a); ok {
			entry.RemoveCommand = cmd
		}
		out = append(out, entry)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func printText(apps []model.AppInfo, days int, diag scan.Diagnostics) {
	var stale int
	var staleBytes int64
	for _, a := range apps {
		if a.IsStale(days) {
			stale++
			staleBytes += a.SizeBytes
		}
	}

	fmt.Printf("%s\n", lipgloss.NewStyle().Bold(true).Render("appfossil report"))
	fmt.Printf("%d apps · %d stale (>%dd) · %s reclaimable\n", len(apps), stale, days, model.HumanSize(staleBytes))
	if hint := accuracyHint(diag); hint != "" {
		fmt.Printf("%s\n", hint)
	}
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "APP\tLAST USED\tSIZE\tSOURCE")
	for _, a := range apps {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.Name, a.LastUsedLabel(), a.SizeLabel(), a.SourceLabel())
	}
	_ = w.Flush()
}

// writeMarkdown renders the report as Markdown and writes it to path, or to
// stdout when path is "-".
func writeMarkdown(path string, apps []model.AppInfo, days int, diag scan.Diagnostics) error {
	content := buildMarkdown(apps, days, diag)
	if path == "-" {
		_, err := os.Stdout.WriteString(content)
		return err
	}
	if err := writeFileNoFollow(path, []byte(content), 0o644); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	fmt.Fprintf(os.Stderr, "Wrote Markdown report for %d apps to %s\n", len(apps), abs)
	return nil
}

func writeFileNoFollow(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// buildMarkdown produces the full Markdown report document.
func buildMarkdown(apps []model.AppInfo, days int, diag scan.Diagnostics) string {
	var stale int
	var staleBytes, totalBytes int64
	staleApps := make([]model.AppInfo, 0)
	approx := false
	for _, a := range apps {
		totalBytes += a.SizeBytes
		if a.LastUsedApprox {
			approx = true
		}
		if a.IsStale(days) {
			stale++
			staleBytes += a.SizeBytes
			staleApps = append(staleApps, a)
		}
	}

	var b strings.Builder
	b.WriteString("# appfossil report\n\n")
	fmt.Fprintf(&b, "_Generated %s_\n\n", time.Now().Format("Mon 2 Jan 2006, 15:04"))

	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "- Apps scanned: **%d**\n", len(apps))
	fmt.Fprintf(&b, "- Stale (not used in %d+ days): **%d**\n", days, stale)
	fmt.Fprintf(&b, "- Reclaimable if stale apps removed: **%s**\n", model.HumanSize(staleBytes))
	fmt.Fprintf(&b, "- Total size of scanned apps: **%s**\n", model.HumanSize(totalBytes))
	accuracy := "precise (usage history)"
	if !diag.KnowledgeReadable {
		accuracy = "approximate (no usage history access)"
	}
	fmt.Fprintf(&b, "- Date accuracy: **%s**\n\n", accuracy)
	b.WriteString("> Privacy: this report contains local app paths, bundle IDs, install sources, ")
	b.WriteString("and last-used dates. Treat it as personal metadata before sharing or committing.\n\n")

	if hint := accuracyHint(diag); hint != "" {
		fmt.Fprintf(&b, "> %s\n\n", hint)
	}

	fmt.Fprintf(&b, "## Stale apps (not used in %d+ days)\n\n", days)
	if len(staleApps) == 0 {
		b.WriteString("_None. Nice and tidy._\n\n")
	} else {
		writeMarkdownTable(&b, staleApps)
		b.WriteString("\n")
	}

	writeRemovalSection(&b, staleApps)

	b.WriteString("## All apps\n\n")
	writeMarkdownTable(&b, apps)

	if approx {
		b.WriteString("\n> Dates marked with `~` are approximate: macOS had no usage-history ")
		b.WriteString("record, so a file activity date was used as a fallback.\n")
	}
	return b.String()
}

// removalCommand returns a copy-paste shell command to remove an app, or false
// when the app should not be removed (system/unknown).
func removalCommand(a model.AppInfo) (string, bool) {
	switch a.Source {
	case model.SourceHomebrew:
		if a.CaskToken != "" {
			return fmt.Sprintf("brew uninstall --cask %s  # %s", a.CaskToken, a.Name), true
		}
		if a.Path == "" {
			return "", false
		}
		return fmt.Sprintf("mv %s ~/.Trash/  # %s", shellQuote(a.Path), a.Name), true
	case model.SourceManual, model.SourceAppStore:
		if a.Path == "" {
			return "", false
		}
		return fmt.Sprintf("mv %s ~/.Trash/  # %s", shellQuote(a.Path), a.Name), true
	default:
		return "", false
	}
}

func shellQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// writeRemovalSection emits grouped removal commands for stale apps.
func writeRemovalSection(b *strings.Builder, apps []model.AppInfo) {
	if len(apps) == 0 {
		return
	}

	var brew, manual, appStore []string
	var systemSkipped int

	for _, a := range apps {
		cmd, ok := removalCommand(a)
		if !ok {
			if a.Source == model.SourceSystem {
				systemSkipped++
			}
			continue
		}
		switch a.Source {
		case model.SourceHomebrew:
			brew = append(brew, cmd)
		case model.SourceManual:
			manual = append(manual, cmd)
		case model.SourceAppStore:
			appStore = append(appStore, cmd)
		}
	}

	if len(brew)+len(manual)+len(appStore) == 0 && systemSkipped == 0 {
		return
	}

	b.WriteString("## How to remove these apps\n\n")
	b.WriteString("> **Warning:** appfossil never removes anything itself. ")
	b.WriteString("Review each command and quit the app first. ")
	b.WriteString("`mv ... ~/.Trash/` is reversible; `brew uninstall` is not.\n\n")

	if len(brew) > 0 {
		b.WriteString("### Homebrew casks\n\n```bash\n")
		for _, cmd := range brew {
			b.WriteString(cmd)
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	if len(manual) > 0 {
		b.WriteString("### Manual installs\n\n```bash\n")
		for _, cmd := range manual {
			b.WriteString(cmd)
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	if len(appStore) > 0 {
		b.WriteString("### App Store\n\n")
		b.WriteString("_You can also remove App Store apps from Launchpad (hold the icon, then click Delete)._\n\n")
		b.WriteString("```bash\n")
		for _, cmd := range appStore {
			b.WriteString(cmd)
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	if systemSkipped > 0 {
		fmt.Fprintf(b, "_Skipped %d system app(s) — do not remove macOS system applications._\n\n", systemSkipped)
	}
}

// writeMarkdownTable writes a GitHub-flavored Markdown table for the apps.
func writeMarkdownTable(b *strings.Builder, apps []model.AppInfo) {
	b.WriteString("| App | Last Used | Days Idle | Size | Source | Date From | Bundle ID |\n")
	b.WriteString("| --- | --- | --: | --: | --- | --- | --- |\n")
	for _, a := range apps {
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s |\n",
			mdCell(a.Name),
			mdCell(a.LastUsedLabel()),
			mdCell(daysCell(a.DaysSinceUsed)),
			mdCell(a.SizeLabel()),
			mdCell(a.SourceLabel()),
			mdCell(a.LastUsedSource.Label()),
			mdCell(orNA(a.BundleID)),
		)
	}
}

func daysCell(d int) string {
	if d < 0 {
		return "—"
	}
	return fmt.Sprintf("%d", d)
}

func orNA(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// mdCell escapes app-controlled content before putting it in a Markdown table.
func mdCell(s string) string {
	s = html.EscapeString(s)
	replacer := strings.NewReplacer(
		`\`, `\\`,
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
	)
	s = replacer.Replace(s)
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
