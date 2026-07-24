package chain_test

import (
	"context"
	"sync"
	"testing"

	"github.com/bishopfox/sliver/scenario/chain"
)

// runChain drives a chain to completion against a test StepExecutor and returns
// the collected events plus the executor's terminal error (nil on success).
func runChain(t *testing.T, ch chain.Chain, stepExec chain.StepExecutor) ([]chain.Event, error) {
	t.Helper()
	ex := chain.NewExecutor(stepExec, nil, nil)
	errCh := make(chan error, 1)
	go func() { errCh <- ex.Run(context.Background(), ch, "", "test-exec") }()
	var events []chain.Event
	for ev := range ex.Events() {
		events = append(events, ev)
	}
	return events, <-errCh
}

// iaExec is a StepExecutor that handles initial_access (returning a fixed session
// id and recording the resolved target) and command steps (recording the session
// id it was dispatched against).
type iaExec struct {
	mu          sync.Mutex
	newSession  string
	gotTarget   chain.Target
	cmdSessions map[string]string // cmd -> sessionID it ran against
}

func (e *iaExec) Execute(ctx context.Context, sessionID string, a chain.Action) (string, string, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch a.Type {
	case chain.ActionInitialAccess:
		t, ok := a.ResolvedTarget()
		if !ok {
			return "", "resolved target missing", 1, nil
		}
		e.gotTarget = t
		return e.newSession, "", 0, nil
	case chain.ActionCommand:
		if e.cmdSessions == nil {
			e.cmdSessions = map[string]string{}
		}
		e.cmdSessions[a.Command.Cmd] = sessionID
		return "ok", "", 0, nil
	}
	return "", "unexpected action", 1, nil
}

func TestSubstituteActionInitialAccess(t *testing.T) {
	a := chain.Action{
		Type: chain.ActionInitialAccess,
		InitialAccess: &chain.InitialAccessAction{
			Target: "{{tgt}}",
			Module: "external",
			Config: map[string]string{"implant_url": "http://c2/{{path}}"},
			Wait:   chain.WaitSpec{MatchHostname: "{{host}}"},
		},
	}
	vars := chain.VarMap{"tgt": "web1", "path": "impl", "host": "victim-web"}
	out := chain.SubstituteAction(a, vars)
	ia := out.InitialAccess
	if ia.Target != "web1" || ia.Config["implant_url"] != "http://c2/impl" || ia.Wait.MatchHostname != "victim-web" {
		t.Fatalf("substitution failed: %+v", ia)
	}
	// original must be untouched (deep copy)
	if a.InitialAccess.Target != "{{tgt}}" {
		t.Fatal("SubstituteAction mutated the original action")
	}
}

func TestInitialAccessBindsSessionForDownstream(t *testing.T) {
	fe := &iaExec{newSession: "sess-new-123"}
	ch := chain.Chain{
		ID: "ia", Name: "ia",
		Targets: []chain.Target{{Name: "web1", Host: "172.20.0.30", Port: 8080, Attrs: map[string]string{"path": "/ping"}}},
		Steps: []chain.Step{
			{
				ID:        "breach",
				OutputVar: "web1_session",
				Action: chain.Action{
					Type:          chain.ActionInitialAccess,
					InitialAccess: &chain.InitialAccessAction{Target: "web1", Module: "external"},
				},
			},
			{
				ID:        "recon",
				DependsOn: []chain.Dep{{ID: "breach"}},
				SessionID: "{{web1_session}}",
				Action:    chain.Action{Type: chain.ActionCommand, Command: &chain.CommandAction{Cmd: "id"}},
			},
		},
	}

	_, err := runChain(t, ch, fe)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if fe.gotTarget.Host != "172.20.0.30" {
		t.Fatalf("target not resolved into action: %+v", fe.gotTarget)
	}
	if got := fe.cmdSessions["id"]; got != "sess-new-123" {
		t.Fatalf("downstream step used session %q, want the newly bound session", got)
	}
}

func TestInitialAccessUnknownTargetFails(t *testing.T) {
	fe := &iaExec{newSession: "x"}
	ch := chain.Chain{
		ID: "ia2", Name: "ia2",
		Steps: []chain.Step{
			{
				ID:     "breach",
				OnFail: chain.FailAbort,
				Action: chain.Action{
					Type:          chain.ActionInitialAccess,
					InitialAccess: &chain.InitialAccessAction{Target: "ghost", Module: "external"},
				},
			},
		},
	}
	_, err := runChain(t, ch, fe)
	if err == nil {
		t.Fatal("expected chain to fail on unknown target")
	}
}
