package scan

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cibermania5/appfossil/internal/model"
)

// classify determines how an app was installed. Order matters: an App Store
// receipt or system location is authoritative, then Homebrew, then manual.
func (m *brewMap) classify(app *model.AppInfo) (model.Source, string) {
	if isSystemApp(app) {
		return model.SourceSystem, ""
	}
	if isAppStoreApp(app.Path) {
		return model.SourceAppStore, ""
	}
	if token, ok := m.lookup(filepath.Base(app.Path)); ok {
		return model.SourceHomebrew, token
	}
	return model.SourceManual, ""
}

// isAppStoreApp reports whether the bundle carries a Mac App Store receipt.
func isAppStoreApp(path string) bool {
	receipt := filepath.Join(path, "Contents", "_MASReceipt", "receipt")
	_, err := os.Stat(receipt)
	return err == nil
}

// isSystemApp reports whether the app ships with macOS.
func isSystemApp(app *model.AppInfo) bool {
	if strings.HasPrefix(app.Path, "/System/") {
		return true
	}
	return false
}
