package chain

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSigmaConditionUnmarshalMapping ensures "var|op: value" mapping nodes decode correctly.
func TestSigmaConditionUnmarshalMapping(t *testing.T) {
	type stepConditions struct {
		Conditions []Condition `yaml:"conditions"`
	}
	input := `
conditions:
  - victim_os|contains: Linux
  - kernel_ver|matches: "5\\.[0-9]+"
`
	var s stepConditions
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Conditions) != 2 {
		t.Fatalf("want 2 conditions, got %d", len(s.Conditions))
	}
	cases := []struct{ wantVar, wantOp, wantVal string }{
		{"victim_os", "contains", "Linux"},
		{"kernel_ver", "matches", `5\.[0-9]+`},
	}
	for i, c := range cases {
		got := s.Conditions[i]
		if got.Var != c.wantVar || got.Op != c.wantOp || got.Value != c.wantVal {
			t.Errorf("condition[%d]: got {%q %q %q}, want {%q %q %q}",
				i, got.Var, got.Op, got.Value, c.wantVar, c.wantOp, c.wantVal)
		}
	}
}

// TestSigmaConditionEvalPass verifies a parsed sigma condition evaluates correctly against vars.
func TestSigmaConditionEvalPass(t *testing.T) {
	c := Condition{Var: "victim_os", Op: "contains", Value: "Linux"}
	vars := VarMap{"victim_os": "Linux"}
	ok, err := EvalCondition(c, vars, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected condition to pass")
	}
}

// TestSigmaConditionMissingVar reproduces the original bug: variable not in scope → skip.
func TestSigmaConditionMissingVar(t *testing.T) {
	c := Condition{Var: "victim_os", Op: "contains", Value: "Linux"}
	// Empty vars — variable not captured yet
	_, err := EvalCondition(c, VarMap{}, 0)
	if err == nil {
		t.Fatal("expected error for missing variable")
	}
}

// TestExplicitConditionUnmarshal ensures the verbose var/op/value form still works.
func TestExplicitConditionUnmarshal(t *testing.T) {
	input := `
conditions:
  - var: victim_os
    op: contains
    value: Linux
`
	type stepConditions struct {
		Conditions []Condition `yaml:"conditions"`
	}
	var s stepConditions
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Conditions) != 1 {
		t.Fatalf("want 1 condition, got %d", len(s.Conditions))
	}
	got := s.Conditions[0]
	if got.Var != "victim_os" || got.Op != "contains" || got.Value != "Linux" {
		t.Errorf("got {%q %q %q}", got.Var, got.Op, got.Value)
	}
}

// TestParseSigmaCondition covers the scalar-string path added for edge-case parsers.
func TestParseSigmaCondition(t *testing.T) {
	cases := []struct {
		input            string
		wantVar, wantOp, wantVal string
		wantOK           bool
	}{
		{"victim_os|contains: Linux", "victim_os", "contains", "Linux", true},
		{"kernel_ver|matches: 5\\.[0-9]+", "kernel_ver", "matches", `5\.[0-9]+`, true},
		{"no_pipe_here", "", "", "", false},
		{"|op: val", "", "", "", false},
		{"var|: val", "", "", "", false},
	}
	for _, tc := range cases {
		v, op, val, ok := parseSigmaCondition(tc.input)
		if ok != tc.wantOK {
			t.Errorf("parseSigmaCondition(%q): ok=%v want %v", tc.input, ok, tc.wantOK)
			continue
		}
		if ok && (v != tc.wantVar || op != tc.wantOp || val != tc.wantVal) {
			t.Errorf("parseSigmaCondition(%q): got {%q %q %q}, want {%q %q %q}",
				tc.input, v, op, val, tc.wantVar, tc.wantOp, tc.wantVal)
		}
	}
}
