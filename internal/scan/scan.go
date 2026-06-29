package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/cibermania5/appfossil/internal/model"
)

// Options controls a scan.
type Options struct {
	// IncludeSystem also scans /System/Applications when true.
	IncludeSystem bool
	// Concurrency is the number of worker goroutines. Defaults to NumCPU.
	Concurrency int
}

// Scanner discovers installed apps and enriches them with metadata.
type Scanner struct {
	opts    Options
	brewMap *brewMap
	usageDB *usageDB
}

// Diagnostics reports environmental facts that affect last-used accuracy.
type Diagnostics struct {
	// Elevated is true when running as root (e.g. via sudo).
	Elevated bool
	// KnowledgeReadable is true when CoreDuet's knowledgeC.db could be read,
	// which yields precise usage history.
	KnowledgeReadable bool
}

// New creates a Scanner. Building the Homebrew cask map and reading the usage
// database happen here so they are done once per scan rather than per app.
func New(opts Options) *Scanner {
	if opts.Concurrency <= 0 {
		opts.Concurrency = runtime.NumCPU()
	}
	return &Scanner{
		opts:    opts,
		brewMap: buildBrewMap(),
		usageDB: buildUsageDB(),
	}
}

// Diagnostics returns accuracy-related facts about the current environment.
func (s *Scanner) Diagnostics() Diagnostics {
	return Diagnostics{
		Elevated:          isElevated(),
		KnowledgeReadable: s.usageDB != nil && s.usageDB.Readable,
	}
}

// searchRoots returns the directories to scan for .app bundles.
func (s *Scanner) searchRoots() []string {
	roots := []string{
		"/Applications",
		"/Applications/Utilities",
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, "Applications"))
	}
	if s.opts.IncludeSystem {
		roots = append(roots,
			"/System/Applications",
			"/System/Applications/Utilities",
		)
	}
	return roots
}

// collectAppPaths walks the search roots and returns the absolute paths of all
// .app bundles found, without descending into the bundles themselves.
func (s *Scanner) collectAppPaths() []string {
	seen := make(map[string]struct{})
	var paths []string

	for _, root := range s.searchRoots() {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".app") {
				if _, dup := seen[path]; !dup {
					seen[path] = struct{}{}
					paths = append(paths, path)
				}
				// Do not descend into the bundle.
				return filepath.SkipDir
			}
			return nil
		})
	}
	return paths
}

// Scan discovers all apps and returns them enriched and sorted with the most
// stale (least recently used) first.
func (s *Scanner) Scan() []model.AppInfo {
	paths := s.collectAppPaths()

	jobs := make(chan string)
	results := make(chan model.AppInfo)

	var wg sync.WaitGroup
	for i := 0; i < s.opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				results <- s.inspect(p)
			}
		}()
	}

	go func() {
		for _, p := range paths {
			jobs <- p
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	apps := make([]model.AppInfo, 0, len(paths))
	for a := range results {
		apps = append(apps, a)
	}

	SortByStaleness(apps)
	return apps
}

// inspect gathers all metadata for a single bundle path.
func (s *Scanner) inspect(path string) model.AppInfo {
	app := model.AppInfo{
		Name: strings.TrimSuffix(filepath.Base(path), ".app"),
		Path: path,
	}
	readBundleMetadata(&app, s.usageDB)
	app.Source, app.CaskToken = s.brewMap.classify(&app)
	return app
}

// SortByStaleness orders apps with never-used/oldest first, then by size
// descending as a tie-breaker so large stale apps bubble up.
func SortByStaleness(apps []model.AppInfo) {
	sort.SliceStable(apps, func(i, j int) bool {
		a, b := apps[i], apps[j]
		ai, bi := staleRank(a), staleRank(b)
		if ai != bi {
			return ai > bi
		}
		return a.SizeBytes > b.SizeBytes
	})
}

// staleRank maps an app to a comparable staleness score (higher = more stale).
func staleRank(a model.AppInfo) int {
	if a.LastUsed == nil || a.DaysSinceUsed < 0 {
		return 1 << 30 // never used sorts to the very top
	}
	return a.DaysSinceUsed
}
