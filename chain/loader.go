package chain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// scenarioYAMLNames are the conventional definition filenames looked for inside a
// scenario folder, in priority order.
var scenarioYAMLNames = []string{"scenario.yaml", "scenario.yml", "chain.yaml", "chain.yml"}

// DiscoverScenarios scans each dir (non-recursively at the top level) for scenario
// definitions. A scenario is either:
//
//   - a *.yaml / *.yml file, or
//   - a folder holding a scenario YAML (scenario.yaml/chain.yaml, else the single
//     *.yaml/*.yml in it) alongside any bundled resources.
//
// The token {{scenario_dir}} in a definition is replaced with the absolute path of
// the scenario's containing directory before parsing, so bundled resources can be
// referenced portably (e.g. run: '["python3", "{{scenario_dir}}/exploit.py"]').
//
// When a definition omits id/name they are derived from the file (or folder) name so
// re-seeding on restart is idempotent. Missing dirs are ignored; unreadable or invalid
// entries are skipped and returned in errs.
func DiscoverScenarios(dirs []string) (chains []Chain, errs []error) {
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("scenario dir %q: %w", dir, err))
			}
			continue
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())

			// idBase seeds a stable id/name when the definition omits them: the folder
			// name for a bundle, the file name for a flat definition.
			var yamlPath, idBase string
			switch {
			case e.IsDir():
				yamlPath = scenarioYAMLInDir(path)
				if yamlPath == "" {
					continue // a plain folder, not a scenario bundle
				}
				idBase = e.Name()
			case isYAML(e.Name()):
				yamlPath = path
				idBase = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			default:
				continue
			}

			ch, err := loadScenario(yamlPath, idBase)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", yamlPath, err))
				continue
			}
			chains = append(chains, ch)
		}
	}
	return chains, errs
}

func isYAML(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".yaml", ".yml":
		return true
	}
	return false
}

// scenarioYAMLInDir returns the definition file inside a scenario folder, or "".
// Conventional names win; otherwise a lone *.yaml/*.yml is used. A folder with
// several unconventionally-named YAMLs is ambiguous and skipped.
func scenarioYAMLInDir(dir string) string {
	for _, name := range scenarioYAMLNames {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var found string
	for _, e := range entries {
		if e.IsDir() || !isYAML(e.Name()) {
			continue
		}
		if found != "" {
			return "" // ambiguous
		}
		found = filepath.Join(dir, e.Name())
	}
	return found
}

func loadScenario(path, idBase string) (Chain, error) {
	var ch Chain
	raw, err := os.ReadFile(path)
	if err != nil {
		return ch, err
	}

	absDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		absDir = filepath.Dir(path)
	}
	raw = []byte(strings.ReplaceAll(string(raw), "{{scenario_dir}}", absDir))

	if err := yaml.Unmarshal(raw, &ch); err != nil {
		return ch, fmt.Errorf("invalid YAML: %w", err)
	}

	// Derive stable id/name from idBase so restarts upsert the same record.
	if ch.ID == "" {
		ch.ID = slug(idBase)
	}
	if ch.Name == "" {
		ch.Name = idBase
	}
	return ch, nil
}

// slug turns a file/folder name into a stable, URL-friendly id.
func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
