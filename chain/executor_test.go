package chain_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bishopfox/sliver/scenario/chain"
	"github.com/bishopfox/sliver/scenario/store"
)

// fakeStepExecutor is a configurable in-process StepExecutor.
type fakeStepExecutor struct {
	mu      sync.Mutex
	results map[string]fakeResult // keyed by action cmd or step fallback
	calls   []string              // recorded step IDs in call order
}

type fakeResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
	delay    time.Duration
}

func newFakeExec() *fakeStepExecutor {
	return &fakeStepExecutor{results: make(map[string]fakeResult)}
}

func (f *fakeStepExecutor) set(cmd string, r fakeResult) {
	f.mu.Lock()
	f.results[cmd] = r
	f.mu.Unlock()
}

func (f *fakeStepExecutor) Execute(ctx context.Context, sessionID string, a chain.Action) (string, string, int, error) {
	key := ""
	if a.Command != nil {
		key = a.Command.Cmd
	}
	f.mu.Lock()
	r := f.results[key]
	f.calls = append(f.calls, key)
	f.mu.Unlock()

	if r.delay > 0 {
		select {
		case <-ctx.Done():
			return "", "", 1, ctx.Err()
		case <-time.After(r.delay):
		}
	}
	return r.stdout, r.stderr, r.exitCode, r.err
}

// fakeAtomicResolver always returns a simple sh command.
type fakeAtomicResolver struct{}

func (fakeAtomicResolver) Resolve(id, name, guid string, idx int, args map[string]string) (string, string, error) {
	return "sh", "echo atomic:" + id, nil
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return st
}

func collectEvents(exec *chain.Executor) []chain.Event {
	var evs []chain.Event
	for ev := range exec.Events() {
		evs = append(evs, ev)
	}
	return evs
}

func runChain(t *testing.T, ch chain.Chain, stepExec chain.StepExecutor) ([]chain.Event, error) {
	t.Helper()
	st := openStore(t)
	st.CreateChain(store.ChainRecord{ID: ch.ID, Name: ch.Name, Data: "{}"})
	exec := chain.NewExecutor(stepExec, fakeAtomicResolver{}, st)
	var evs []chain.Event
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		evs = collectEvents(exec)
	}()
	err := exec.Run(context.Background(), ch, "sess1", "exec1")
	wg.Wait()
	return evs, err
}

func cmdStep(id, cmd string, deps ...string) chain.Step {
	s := chain.Step{
		ID: id,
		Action: chain.Action{
			Type:    chain.ActionCommand,
			Command: &chain.CommandAction{Interpreter: "sh", Cmd: cmd},
		},
	}
	for _, d := range deps {
		s.DependsOn = append(s.DependsOn, chain.Dep{ID: d})
	}
	return s
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestExecutorLinearHappyPath(t *testing.T) {
	fe := newFakeExec()
	fe.set("echo a", fakeResult{stdout: "a"})
	fe.set("echo b", fakeResult{stdout: "b"})
	fe.set("echo c", fakeResult{stdout: "c"})

	ch := chain.Chain{
		ID: "test", Name: "test",
		Steps: []chain.Step{
			cmdStep("s1", "echo a"),
			cmdStep("s2", "echo b", "s1"),
			cmdStep("s3", "echo c", "s2"),
		},
	}
	evs, err := runChain(t, ch, fe)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Count step_done events
	var dones int
	for _, ev := range evs {
		if ev.Type == chain.EventStepDone {
			dones++
		}
	}
	if dones != 3 {
		t.Errorf("want 3 step_done, got %d", dones)
	}
}

func TestExecutorOutputVar(t *testing.T) {
	fe := newFakeExec()
	fe.set("echo hello", fakeResult{stdout: "hello\n"})
	fe.set("echo hello", fakeResult{stdout: "hello\n"}) // s2 uses substituted cmd

	ch := chain.Chain{
		ID: "test", Name: "test",
		Steps: []chain.Step{
			{
				ID: "s1",
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Interpreter: "sh", Cmd: "echo hello"},
				},
				OutputVar: "myvar",
			},
			{
				ID:        "s2",
				DependsOn: []chain.Dep{{ID: "s1"}},
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Interpreter: "sh", Cmd: "echo {{myvar}}"},
				},
			},
		},
	}

	// s2's cmd after substitution should be "echo hello"
	fe.set("echo hello", fakeResult{stdout: "hello"})

	st := openStore(t)
	st.CreateChain(store.ChainRecord{ID: ch.ID, Name: ch.Name, Data: "{}"})
	exec := chain.NewExecutor(fe, fakeAtomicResolver{}, st)
	var evs []chain.Event
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		evs = collectEvents(exec)
	}()
	if err := exec.Run(context.Background(), ch, "sess", "ex1"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	wg.Wait()

	var dones int
	for _, ev := range evs {
		if ev.Type == chain.EventStepDone {
			dones++
		}
	}
	if dones != 2 {
		t.Errorf("want 2 step_done, got %d", dones)
	}
}

func TestExecutorOnFailAbort(t *testing.T) {
	fe := newFakeExec()
	fe.set("fail", fakeResult{exitCode: 1})
	fe.set("ok", fakeResult{stdout: "ok"})

	ch := chain.Chain{
		ID: "test", Name: "test",
		Steps: []chain.Step{
			{
				ID: "s1",
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Cmd: "fail"},
				},
				OnFail: "abort",
			},
			cmdStep("s2", "ok", "s1"),
		},
	}
	_, err := runChain(t, ch, fe)
	if err == nil {
		t.Fatal("expected abort error, got nil")
	}
}

func TestExecutorOnFailContinueNoErr(t *testing.T) {
	fe := newFakeExec()
	fe.set("fail", fakeResult{exitCode: 1})
	fe.set("ok", fakeResult{stdout: "ok"})

	ch := chain.Chain{
		ID: "test", Name: "test",
		Steps: []chain.Step{
			{
				ID: "s1",
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Cmd: "fail"},
				},
				OnFail: "continue_no_err",
			},
			cmdStep("s2", "ok"),
		},
	}
	_, err := runChain(t, ch, fe)
	if err != nil {
		t.Fatalf("continue_no_err should not return error, got: %v", err)
	}
}

func TestExecutorOnFailContinue(t *testing.T) {
	fe := newFakeExec()
	fe.set("fail", fakeResult{exitCode: 1})
	fe.set("ok", fakeResult{stdout: "ok"})

	ch := chain.Chain{
		ID: "test", Name: "test",
		Steps: []chain.Step{
			{
				ID: "s1",
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Cmd: "fail"},
				},
				OnFail: "continue",
			},
			cmdStep("s2", "ok"),
		},
	}
	_, err := runChain(t, ch, fe)
	if err == nil {
		t.Fatal("continue with failure should still return chain error")
	}
}

func TestExecutorSkipDependents(t *testing.T) {
	fe := newFakeExec()
	fe.set("fail", fakeResult{exitCode: 1})

	ch := chain.Chain{
		ID: "test", Name: "test",
		Steps: []chain.Step{
			{
				ID: "s1",
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Cmd: "fail"},
				},
				OnFail: "skip_dependents",
			},
			cmdStep("s2", "ok", "s1"),
		},
	}
	evs, _ := runChain(t, ch, fe)
	var skips int
	for _, ev := range evs {
		if ev.Type == chain.EventStepSkipped {
			skips++
		}
	}
	if skips == 0 {
		t.Error("expected s2 to be skipped")
	}
}

func TestExecutorConditionSkip(t *testing.T) {
	fe := newFakeExec()
	fe.set("echo linux", fakeResult{stdout: "linux"})

	ch := chain.Chain{
		ID: "test", Name: "test",
		Steps: []chain.Step{
			{
				ID: "probe",
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Cmd: "echo linux"},
				},
				OutputVar: "os",
			},
			{
				ID:        "windows_only",
				DependsOn: []chain.Dep{{ID: "probe"}},
				Conditions: []chain.Condition{
					{Var: "os", Op: "contains", Value: "Windows"},
				},
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Cmd: "should_not_run"},
				},
			},
		},
	}
	evs, err := runChain(t, ch, fe)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var skipped bool
	for _, ev := range evs {
		if ev.Type == chain.EventStepSkipped && ev.StepID == "windows_only" {
			skipped = true
		}
	}
	if !skipped {
		t.Error("windows_only step should be skipped")
	}
}

func TestExecutorContextCancel(t *testing.T) {
	fe := newFakeExec()
	fe.set("slow", fakeResult{delay: 5 * time.Second})

	ch := chain.Chain{
		ID: "test", Name: "test",
		Steps: []chain.Step{
			{
				ID: "s1",
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Cmd: "slow"},
				},
			},
		},
	}

	st := openStore(t)
	st.CreateChain(store.ChainRecord{ID: ch.ID, Name: ch.Name, Data: "{}"})
	exec := chain.NewExecutor(fe, nil, st)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { collectEvents(exec) }()
	go func() {
		done <- exec.Run(ctx, ch, "sess", "exec2")
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected context error, got nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not respect context cancellation")
	}
}

func TestExecutorStepLogPersisted(t *testing.T) {
	fe := newFakeExec()
	fe.set("echo hi", fakeResult{stdout: "hi"})

	ch := chain.Chain{
		ID: "log-test", Name: "log-test",
		Steps: []chain.Step{cmdStep("s1", "echo hi")},
	}

	st := openStore(t)
	st.CreateChain(store.ChainRecord{ID: ch.ID, Name: ch.Name, Data: "{}"})
	exec := chain.NewExecutor(fe, nil, st)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectEvents(exec)
	}()
	if err := exec.Run(context.Background(), ch, "sess", "execlog1"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	wg.Wait()

	logs, err := st.GetStepLogs("execlog1")
	if err != nil {
		t.Fatalf("GetStepLogs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("expected step logs to be persisted")
	}
	last := logs[len(logs)-1]
	if last.Status != "done" {
		t.Errorf("final log status=%q want done", last.Status)
	}
}

func TestExecutorAtomicResolution(t *testing.T) {
	fe := newFakeExec()
	fe.set("echo atomic:T1082", fakeResult{stdout: "resolved"})

	ch := chain.Chain{
		ID: "atomic-test", Name: "atomic-test",
		Steps: []chain.Step{
			{
				ID: "s1",
				Action: chain.Action{
					Type:      chain.ActionAtomic,
					AtomicRef: &chain.AtomicRef{ID: "T1082", Test: 0},
				},
			},
		},
	}
	_, err := runChain(t, ch, fe)
	if err != nil {
		t.Fatalf("Run with atomic: %v", err)
	}
}
