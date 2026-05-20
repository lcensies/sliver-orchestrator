package chain

import (
	"testing"
)

func steps(pairs ...string) []Step {
	var out []Step
	for i := 0; i < len(pairs); i += 2 {
		id := pairs[i]
		deps := pairs[i+1]
		s := Step{ID: id}
		if deps != "" {
			for _, d := range splitIDs(deps) {
				s.DependsOn = append(s.DependsOn, Dep{ID: d})
			}
		}
		out = append(out, s)
	}
	return out
}

func splitIDs(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func assertOrder(t *testing.T, order []Step, ids ...string) {
	t.Helper()
	if len(order) != len(ids) {
		t.Fatalf("len(order)=%d want %d", len(order), len(ids))
	}
	pos := make(map[string]int)
	for i, s := range order {
		pos[s.ID] = i
	}
	// Just check all requested IDs are present
	for _, id := range ids {
		if _, ok := pos[id]; !ok {
			t.Errorf("step %q missing from order", id)
		}
	}
}

func TestResolveLinearChain(t *testing.T) {
	ss := steps("a", "", "b", "a", "c", "b")
	order, err := Resolve(ss)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// a must come before b, b before c
	pos := make(map[string]int)
	for i, s := range order {
		pos[s.ID] = i
	}
	if pos["a"] >= pos["b"] || pos["b"] >= pos["c"] {
		t.Errorf("wrong order: a=%d b=%d c=%d", pos["a"], pos["b"], pos["c"])
	}
}

func TestResolveDiamond(t *testing.T) {
	ss := []Step{
		{ID: "a"},
		{ID: "b", DependsOn: []Dep{{ID: "a"}}},
		{ID: "c", DependsOn: []Dep{{ID: "a"}}},
		{ID: "d", DependsOn: []Dep{{ID: "b"}, {ID: "c"}}},
	}
	order, err := Resolve(ss)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	assertOrder(t, order, "a", "b", "c", "d")
}

func TestResolveParallelRoots(t *testing.T) {
	ss := []Step{{ID: "x"}, {ID: "y"}, {ID: "z"}}
	order, err := Resolve(ss)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("want 3, got %d", len(order))
	}
}

func TestResolveCycleDetected(t *testing.T) {
	ss := []Step{
		{ID: "a", DependsOn: []Dep{{ID: "b"}}},
		{ID: "b", DependsOn: []Dep{{ID: "a"}}},
	}
	if _, err := Resolve(ss); err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestResolveDuplicateID(t *testing.T) {
	ss := []Step{{ID: "a"}, {ID: "a"}}
	if _, err := Resolve(ss); err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
}

func TestResolveUnknownDep(t *testing.T) {
	ss := []Step{
		{ID: "a", DependsOn: []Dep{{ID: "nonexistent"}}},
	}
	if _, err := Resolve(ss); err == nil {
		t.Fatal("expected unknown dep error, got nil")
	}
}

func TestResolveEmptyID(t *testing.T) {
	ss := []Step{{ID: ""}}
	if _, err := Resolve(ss); err == nil {
		t.Fatal("expected empty ID error, got nil")
	}
}

func TestReadyStepsAnyGroup(t *testing.T) {
	// d depends on any of [b, c]
	ss := []Step{
		{ID: "a"},
		{ID: "b", DependsOn: []Dep{{ID: "a"}}},
		{ID: "c", DependsOn: []Dep{{ID: "a"}}},
		{ID: "d", DependsOn: []Dep{{Any: []string{"b", "c"}}}},
	}

	// b completed, c not yet settled → d should be ready (any is satisfied)
	completed := map[string]bool{"a": true, "b": true}
	failed := map[string]bool{}
	skipped := map[string]bool{}

	ready := ReadySteps(ss, completed, failed, skipped)
	found := false
	for _, id := range ready {
		if id == "d" {
			found = true
		}
	}
	if !found {
		t.Errorf("d should be ready when b completed (any group), ready=%v", ready)
	}
}

func TestReadyStepsAnyGroupHopeless(t *testing.T) {
	// d depends on any of [b, c]; both failed → d should be ready (hopeless gate opens)
	ss := []Step{
		{ID: "d", DependsOn: []Dep{{Any: []string{"b", "c"}}}},
	}
	completed := map[string]bool{}
	failed := map[string]bool{"b": true, "c": true}
	skipped := map[string]bool{}

	ready := ReadySteps(ss, completed, failed, skipped)
	found := false
	for _, id := range ready {
		if id == "d" {
			found = true
		}
	}
	if !found {
		t.Errorf("d should be ready when all any-members failed, ready=%v", ready)
	}
}

func TestReadyStepsAllGroup(t *testing.T) {
	ss := []Step{
		{ID: "d", DependsOn: []Dep{{All: []string{"b", "c"}}}},
	}

	// only b done → not ready
	ready := ReadySteps(ss,
		map[string]bool{"b": true},
		map[string]bool{},
		map[string]bool{},
	)
	for _, id := range ready {
		if id == "d" {
			t.Error("d should NOT be ready when only b settled in all-group")
		}
	}

	// both settled → ready
	ready = ReadySteps(ss,
		map[string]bool{"b": true},
		map[string]bool{"c": true},
		map[string]bool{},
	)
	found := false
	for _, id := range ready {
		if id == "d" {
			found = true
		}
	}
	if !found {
		t.Errorf("d should be ready when all members settled, ready=%v", ready)
	}
}
