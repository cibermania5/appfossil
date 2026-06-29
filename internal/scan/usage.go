package scan

import (
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// macAbsoluteEpoch is the number of seconds between the Unix epoch
// (1970-01-01) and the Mac/CoreData absolute time epoch (2001-01-01).
const macAbsoluteEpoch = 978307200

const sqlitePath = "/usr/bin/sqlite3"

// usageDB holds precise per-bundle last-used timestamps read from CoreDuet's
// knowledgeC.db. It is always non-nil; Readable reports whether any database
// could actually be opened (which typically requires root or Full Disk Access).
type usageDB struct {
	byBundle map[string]time.Time
	Readable bool
}

// lookup returns the most recent recorded usage time for a bundle id.
func (u *usageDB) lookup(bundleID string) (time.Time, bool) {
	if u == nil || bundleID == "" {
		return time.Time{}, false
	}
	t, ok := u.byBundle[bundleID]
	return t, ok
}

// knowledgeDBPaths returns the candidate knowledgeC.db locations: the
// system-wide CoreDuet store and the invoking user's per-user store.
func knowledgeDBPaths() []string {
	paths := []string{
		"/private/var/db/CoreDuet/Knowledge/knowledgeC.db",
	}
	if home := realUserHome(); home != "" {
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", "Knowledge", "knowledgeC.db"),
		)
	}
	return paths
}

// knowledgeQuery aggregates the most recent app usage/focus event per bundle.
const knowledgeQuery = `SELECT ZVALUESTRING, MAX(ZENDDATE) FROM ZOBJECT ` +
	`WHERE ZSTREAMNAME IN ('/app/usage','/app/inFocus','/app/activity') ` +
	`AND ZVALUESTRING IS NOT NULL GROUP BY ZVALUESTRING;`

// buildUsageDB reads all available knowledgeC.db files and merges them, keeping
// the most recent timestamp per bundle id. Failures are swallowed: the result
// is simply empty and callers fall back to other signals.
func buildUsageDB() *usageDB {
	db := &usageDB{byBundle: map[string]time.Time{}}

	if !executableFile(sqlitePath) {
		return db
	}

	for _, path := range knowledgeDBPaths() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		// Open read-only and as immutable so a stale -wal/-shm or lack of write
		// access to the directory cannot block the query.
		out, err := commandOutputLimited(metadataCommandTimeout, maxSQLiteBytes, sqlitePath, "-readonly", "-separator", "|", sqliteURI(path), knowledgeQuery)
		if err != nil {
			continue
		}
		db.Readable = true
		for bundle, t := range parseUsageRows(string(out)) {
			if cur, ok := db.byBundle[bundle]; !ok || t.After(cur) {
				db.byBundle[bundle] = t
			}
		}
	}
	return db
}

func sqliteURI(path string) string {
	u := url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: "immutable=1",
	}
	return u.String()
}

// parseUsageRows parses "bundle|zenddate" lines into a bundle->time map.
func parseUsageRows(data string) map[string]time.Time {
	out := map[string]time.Time{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.LastIndex(line, "|")
		if i <= 0 {
			continue
		}
		bundle := strings.TrimSpace(line[:i])
		secs, err := strconv.ParseFloat(strings.TrimSpace(line[i+1:]), 64)
		if err != nil || secs <= 0 {
			continue
		}
		t := macAbsoluteToTime(secs)
		if cur, ok := out[bundle]; !ok || t.After(cur) {
			out[bundle] = t
		}
	}
	return out
}

// macAbsoluteToTime converts CoreData/Mac absolute seconds to a time.Time.
func macAbsoluteToTime(secs float64) time.Time {
	return time.Unix(int64(secs)+macAbsoluteEpoch, 0)
}

// realUserHome resolves the home directory of the human user, even when the
// process is running under sudo (where $HOME may point at /var/root).
func realUserHome() string {
	if su := os.Getenv("SUDO_USER"); isElevated() && su != "" && su != "root" {
		if u, err := user.Lookup(su); err == nil && u.HomeDir != "" {
			return u.HomeDir
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// isElevated reports whether the process is running as root.
func isElevated() bool {
	return os.Geteuid() == 0
}
