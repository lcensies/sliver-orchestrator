package sliver

import (
	"context"
	"sync"
	"testing"

	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/rpcpb"
	"google.golang.org/grpc"

	"github.com/bishopfox/sliver/scenario/chain"
	"github.com/bishopfox/sliver/scenario/initialaccess"
)

// fakeRPC embeds the full SliverRPCClient interface (so it satisfies the type) but
// only implements GetSessions. Each GetSessions call returns the next scripted
// snapshot, simulating a beacon appearing after delivery.
type fakeRPC struct {
	rpcpb.SliverRPCClient
	mu        sync.Mutex
	snapshots [][]*clientpb.Session
	idx       int
}

func (f *fakeRPC) GetSessions(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Sessions, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	snap := f.snapshots[len(f.snapshots)-1]
	if f.idx < len(f.snapshots) {
		snap = f.snapshots[f.idx]
		f.idx++
	}
	return &clientpb.Sessions{Sessions: snap}, nil
}

// stubModule returns a fixed Result without touching the network.
type stubModule struct {
	name string
	res  initialaccess.Result
}

func (s *stubModule) Name() string { return s.name }
func (s *stubModule) Run(context.Context, initialaccess.Request) (initialaccess.Result, error) {
	return s.res, nil
}

func iaAction(host string, wait chain.WaitSpec) chain.Action {
	a := chain.Action{
		Type:          chain.ActionInitialAccess,
		InitialAccess: &chain.InitialAccessAction{Target: "web1", Module: "stub", Wait: wait},
	}
	a.SetResolvedTarget(chain.Target{Name: "web1", Host: host})
	return a
}

func TestExecInitialAccess_BindsNewSession(t *testing.T) {
	rpc := &fakeRPC{snapshots: [][]*clientpb.Session{
		{{ID: "old-1"}}, // snapshot before delivery
		{{ID: "old-1"}, {ID: "new-9", Hostname: "victim-web", OS: "linux"}}, // after
	}}
	reg := initialaccess.NewRegistry()
	reg.Register(&stubModule{name: "stub", res: initialaccess.Result{Ok: true, Note: "done"}})

	e := NewExecutor(rpc, "").WithInitialAccessModules(reg)
	stdout, _, code, err := e.Execute(context.Background(), "", iaAction("172.20.0.30", chain.WaitSpec{Timeout: "5s", MatchHostname: "victim-web"}))
	if err != nil || code != 0 {
		t.Fatalf("execInitialAccess failed: code=%d err=%v", code, err)
	}
	if stdout != "new-9" {
		t.Fatalf("expected new session id on stdout, got %q", stdout)
	}
}

func TestExecInitialAccess_ModuleFailureNoWait(t *testing.T) {
	rpc := &fakeRPC{snapshots: [][]*clientpb.Session{{{ID: "old-1"}}}}
	reg := initialaccess.NewRegistry()
	reg.Register(&stubModule{name: "stub", res: initialaccess.Result{Ok: false, Note: "exploit missed"}})

	e := NewExecutor(rpc, "").WithInitialAccessModules(reg)
	_, _, code, err := e.Execute(context.Background(), "", iaAction("172.20.0.30", chain.WaitSpec{Timeout: "5s"}))
	if err == nil || code == 0 {
		t.Fatal("expected failure when module reports Ok=false")
	}
}

func TestExecInitialAccess_TimeoutWhenNoNewSession(t *testing.T) {
	// Session set never grows -> wait times out.
	rpc := &fakeRPC{snapshots: [][]*clientpb.Session{{{ID: "old-1"}}}}
	reg := initialaccess.NewRegistry()
	reg.Register(&stubModule{name: "stub", res: initialaccess.Result{Ok: true}})

	e := NewExecutor(rpc, "").WithInitialAccessModules(reg)
	_, _, code, err := e.Execute(context.Background(), "", iaAction("172.20.0.30", chain.WaitSpec{Timeout: "1s"}))
	if err == nil || code == 0 {
		t.Fatal("expected timeout error when no new session appears")
	}
}

func TestExecInitialAccess_UnknownModule(t *testing.T) {
	rpc := &fakeRPC{snapshots: [][]*clientpb.Session{{{ID: "old-1"}}}}
	e := NewExecutor(rpc, "") // default registry: only "external"
	_, _, code, err := e.Execute(context.Background(), "", iaAction("172.20.0.30", chain.WaitSpec{Timeout: "1s"}))
	if err == nil || code == 0 {
		t.Fatal("expected error for unregistered module 'stub'")
	}
}
