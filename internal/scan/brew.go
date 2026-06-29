package scan

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
)

const brewInfoBatchSize = 100

var trustedBrewPaths = []string{
	"/opt/homebrew/bin/brew",
	"/usr/local/bin/brew",
}

// brewMap maps installed .app bundle filenames to the Homebrew cask token that
// installed them. It is empty (but non-nil) when Homebrew is unavailable.
type brewMap struct {
	appToToken map[string]string // lowercased "foo.app" -> cask token
}

// lookup returns the cask token for a given .app filename, if known.
func (m *brewMap) lookup(appFileName string) (string, bool) {
	if m == nil || m.appToToken == nil {
		return "", false
	}
	t, ok := m.appToToken[strings.ToLower(appFileName)]
	return t, ok
}

// brewInfoV2 mirrors the relevant subset of `brew info --json=v2 --cask`.
type brewInfoV2 struct {
	Casks []struct {
		Token     string                       `json:"token"`
		Artifacts []map[string]json.RawMessage `json:"artifacts"`
	} `json:"casks"`
}

// buildBrewMap queries Homebrew once for all installed casks and builds the
// app-to-token lookup. Errors are swallowed: a missing/failed brew simply
// yields an empty map and apps fall through to other classification.
func buildBrewMap() *brewMap {
	m := &brewMap{appToToken: map[string]string{}}

	brew, ok := brewPath()
	if !ok {
		return m
	}

	listOut, err := commandOutputLimited(brewCommandTimeout, maxBrewListBytes, brew, "list", "--cask", "-1")
	if err != nil {
		return m
	}
	tokens := strings.Fields(string(listOut))
	if len(tokens) == 0 {
		return m
	}

	for start := 0; start < len(tokens); start += brewInfoBatchSize {
		end := start + brewInfoBatchSize
		if end > len(tokens) {
			end = len(tokens)
		}

		args := append([]string{"info", "--json=v2", "--cask"}, tokens[start:end]...)
		infoOut, err := commandOutputLimited(brewCommandTimeout, maxBrewInfoBytes, brew, args...)
		if err != nil {
			continue
		}

		var info brewInfoV2
		if err := json.Unmarshal(infoOut, &info); err != nil {
			continue
		}

		for _, cask := range info.Casks {
			for _, artifact := range cask.Artifacts {
				raw, ok := artifact["app"]
				if !ok {
					continue
				}
				for _, name := range parseAppArtifact(raw) {
					m.appToToken[strings.ToLower(name)] = cask.Token
				}
			}
		}
	}
	return m
}

func brewPath() (string, bool) {
	if isElevated() {
		for _, path := range trustedBrewPaths {
			if executableFile(path) {
				return path, true
			}
		}
		return "", false
	}
	brew, err := exec.LookPath("brew")
	if err != nil {
		return "", false
	}
	return brew, true
}

// parseAppArtifact extracts .app filenames from an "app" artifact value, which
// may contain plain strings ("Foo.app") and/or objects with a "target" path.
func parseAppArtifact(raw json.RawMessage) []string {
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return nil
	}
	var names []string
	for _, e := range elems {
		var s string
		if err := json.Unmarshal(e, &s); err == nil {
			names = append(names, filepath.Base(s))
			continue
		}
		var obj struct {
			Target string `json:"target"`
		}
		if err := json.Unmarshal(e, &obj); err == nil && obj.Target != "" {
			names = append(names, filepath.Base(obj.Target))
		}
	}
	return names
}
