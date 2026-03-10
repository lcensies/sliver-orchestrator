package atomic

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// pathToAtomicsRE matches the ART PathToAtomicsFolder placeholder including
// any trailing slash/backslash, e.g. "PathToAtomicsFolder/", "PathToAtomicsFolder\".
// GoART uses the same pattern: regexp.MustCompile(`PathToAtomicsFolder(\\|\/)`)
var pathToAtomicsRE = regexp.MustCompile(`PathToAtomicsFolder[/\\]?`)

// Library is an in-memory index of MITRE ATT&CK techniques loaded from YAML files.
type Library struct {
	techniques map[string]*Technique // keyed by technique ID, e.g. "T1059.001"
	baseDir    string                // atomics directory, used for PathToAtomicsFolder substitution
}

// NewLibrary creates an empty Library.
func NewLibrary() *Library {
	return &Library{techniques: make(map[string]*Technique)}
}

// LoadDir reads ART technique YAML files from dir and any nested subdirectories.
// Both .yaml and .yml files are supported. The directory path is recorded as the
// base for PathToAtomicsFolder substitution.
func (lib *Library) LoadDir(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving atomics dir %q: %w", dir, err)
	}
	lib.baseDir = abs

	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("reading atomics dir %q: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("atomics path %q is not a directory", abs)
	}

	var files []string
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !isAtomicYAML(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("reading atomics dir %q: %w", abs, err)
	}

	sort.Strings(files)
	for _, path := range files {
		if err := lib.LoadFile(path); err != nil {
			rel, relErr := filepath.Rel(abs, path)
			if relErr != nil {
				rel = path
			}
			return fmt.Errorf("loading %s: %w", rel, err)
		}
	}
	return nil
}

func isAtomicYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// LoadFile parses a single technique YAML file and indexes it.
func (lib *Library) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var t Technique
	if err := yaml.Unmarshal(data, &t); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	if t.ID == "" {
		return fmt.Errorf("%s: attack_technique field is empty", path)
	}
	lib.techniques[t.ID] = &t
	return nil
}

// Get returns the technique for the given ID, e.g. "T1059.001".
func (lib *Library) Get(id string) (*Technique, bool) {
	t, ok := lib.techniques[id]
	return t, ok
}

// All returns all loaded techniques (order not guaranteed).
func (lib *Library) All() []*Technique {
	out := make([]*Technique, 0, len(lib.techniques))
	for _, t := range lib.techniques {
		out = append(out, t)
	}
	return out
}

// Filter returns techniques matching tactic and/or platform.
// An empty string for either parameter means "match any".
func (lib *Library) Filter(tactic, platform string) []*Technique {
	var out []*Technique
	for _, t := range lib.techniques {
		if tactic != "" && !strings.EqualFold(t.Tactic, tactic) {
			continue
		}
		if platform != "" {
			found := false
			for _, p := range t.Platforms {
				if strings.EqualFold(p, platform) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// Resolve returns the interpreter name and fully substituted command for a specific test.
// The test can be located by zero-based index, name, or GUID — whichever is non-empty/non-negative.
// Priority: GUID > name > index.
// Args in the map override input_arguments defaults.
// Implements the chain.AtomicResolver interface.
func (lib *Library) Resolve(techniqueID, name, guid string, testIdx int, args map[string]string) (interpreter, command string, err error) {
	t, ok := lib.Get(techniqueID)
	if !ok {
		return "", "", fmt.Errorf("technique %q not found in library", techniqueID)
	}

	test, err := lib.findTest(t, name, guid, testIdx)
	if err != nil {
		return "", "", err
	}

	if test.Executor == nil {
		return "", "", fmt.Errorf("test %q has no executor", test.Name)
	}
	if test.Executor.Name == "manual" {
		return "", "", fmt.Errorf("test %q requires manual execution (no automated executor)", test.Name)
	}

	// Build effective argument map: defaults first, then caller overrides.
	// Mirrors GoART checkArgsAndGetDefaults.
	effective, err := lib.resolveArgs(test, args)
	if err != nil {
		return "", "", fmt.Errorf("resolving args for %s/%q: %w", techniqueID, test.Name, err)
	}

	// Interpolate #{arg} placeholders and PathToAtomicsFolder.
	// Mirrors GoART interpolateWithArgs.
	cmd := lib.interpolate(test.Executor.Command, effective)

	// Normalise interpreter names to what the Sliver executor understands.
	interp := normaliseInterpreter(test.Executor.Name)

	return interp, cmd, nil
}

// findTest locates a test within a technique by GUID, name, or index — in that priority order.
// Mirrors GoART getTest logic.
func (lib *Library) findTest(t *Technique, name, guid string, idx int) (*Test, error) {
	if guid != "" {
		for i := range t.Tests {
			if t.Tests[i].GUID == guid {
				return &t.Tests[i], nil
			}
		}
		return nil, fmt.Errorf("technique %s: no test with GUID %q", t.ID, guid)
	}

	if name != "" {
		for i := range t.Tests {
			if t.Tests[i].Name == name {
				return &t.Tests[i], nil
			}
		}
		return nil, fmt.Errorf("technique %s: no test named %q", t.ID, name)
	}

	if idx < 0 || idx >= len(t.Tests) {
		return nil, fmt.Errorf("technique %s: test index %d out of range (has %d test(s))", t.ID, idx, len(t.Tests))
	}
	return &t.Tests[idx], nil
}

// resolveArgs builds the effective argument map for a test.
// Defaults from the test definition are filled in first; caller-supplied args override.
// Mirrors GoART checkArgsAndGetDefaults.
func (lib *Library) resolveArgs(test *Test, overrides map[string]string) (map[string]string, error) {
	effective := make(map[string]string, len(test.InputArguments))
	for k, a := range test.InputArguments {
		if a.Default != "" {
			effective[k] = a.Default
		}
	}
	for k, v := range overrides {
		effective[k] = v
	}
	// Verify required args (no default and not supplied) are present.
	for k, a := range test.InputArguments {
		if _, ok := effective[k]; !ok && a.Default == "" {
			return nil, fmt.Errorf("required argument %q has no default and was not supplied", k)
		}
	}
	return effective, nil
}

// interpolate substitutes #{arg} placeholders and replaces the ART
// PathToAtomicsFolder token with the library's base directory.
// Mirrors GoART interpolateWithArgs.
func (lib *Library) interpolate(cmd string, args map[string]string) string {
	// Replace PathToAtomicsFolder with the actual directory path.
	// GoART replaces it with the base dir (local or bundled include path).
	base := lib.baseDir
	if base == "" {
		base = "."
	}
	cmd = pathToAtomicsRE.ReplaceAllString(cmd, base+string(filepath.Separator))

	// Replace #{arg_name} placeholders (ART convention).
	for k, v := range args {
		cmd = strings.ReplaceAll(cmd, "#{"+k+"}", v)
	}

	return strings.TrimSpace(cmd)
}

// normaliseInterpreter maps ART executor names to the values the Sliver executor understands.
// Mirrors the switch in GoART's executor dispatch logic.
func normaliseInterpreter(name string) string {
	switch strings.ToLower(name) {
	case "command_prompt":
		return "cmd"
	case "powershell":
		return "powershell"
	case "bash":
		return "bash"
	default: // "sh", ""
		return "sh"
	}
}

// Count returns the number of loaded techniques.
func (lib *Library) Count() int {
	return len(lib.techniques)
}
