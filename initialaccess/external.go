package initialaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ExternalModule is the built-in initial-access module that runs an arbitrary
// external command/script — the escape hatch that makes any tool pluggable
// (msfconsole -r <rc>, a python exploit, a shell one-liner) without recompiling.
//
// Contract (config keys):
//
//	run    (required) JSON array of argv, e.g. ["python3","/opt/exploits/web_rce.py"].
//	       A single string is also accepted and split on whitespace.
//	shell  (optional) if set truthy and run is empty, the value is executed via
//	       `sh -c <shell>`.
//
// The full Request (target + config) is marshaled to JSON and written to the
// child's stdin so the exploit can read its parameters. The child SHOULD print a
// JSON Result ({"ok":true,"note":"...","hostname":"..."}) on stdout; if stdout is
// not valid JSON, Ok is inferred from the process exit code (0 => Ok).
type ExternalModule struct{}

// Name implements Module.
func (m *ExternalModule) Name() string { return "external" }

// Run implements Module.
func (m *ExternalModule) Run(ctx context.Context, req Request) (Result, error) {
	argv, err := parseArgv(req.Config)
	if err != nil {
		return Result{}, err
	}

	input, err := json.Marshal(req)
	if err != nil {
		return Result{}, fmt.Errorf("marshaling module request: %w", err)
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// Failed to start (binary missing, etc.) — a real transport error.
			return Result{}, fmt.Errorf("running external module %v: %w (stderr: %s)", argv, runErr, strings.TrimSpace(stderr.String()))
		}
	}

	// Prefer a JSON Result on stdout; fall back to exit-code inference.
	res, parsed := parseResult(stdout.Bytes())
	if !parsed {
		res.Ok = exitCode == 0
		note := strings.TrimSpace(stdout.String())
		if note == "" {
			note = strings.TrimSpace(stderr.String())
		}
		res.Note = note
	}
	if !res.Ok && res.Note == "" {
		res.Note = fmt.Sprintf("external module exited %d", exitCode)
	}
	return res, nil
}

// parseArgv extracts the command to run from the module config.
func parseArgv(cfg map[string]string) ([]string, error) {
	if raw, ok := cfg["run"]; ok && strings.TrimSpace(raw) != "" {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "[") {
			var argv []string
			if err := json.Unmarshal([]byte(trimmed), &argv); err != nil {
				return nil, fmt.Errorf("parsing 'run' as JSON array: %w", err)
			}
			if len(argv) == 0 {
				return nil, errors.New("'run' array is empty")
			}
			return argv, nil
		}
		return strings.Fields(trimmed), nil
	}
	if sh := strings.TrimSpace(cfg["shell"]); sh != "" {
		return []string{"sh", "-c", sh}, nil
	}
	return nil, errors.New("external module requires a 'run' (argv) or 'shell' config key")
}

// parseResult attempts to decode a JSON Result from stdout. It scans for the last
// non-empty line that parses as a JSON object, so exploit scripts may emit
// progress lines before the final result.
func parseResult(stdout []byte) (Result, bool) {
	lines := strings.Split(string(stdout), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var r Result
		if err := json.Unmarshal([]byte(line), &r); err == nil {
			return r, true
		}
	}
	return Result{}, false
}
