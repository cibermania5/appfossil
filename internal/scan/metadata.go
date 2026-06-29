package scan

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cibermania5/appfossil/internal/model"
)

const (
	plutilPath = "/usr/bin/plutil"
	mdlsPath   = "/usr/bin/mdls"

	maxBundleFiles = 100_000
	maxBundleBytes = 100 << 30
)

// readBundleMetadata fills in bundle id, version, last-used date and size.
func readBundleMetadata(app *model.AppInfo, udb *usageDB) {
	readInfoPlist(app)
	readLastUsed(app, udb)
	app.SizeBytes = dirSize(app.Path)
}

// infoPlist captures the handful of Info.plist keys we care about.
type infoPlist struct {
	BundleID       string `json:"CFBundleIdentifier"`
	ShortVersion   string `json:"CFBundleShortVersionString"`
	BundleVersion  string `json:"CFBundleVersion"`
	BundleName     string `json:"CFBundleName"`
	BundleExecutab string `json:"CFBundleExecutable"`
}

// readInfoPlist parses Contents/Info.plist via plutil (converted to JSON so we
// avoid a third-party plist dependency).
func readInfoPlist(app *model.AppInfo) {
	plistPath := filepath.Join(app.Path, "Contents", "Info.plist")
	if _, err := os.Stat(plistPath); err != nil {
		return
	}
	out, err := commandOutputLimited(metadataCommandTimeout, maxPlistJSONBytes, plutilPath, "-convert", "json", "-o", "-", plistPath)
	if err != nil {
		return
	}
	var p infoPlist
	if err := json.Unmarshal(out, &p); err != nil {
		return
	}
	app.BundleID = p.BundleID
	if p.ShortVersion != "" {
		app.Version = p.ShortVersion
	} else {
		app.Version = p.BundleVersion
	}
	if p.BundleName != "" {
		app.Name = p.BundleName
	}
}

// lastUsedLayouts are the date formats mdls -raw may emit.
var lastUsedLayouts = []string{
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05 -0700 MST",
}

// readLastUsed resolves the app's last-used date from the strongest available
// signal: CoreDuet usage history (needs sudo/FDA), then Spotlight, then
// ~/Library activity, then file modification dates.
func readLastUsed(app *model.AppInfo, udb *usageDB) {
	if t, ok := udb.lookup(app.BundleID); ok {
		setLastUsed(app, t, model.UsageKnowledge)
		return
	}
	if t, ok := spotlightLastUsed(app.Path); ok {
		setLastUsed(app, t, model.UsageSpotlight)
		return
	}
	if t, ok := libraryActivity(app.BundleID); ok {
		setLastUsed(app, t, model.UsageLibrarySignal)
		return
	}
	if t, ok := executableModTime(app); ok {
		setLastUsed(app, t, model.UsageFileDate)
		return
	}
	if fi, err := os.Stat(app.Path); err == nil {
		setLastUsed(app, fi.ModTime(), model.UsageFileDate)
		return
	}
	app.DaysSinceUsed = -1
	app.LastUsedSource = model.UsageNone
}

// libraryActivity infers recent use from the most recent modification time of
// the app's per-user data under ~/Library. These directories are touched when
// an app runs (containers, saved window state, preferences, caches), so the
// newest mtime is a reasonable lower bound on when the app was last used.
func libraryActivity(bundleID string) (time.Time, bool) {
	if !safeBundleID(bundleID) {
		return time.Time{}, false
	}
	home := realUserHome()
	if home == "" {
		return time.Time{}, false
	}
	lib := filepath.Join(home, "Library")
	candidates := []string{
		filepath.Join(lib, "Saved Application State", bundleID+".savedState"),
		filepath.Join(lib, "Containers", bundleID),
		filepath.Join(lib, "Preferences", bundleID+".plist"),
		filepath.Join(lib, "HTTPStorages", bundleID),
		filepath.Join(lib, "Caches", bundleID),
		filepath.Join(lib, "Application Support", bundleID),
	}
	var newest time.Time
	found := false
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil {
			if fi.ModTime().After(newest) {
				newest = fi.ModTime()
				found = true
			}
		}
	}
	return newest, found
}

func safeBundleID(bundleID string) bool {
	if bundleID == "" || len(bundleID) > 255 || filepath.IsAbs(bundleID) {
		return false
	}
	if strings.ContainsAny(bundleID, `/\`) {
		return false
	}
	for _, part := range strings.Split(bundleID, ".") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// spotlightLastUsed queries mdls for kMDItemLastUsedDate.
func spotlightLastUsed(path string) (time.Time, bool) {
	out, err := commandOutputLimited(metadataCommandTimeout, maxMDLSBytes, mdlsPath, "-name", "kMDItemLastUsedDate", "-raw", path)
	if err != nil {
		return time.Time{}, false
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" || raw == "(null)" {
		return time.Time{}, false
	}
	for _, layout := range lastUsedLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// executableModTime returns the modification time of the bundle's main binary.
func executableModTime(app *model.AppInfo) (time.Time, bool) {
	// Re-read the executable name from Info.plist if we have it; otherwise guess.
	plistPath := filepath.Join(app.Path, "Contents", "Info.plist")
	var execName string
	if out, err := commandOutputLimited(metadataCommandTimeout, 4<<10, plutilPath, "-extract", "CFBundleExecutable", "raw", "-o", "-", plistPath); err == nil {
		// Constrain to a basename: CFBundleExecutable comes from the bundle and
		// could otherwise contain "../" to escape the bundle directory.
		execName = filepath.Base(strings.TrimSpace(string(out)))
	}
	if execName != "" && execName != "." && execName != string(filepath.Separator) {
		exe := filepath.Join(app.Path, "Contents", "MacOS", execName)
		if fi, err := os.Stat(exe); err == nil {
			return fi.ModTime(), true
		}
	}
	// Fall back to the newest file in Contents/MacOS.
	macOS := filepath.Join(app.Path, "Contents", "MacOS")
	entries, err := os.ReadDir(macOS)
	if err != nil {
		return time.Time{}, false
	}
	var newest time.Time
	found := false
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
			found = true
		}
	}
	return newest, found
}

// setLastUsed records the timestamp, its source, and computes the day delta.
func setLastUsed(app *model.AppInfo, t time.Time, src model.UsageSource) {
	tt := t
	app.LastUsed = &tt
	app.LastUsedSource = src
	app.LastUsedApprox = src.Approximate()
	days := int(time.Since(t).Hours() / 24)
	if days < 0 {
		days = 0
	}
	app.DaysSinceUsed = days
}

// dirSize sums the sizes of all regular files within a directory tree.
func dirSize(root string) int64 {
	var total int64
	var files int
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files++
			total += info.Size()
			if files >= maxBundleFiles || total >= maxBundleBytes {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return total
}
