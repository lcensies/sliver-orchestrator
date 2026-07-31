package chain

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiscoverScenarios covers the three discovery shapes plus {{scenario_dir}}
// substitution and id/name derivation.
func TestDiscoverScenarios(t *testing.T) {
	root := t.TempDir()

	// 1. Flat file with an explicit id.
	write(t, filepath.Join(root, "flat.yaml"), `id: flat-one
name: Flat One
steps:
  - id: s1
    action: { type: command, command: { cmd: "id" } }
`)

	// 2. Bundle folder with a conventional scenario.yaml + a resource; id derived
	//    from the FOLDER name; {{scenario_dir}} points at the folder.
	bundle := filepath.Join(root, "my-bundle")
	mkdir(t, bundle)
	write(t, filepath.Join(bundle, "exploit.py"), "print('x')\n")
	write(t, filepath.Join(bundle, "scenario.yaml"), `name: Bundled
steps:
  - id: s1
    action: { type: command, command: { cmd: "run {{scenario_dir}}/exploit.py" } }
`)

	// 3. Folder with a single, unconventionally-named yaml.
	single := filepath.Join(root, "single")
	mkdir(t, single)
	write(t, filepath.Join(single, "whatever.yml"), `name: Single
steps:
  - id: s1
    action: { type: command, command: { cmd: "id" } }
`)

	// Noise that must be ignored.
	write(t, filepath.Join(root, "notes.txt"), "ignore me")
	mkdir(t, filepath.Join(root, "empty-folder"))

	chains, errs := DiscoverScenarios([]string{root, "/does/not/exist"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	byID := map[string]Chain{}
	for _, c := range chains {
		byID[c.ID] = c
	}
	if len(byID) != 3 {
		t.Fatalf("expected 3 scenarios, got %d: %v", len(byID), keys(byID))
	}

	if c, ok := byID["flat-one"]; !ok || c.Name != "Flat One" {
		t.Errorf("flat: got %+v", c)
	}

	// Bundle: id from folder name, {{scenario_dir}} resolved to the bundle path.
	b, ok := byID["my-bundle"]
	if !ok {
		t.Fatalf("bundle not discovered; got %v", keys(byID))
	}
	if b.Name != "Bundled" {
		t.Errorf("bundle name: %q", b.Name)
	}
	want := bundle + "/exploit.py"
	if got := b.Steps[0].Action.Command.Cmd; !contains(got, want) {
		t.Errorf("{{scenario_dir}} not substituted: cmd=%q want substring %q", got, want)
	}

	if _, ok := byID["single"]; !ok {
		t.Errorf("single-yaml folder not discovered; got %v", keys(byID))
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
func keys(m map[string]Chain) []string {
	var k []string
	for id := range m {
		k = append(k, id)
	}
	return k
}
func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
