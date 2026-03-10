package atomic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDirSupportsAtomicRedTeamLayout(t *testing.T) {
	root := t.TempDir()

	writeTechniqueFile(t, filepath.Join(root, "T1082", "T1082.yaml"), `
attack_technique: T1082
display_name: System Information Discovery
atomic_tests:
  - name: Collect system info
    supported_platforms: [linux]
    executor:
      name: sh
      command: echo PathToAtomicsFolder/T1082/src
`)

	writeTechniqueFile(t, filepath.Join(root, "T1016", "T1016.yml"), `
attack_technique: T1016
display_name: System Network Configuration Discovery
atomic_tests:
  - name: Show interfaces
    supported_platforms: [linux]
    executor:
      name: sh
      command: ip addr
`)

	lib := NewLibrary()
	if err := lib.LoadDir(root); err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	if got := lib.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}

	if _, ok := lib.Get("T1082"); !ok {
		t.Fatal("expected T1082 to be loaded from nested ART path")
	}
	if _, ok := lib.Get("T1016"); !ok {
		t.Fatal("expected T1016 to be loaded from nested .yml path")
	}

	_, cmd, err := lib.Resolve("T1082", "", "", 0, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := "echo " + filepath.Join(root, "T1082", "src")
	if cmd != want {
		t.Fatalf("Resolve() command = %q, want %q", cmd, want)
	}
}

func writeTechniqueFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
