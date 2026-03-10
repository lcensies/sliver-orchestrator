// Package chain provides the data model and execution engine for attack scenario chains.
// A Chain is a DAG of Steps; each Step describes an action to execute on a remote host
// via a Sliver session.  Steps can depend on each other, pass output between each other
// via named variables, and conditionally skip based on prior results.
package chain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ActionType identifies what kind of action a Step performs.
type ActionType string

const (
	// ActionCommand runs a raw command on the remote host using the specified interpreter.
	ActionCommand ActionType = "command"
	// ActionAtomic resolves and runs a technique from the atomic library.
	ActionAtomic ActionType = "atomic"
	// ActionUpload transfers a local file (by path on the C2 server) to the remote host.
	ActionUpload ActionType = "upload"
	// ActionBinary fetches a binary from an embedded payload or a URL, uploads it to the
	// remote host via Sliver, executes it, and optionally removes it afterwards.
	ActionBinary ActionType = "binary"
	// ActionProbe interrogates the victim's environment (OS, kernel, installed software)
	// and optionally gates further execution on a regex match against the result.
	ActionProbe ActionType = "probe"
	// ActionPython runs a Python script on the C2 server.  The script receives
	// SLIVER_CONFIG and SESSION_ID env vars so it can use sliver-py for direct
	// Sliver interaction.  stdout/stderr/exit-code are treated as step output.
	ActionPython ActionType = "python"
	// ActionSliverRPC dispatches a named Sliver gRPC call with JSON parameters.
	ActionSliverRPC ActionType = "sliver_rpc"
)

// FailPolicy controls executor behaviour when a step returns a non-zero exit code or error.
type FailPolicy string

const (
	// FailAbort stops the entire chain immediately.
	FailAbort FailPolicy = "abort"
	// FailContinue ignores the failure and continues with other steps; the step is still
	// counted as failed, so the chain reports failure at the end if any step failed.
	FailContinue FailPolicy = "continue"
	// FailContinueOK (YAML: continue_no_err) continues execution and does not count this
	// step's failure toward the final chain outcome. Use for optional/non-critical steps
	// so the chain can still complete successfully when they fail.
	FailContinueOK FailPolicy = "continue_no_err"
	// FailSkipDependents marks all steps that (transitively) depend on this step as skipped.
	FailSkipDependents FailPolicy = "skip_dependents"
)

// Chain is the top-level scenario definition.  Its Steps form a directed acyclic graph
// via the DependsOn field; the executor resolves them in topological order.
type Chain struct {
	ID           string   `json:"id"            yaml:"id"`
	Name         string   `json:"name"          yaml:"name"`
	Description  string   `json:"description"   yaml:"description"`
	MITRETactics []string `json:"mitre_tactics" yaml:"mitre_tactics"`
	Tags         []string `json:"tags"          yaml:"tags"`
	Steps        []Step   `json:"steps"         yaml:"steps"`
}

// Dep is a single entry in a Step's DependsOn list.
// It is either a plain step ID string or an operator group:
//
//	"step_id"                     — that one step must settle
//	{any: [id1, id2, ...]}        — at least one member must complete (unblocks eagerly)
//	{all: [id1, id2, ...]}        — all members must settle (same as listing them flat)
//
// YAML examples:
//
//	depends_on:
//	  - sysinfo
//	  - {any: [cron_persistence, persist_bashrc, persist_profile_d]}
type Dep struct {
	ID  string   // non-empty for a plain step ID
	Any []string // non-empty for {any: [...]} groups
	All []string // non-empty for {all: [...]} groups
}

// AllIDs returns every step ID referenced by this entry.
func (d Dep) AllIDs() []string {
	if d.ID != "" {
		return []string{d.ID}
	}
	ids := make([]string, 0, len(d.Any)+len(d.All))
	ids = append(ids, d.Any...)
	ids = append(ids, d.All...)
	return ids
}

// UnmarshalYAML supports both scalar ("step_id") and map ({any:[...]} / {all:[...]}) forms.
func (d *Dep) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		d.ID = value.Value
		return nil
	case yaml.MappingNode:
		var m struct {
			Any []string `yaml:"any"`
			All []string `yaml:"all"`
		}
		if err := value.Decode(&m); err != nil {
			return fmt.Errorf("depends_on operator map: %w", err)
		}
		if len(m.Any) == 0 && len(m.All) == 0 {
			return fmt.Errorf("depends_on operator map requires 'any' or 'all' key with at least one step ID")
		}
		d.Any = m.Any
		d.All = m.All
		return nil
	}
	return fmt.Errorf("depends_on entry must be a string ID or an operator map {any:[...]} / {all:[...]}")
}

// MarshalJSON serialises plain IDs as JSON strings and operator groups as objects.
func (d Dep) MarshalJSON() ([]byte, error) {
	if d.ID != "" {
		return json.Marshal(d.ID)
	}
	type wireOp struct {
		Any []string `json:"any,omitempty"`
		All []string `json:"all,omitempty"`
	}
	return json.Marshal(wireOp{Any: d.Any, All: d.All})
}

// UnmarshalJSON accepts both plain JSON strings and operator-map objects.
func (d *Dep) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		d.ID = s
		return nil
	}
	type wireOp struct {
		Any []string `json:"any"`
		All []string `json:"all"`
	}
	var op wireOp
	if err := json.Unmarshal(b, &op); err != nil {
		return fmt.Errorf("depends_on entry must be a string or {any:[...]} / {all:[...]} map: %w", err)
	}
	d.Any = op.Any
	d.All = op.All
	return nil
}

// Step is one node in the chain graph.
type Step struct {
	ID         string      `json:"id"         yaml:"id"`
	Name       string      `json:"name"       yaml:"name"`
	DependsOn  []Dep       `json:"depends_on" yaml:"depends_on"`
	Conditions []Condition `json:"conditions" yaml:"conditions"`
	Action     Action      `json:"action"     yaml:"action"`

	// OutputVar captures the stdout of this step into a named variable that
	// later steps can reference with {{VarName}} syntax.
	// If OutputFilter is also set, the extracted capture group is stored instead.
	OutputVar    string        `json:"output_var"    yaml:"output_var"`
	OutputFilter *OutputFilter `json:"output_filter" yaml:"output_filter"`

	// OutputExtract extracts multiple named variables from stdout in a single step.
	// Each entry applies an independent regex and stores the capture group result.
	// These are populated in addition to (and independently of) OutputVar.
	OutputExtract []OutputCapture `json:"output_extract" yaml:"output_extract"`

	Timeout string     `json:"timeout" yaml:"timeout"` // e.g. "30s", "5m"
	OnFail  FailPolicy `json:"on_fail" yaml:"on_fail"`
}

// OutputFilter selects a specific part of stdout using a Go regex capture group.
// When set alongside OutputVar, the capture group is stored rather than the full stdout.
type OutputFilter struct {
	// Regex is a Go regular expression containing at least one capture group.
	Regex string `json:"regex" yaml:"regex"`
	// Group is the capture group index to extract (1-based, default 1).
	Group int `json:"group" yaml:"group"`
}

// OutputCapture extracts a named variable from stdout using a regex capture group.
// Used in Step.OutputExtract to pull multiple values from one step's output.
type OutputCapture struct {
	// Var is the variable name, referenceable as {{Var}} in later steps.
	Var string `json:"var" yaml:"var"`
	// Regex is a Go regular expression containing at least one capture group.
	Regex string `json:"regex" yaml:"regex"`
	// Group is the capture group index to extract (1-based, default 1).
	Group int `json:"group" yaml:"group"`
}

// TimeoutDuration parses Timeout; returns 60 s if empty or unparseable.
func (s *Step) TimeoutDuration() time.Duration {
	if s.Timeout == "" {
		return 60 * time.Second
	}
	d, err := time.ParseDuration(s.Timeout)
	if err != nil {
		return 60 * time.Second
	}
	return d
}

// FailurePolicy returns the effective fail policy (default: FailContinue).
func (s *Step) FailurePolicy() FailPolicy {
	if s.OnFail == "" {
		return FailContinue
	}
	return s.OnFail
}

// AllDepIDs returns every step ID referenced anywhere in DependsOn (flattened).
// Used for cycle detection and graph validation.
func (s *Step) AllDepIDs() []string {
	var ids []string
	for _, dep := range s.DependsOn {
		ids = append(ids, dep.AllIDs()...)
	}
	return ids
}

// Action describes what a step does.  Exactly one of the pointer fields should be non-nil.
type Action struct {
	Type      ActionType     `json:"type"       yaml:"type"`
	Command   *CommandAction `json:"command"    yaml:"command"`
	AtomicRef *AtomicRef     `json:"atomic_ref" yaml:"atomic_ref"`
	Upload    *UploadAction  `json:"upload"     yaml:"upload"`
	Binary    *BinaryAction  `json:"binary"     yaml:"binary"`
	Probe     *ProbeAction   `json:"probe"      yaml:"probe"`
	Python    *PythonAction  `json:"python"     yaml:"python"`
	RPCAction *RPCAction     `json:"sliver_rpc" yaml:"sliver_rpc"`
}

// CommandAction executes a raw command string using the named interpreter.
// Interpreter values: "sh" (default), "bash", "powershell", "cmd".
type CommandAction struct {
	Interpreter string `json:"interpreter" yaml:"interpreter"`
	Cmd         string `json:"cmd"         yaml:"cmd"`
}

// AtomicRef references a specific test within the atomic technique library.
// The test is located using the first non-empty/non-negative field in priority order:
// GUID > Name > Test index.
type AtomicRef struct {
	ID   string            `json:"id"   yaml:"id"`   // e.g. "T1059.001"
	Test int               `json:"test" yaml:"test"` // zero-based index; ignored when Name or GUID is set
	Name string            `json:"name" yaml:"name"` // exact test name (alternative to index)
	GUID string            `json:"guid" yaml:"guid"` // auto_generated_guid (highest priority)
	Args map[string]string `json:"args" yaml:"args"` // override input_arguments defaults
}

// UploadAction copies a local file (by path on the C2 server) to the remote host.
type UploadAction struct {
	LocalPath  string `json:"local_path"  yaml:"local_path"`
	RemotePath string `json:"remote_path" yaml:"remote_path"`
	Execute    bool   `json:"execute"     yaml:"execute"`
}

// BinaryAction fetches a binary — either from an inline base64-encoded payload or by
// downloading it from a URL on the C2 server — then uploads it to the victim via Sliver,
// executes it, and optionally removes it.
//
// Exactly one of Data or URL must be set.
type BinaryAction struct {
	// Data is the base64-encoded binary payload embedded directly in the chain definition.
	Data string `json:"data" yaml:"data"`
	// URL is a URL the C2 server downloads the binary from before uploading to the victim.
	URL string `json:"url" yaml:"url"`

	// RemotePath is the destination path on the victim.
	// If empty, a temp path is generated automatically (/tmp/scn_<rand> on Linux,
	// C:\Windows\Temp\scn_<rand>.exe on Windows).
	RemotePath string `json:"remote_path" yaml:"remote_path"`

	// Args are appended to the remote_path when building the execution command.
	Args string `json:"args" yaml:"args"`

	// Platform controls execution and cleanup commands: "linux" (default) or "windows".
	// On linux: chmod +x before running, rm -f on cleanup.
	// On windows: execute directly, del on cleanup.
	Platform string `json:"platform" yaml:"platform"`

	// Cleanup removes the uploaded binary from the victim after execution.
	Cleanup bool `json:"cleanup" yaml:"cleanup"`
}

// RPCAction calls a Sliver RPC method by name with a free-form parameter map.
type RPCAction struct {
	Method string            `json:"method" yaml:"method"`
	Params map[string]string `json:"params" yaml:"params"`
}

// ProbeAction interrogates the victim's environment and optionally validates the result
// against a regex pattern.  The raw detected value is always emitted as stdout so it
// can be captured with output_var / output_filter / output_extract.
//
// When Match is set: exit 0 on match, exit 1 on mismatch (triggers on_fail policy).
// When Match is empty: always exits 0 — useful purely for discovery / capture.
type ProbeAction struct {
	// Kind is the aspect of the environment to interrogate.
	// Supported values: "os", "kernel", "arch", "software_exists", "software_version"
	Kind string `json:"kind" yaml:"kind"`

	// Software is the program name used by software_exists and software_version probes.
	Software string `json:"software" yaml:"software"`

	// Match is a Go regular expression validated against the probe's stdout.
	// Supports {{VarName}} substitution so match patterns can be dynamic.
	Match string `json:"match" yaml:"match"`

	// Platform is the victim's OS family used to select the correct detection command.
	// Values: "linux" (default), "windows", "darwin".
	Platform string `json:"platform" yaml:"platform"`
}

// PythonAction runs a Python script on the C2 server (not on the victim).
// Scripts can use sliver-py (https://github.com/sliverarmory/sliver-py) for full
// Sliver interaction, or any other Python library available on the C2 host.
//
// Built-in environment variables always injected:
//   - SLIVER_CONFIG — path to the Sliver operator .cfg file
//   - SESSION_ID    — the current target session ID
//
// Additional context variables can be forwarded explicitly via the Env map using
// {{VarName}} substitution in values, e.g. NTLM_HASH: "{{ntlm_hash}}".
type PythonAction struct {
	// Script is the path to a .py file on the C2 server filesystem.
	Script string `json:"script" yaml:"script"`
	// Inline is an inline Python script (alternative to Script).
	// The content is written to a temp file before execution.
	Inline string `json:"inline" yaml:"inline"`
	// Args are additional command-line arguments appended after the script path.
	// {{VarName}} substitution applies.
	Args []string `json:"args" yaml:"args"`
	// Env contains extra environment variables for the script process.
	// Values support {{VarName}} substitution.
	Env map[string]string `json:"env" yaml:"env"`
}

// Condition gates step execution.  The step is skipped (not failed) when any condition
// evaluates to false.
//
// Conditions support two YAML forms:
//   - Explicit: var, op, value (and optional negate).
//   - Sigma-style: a single key "var|op" with the comparison value, e.g. victim_os|contains: Linux
type Condition struct {
	// Var is the name of a previously captured output variable, or the special value "exit_code".
	Var   string `json:"var"    yaml:"var"`
	// Op is the comparison operator: "eq", "neq", "contains", "matches", "gt", "lt".
	Op    string `json:"op"     yaml:"op"`
	Value string `json:"value"  yaml:"value"`
	// Negate inverts the result of the comparison.
	Negate bool `json:"negate" yaml:"negate"`
}

// UnmarshalYAML supports both explicit (var/op/value) and sigma-style (var|op: value) condition syntax.
// Sigma-style can appear as a mapping key (victim_os|contains: Linux) or, when the parser
// emits a scalar, as the string "var|op: value" which we parse manually.
func (c *Condition) UnmarshalYAML(value *yaml.Node) error {
	// Sigma-style as scalar string (some YAML parsers represent the list item as a string)
	if value.Kind == yaml.ScalarNode && strings.Contains(value.Value, "|") {
		if varName, op, val, ok := parseSigmaCondition(value.Value); ok {
			c.Var = varName
			c.Op = op
			c.Value = val
			return nil
		}
	}
	if value.Kind != yaml.MappingNode {
		return value.Decode(c)
	}
	// Look for sigma-style key (var|op)
	for i := 0; i+1 < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && strings.Contains(keyNode.Value, "|") {
			parts := strings.SplitN(keyNode.Value, "|", 2)
			c.Var = strings.TrimSpace(parts[0])
			c.Op = strings.TrimSpace(parts[1])
			if valNode.Kind == yaml.ScalarNode {
				c.Value = strings.TrimSpace(valNode.Value)
			} else {
				var s string
				if err := valNode.Decode(&s); err == nil {
					c.Value = strings.TrimSpace(s)
				}
			}
			for j := 0; j+1 < len(value.Content); j += 2 {
				if value.Content[j].Value == "negate" {
					_ = value.Content[j+1].Decode(&c.Negate)
					break
				}
			}
			return nil
		}
	}
	var raw struct {
		Var    string `yaml:"var"`
		Op     string `yaml:"op"`
		Value  string `yaml:"value"`
		Negate bool   `yaml:"negate"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	c.Var = raw.Var
	c.Op = raw.Op
	c.Value = raw.Value
	c.Negate = raw.Negate
	return nil
}

// parseSigmaCondition parses "var|op: value" or "var|op: value" with optional leading/trailing space.
// Returns (var, op, value, true) on success.
func parseSigmaCondition(s string) (varName, op, val string, ok bool) {
	s = strings.TrimSpace(s)
	pipe := strings.Index(s, "|")
	if pipe <= 0 || pipe >= len(s)-1 {
		return "", "", "", false
	}
	varName = strings.TrimSpace(s[:pipe])
	rest := strings.TrimSpace(s[pipe+1:])
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return "", "", "", false
	}
	op = strings.TrimSpace(rest[:colon])
	val = strings.TrimSpace(rest[colon+1:])
	if varName == "" || op == "" {
		return "", "", "", false
	}
	return varName, op, val, true
}
