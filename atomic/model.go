// Package atomic provides the MITRE ATT&CK technique library.
// The format is compatible with Atomic Red Team (https://github.com/redcanaryco/atomic-red-team)
// so existing ART YAML files can be dropped in without modification.
//
// Types are aligned with the GoART project (github.com/lcensies/go-atomicredteam)
// and the upstream ART spec (atomic_red_team/spec.yaml).
package atomic

// Technique represents a MITRE ATT&CK technique with one or more executable atomic tests.
type Technique struct {
	// ID is the ATT&CK technique identifier, e.g. "T1059.001".
	ID          string `yaml:"attack_technique"`
	DisplayName string `yaml:"display_name"`

	// Tactic is a custom field (not in standard ART) for quick filtering.
	// Values match ATT&CK tactic names: initial-access, execution, persistence, etc.
	Tactic string `yaml:"tactic"`

	// Platforms lists target OSes at the technique level for quick filtering.
	Platforms []string `yaml:"platforms"`

	Tests []Test `yaml:"atomic_tests"`
}

// Test is a single executable test within a technique.
type Test struct {
	Name        string `yaml:"name"`
	GUID        string `yaml:"auto_generated_guid,omitempty"`
	Description string `yaml:"description,omitempty"`

	SupportedPlatforms []string `yaml:"supported_platforms"`

	// InputArguments describes the parameters the test accepts.
	InputArguments map[string]InputArg `yaml:"input_arguments,omitempty"`

	// DependencyExecutorName is the executor used to check/install prerequisites.
	DependencyExecutorName string       `yaml:"dependency_executor_name,omitempty"`
	Dependencies           []Dependency `yaml:"dependencies,omitempty"`

	// Executor specifies how the test command is run.
	Executor *TestExecutor `yaml:"executor"`

	// CleanupCommand (top-level ART field, mirrors Executor.CleanupCommand).
	CleanupCommand string `yaml:"cleanup_command,omitempty"`
}

// InputArg describes one input parameter for a test.
type InputArg struct {
	Description string `yaml:"description"`
	// Type is one of: string, path, url, integer, float.
	Type    string `yaml:"type"`
	Default string `yaml:"default"`
}

// Dependency is a prerequisite that must be satisfied before running the test.
type Dependency struct {
	Description      string `yaml:"description"`
	PrereqCommand    string `yaml:"prereq_command,omitempty"`
	GetPrereqCommand string `yaml:"get_prereq_command,omitempty"`
}

// TestExecutor specifies the runtime that executes the test command.
type TestExecutor struct {
	// Name is one of: sh, bash, command_prompt, powershell, manual.
	Name              string `yaml:"name"`
	ElevationRequired bool   `yaml:"elevation_required"`
	// Command is the shell command(s) to execute (all executors except manual).
	Command string `yaml:"command,omitempty"`
	// Steps is used by the "manual" executor in lieu of Command.
	Steps         string `yaml:"steps,omitempty"`
	CleanupCommand string `yaml:"cleanup_command,omitempty"`
}
