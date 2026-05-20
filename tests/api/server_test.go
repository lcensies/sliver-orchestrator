package apitest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/scenario/api"
	"github.com/bishopfox/sliver/scenario/atomic"
	"github.com/bishopfox/sliver/scenario/chain"
	"github.com/bishopfox/sliver/scenario/store"
)

// ─── test helpers ─────────────────────────────────────────────────────────────

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	lib := atomic.NewLibrary()
	loadFixtureAtomics(t, lib)
	rpc := &stubRPC{}
	srv := api.NewServer(st, lib, rpc, "", "", "*")
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, st
}

func loadFixtureAtomics(t *testing.T, lib *atomic.Library) {
	t.Helper()
	dir := t.TempDir()
	techDir := filepath.Join(dir, "T1082")
	os.MkdirAll(techDir, 0o755)
	os.WriteFile(filepath.Join(techDir, "T1082.yaml"), []byte(`
attack_technique: T1082
display_name: System Information Discovery
tactic: discovery
platforms: [linux]
atomic_tests:
  - name: List OS Info
    supported_platforms: [linux]
    executor:
      name: sh
      command: uname -a
`), 0o644)
	if err := lib.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
}

func get(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/v1" + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func postJSON(t *testing.T, ts *httptest.Server, path string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(ts.URL+"/api/v1"+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func putJSON(t *testing.T, ts *httptest.Server, path string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1"+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

func del(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1"+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d want=%d body=%s", resp.StatusCode, want, body)
	}
}

func minimalChain(name string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": "test chain",
		"steps": []map[string]any{
			{
				"id": "s1",
				"action": map[string]any{
					"type":    "command",
					"command": map[string]any{"interpreter": "sh", "cmd": "echo hello"},
				},
			},
		},
	}
}

// ─── Health ───────────────────────────────────────────────────────────────────

func TestHealthOK(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/health")
	assertStatus(t, resp, 200)
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status=%v want ok", body["status"])
	}
}

func TestCORSPreflight(t *testing.T) {
	ts, _ := newTestServer(t)
	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, resp, 204)
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Error("missing Access-Control-Allow-Origin")
	}
}

func TestCORSResponseHeaders(t *testing.T) {
	ts, _ := newTestServer(t)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, _ := http.DefaultClient.Do(req)
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Error("CORS header missing on GET response")
	}
}

// ─── Atomics ──────────────────────────────────────────────────────────────────

func TestListAtomicsWithData(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/atomics")
	assertStatus(t, resp, 200)
	var items []map[string]any
	decodeJSON(t, resp, &items)
	if len(items) == 0 {
		t.Fatal("expected atomics, got empty list")
	}
	if items[0]["id"] != "T1082" {
		t.Errorf("expected T1082, got %v", items[0]["id"])
	}
}

func TestListAtomicsFilterTactic(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/atomics?tactic=discovery")
	assertStatus(t, resp, 200)
	var items []map[string]any
	decodeJSON(t, resp, &items)
	if len(items) == 0 {
		t.Error("expected at least one discovery atomic")
	}
	resp2 := get(t, ts, "/atomics?tactic=exfiltration")
	assertStatus(t, resp2, 200)
	var items2 []map[string]any
	decodeJSON(t, resp2, &items2)
	if len(items2) != 0 {
		t.Errorf("expected 0 exfiltration atomics, got %d", len(items2))
	}
}

func TestGetAtomicFound(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/atomics/T1082")
	assertStatus(t, resp, 200)
	var item map[string]any
	decodeJSON(t, resp, &item)
	if item["ID"] != "T1082" {
		t.Errorf("got %v", item["attack_technique"])
	}
}

func TestGetAtomicNotFound(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/atomics/T9999")
	assertStatus(t, resp, 404)
}

// ─── Chains CRUD ──────────────────────────────────────────────────────────────

func TestListChainsEmpty(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/chains")
	assertStatus(t, resp, 200)
	var items []any
	decodeJSON(t, resp, &items)
	if len(items) != 0 {
		t.Errorf("want empty list, got %d items", len(items))
	}
}

func TestCreateAndGetChain(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := postJSON(t, ts, "/chains", minimalChain("My Chain"))
	assertStatus(t, resp, 201)
	var created map[string]any
	decodeJSON(t, resp, &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("expected id in response")
	}
	if created["name"] != "My Chain" {
		t.Errorf("name=%v", created["name"])
	}

	resp2 := get(t, ts, "/chains/"+id)
	assertStatus(t, resp2, 200)
	var got map[string]any
	decodeJSON(t, resp2, &got)
	if got["name"] != "My Chain" {
		t.Errorf("GET name=%v", got["name"])
	}
}

func TestCreateChainYAML(t *testing.T) {
	ts, _ := newTestServer(t)
	yamlBody := `
name: yaml chain
steps:
  - id: s1
    action:
      type: command
      command:
        interpreter: sh
        cmd: echo hi
`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/chains",
		strings.NewReader(yamlBody))
	req.Header.Set("Content-Type", "application/yaml")
	resp, _ := http.DefaultClient.Do(req)
	assertStatus(t, resp, 201)
	var created map[string]any
	decodeJSON(t, resp, &created)
	if created["name"] != "yaml chain" {
		t.Errorf("name=%v", created["name"])
	}
}

func TestCreateChainMissingName(t *testing.T) {
	ts, _ := newTestServer(t)
	body := map[string]any{"steps": []any{}}
	resp := postJSON(t, ts, "/chains", body)
	assertStatus(t, resp, 400)
}

func TestCreateChainInvalidDAG(t *testing.T) {
	ts, _ := newTestServer(t)
	body := map[string]any{
		"name": "cycle",
		"steps": []map[string]any{
			{"id": "a", "depends_on": []string{"b"},
				"action": map[string]any{"type": "command", "command": map[string]any{"cmd": "x"}}},
			{"id": "b", "depends_on": []string{"a"},
				"action": map[string]any{"type": "command", "command": map[string]any{"cmd": "y"}}},
		},
	}
	resp := postJSON(t, ts, "/chains", body)
	assertStatus(t, resp, 400)
}

func TestUpdateChain(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := postJSON(t, ts, "/chains", minimalChain("original"))
	var created map[string]any
	decodeJSON(t, resp, &created)
	id := created["id"].(string)

	updated := minimalChain("updated")
	updated["id"] = id
	resp2 := putJSON(t, ts, "/chains/"+id, updated)
	assertStatus(t, resp2, 200)
	var got map[string]any
	decodeJSON(t, resp2, &got)
	if got["name"] != "updated" {
		t.Errorf("name=%v", got["name"])
	}
}

func TestUpdateChainNotFound(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := putJSON(t, ts, "/chains/no-such-id", minimalChain("x"))
	assertStatus(t, resp, 404)
}

func TestDeleteChain(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := postJSON(t, ts, "/chains", minimalChain("to-delete"))
	var created map[string]any
	decodeJSON(t, resp, &created)
	id := created["id"].(string)

	resp2 := del(t, ts, "/chains/"+id)
	assertStatus(t, resp2, 204)

	resp3 := get(t, ts, "/chains/"+id)
	assertStatus(t, resp3, 404)
}

func TestListChains(t *testing.T) {
	ts, _ := newTestServer(t)
	for i := 0; i < 3; i++ {
		postJSON(t, ts, "/chains", minimalChain(fmt.Sprintf("chain-%d", i)))
	}
	resp := get(t, ts, "/chains")
	assertStatus(t, resp, 200)
	var items []any
	decodeJSON(t, resp, &items)
	if len(items) != 3 {
		t.Errorf("want 3, got %d", len(items))
	}
}

// ─── Execute / Dry-run ────────────────────────────────────────────────────────

func TestDryRunReturnsOrder(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := postJSON(t, ts, "/chains", minimalChain("dryrun"))
	var created map[string]any
	decodeJSON(t, resp, &created)
	id := created["id"].(string)

	body := map[string]any{"session_id": "sess1", "dry_run": true}
	resp2 := postJSON(t, ts, "/chains/"+id+"/execute", body)
	assertStatus(t, resp2, 200)
	var result map[string]any
	decodeJSON(t, resp2, &result)
	if result["dry_run"] != true {
		t.Errorf("dry_run=%v", result["dry_run"])
	}
	order, ok := result["order"].([]any)
	if !ok || len(order) == 0 {
		t.Errorf("expected non-empty order, got %v", result["order"])
	}
}

func TestExecuteMissingSessionID(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := postJSON(t, ts, "/chains", minimalChain("exec-miss"))
	var created map[string]any
	decodeJSON(t, resp, &created)
	id := created["id"].(string)

	resp2 := postJSON(t, ts, "/chains/"+id+"/execute", map[string]any{})
	assertStatus(t, resp2, 400)
}

func TestExecuteChainNotFound(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := postJSON(t, ts, "/chains/no-such/execute", map[string]any{"session_id": "s"})
	assertStatus(t, resp, 404)
}

// ─── Executions ───────────────────────────────────────────────────────────────

func TestListExecutionsEmpty(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/executions")
	assertStatus(t, resp, 200)
	var items []any
	decodeJSON(t, resp, &items)
	if len(items) != 0 {
		t.Errorf("want 0, got %d", len(items))
	}
}

func seedExecution(t *testing.T, st *store.Store, chainID, execID string) {
	t.Helper()
	st.CreateChain(store.ChainRecord{ID: chainID, Name: chainID, Data: "{}"})
	st.CreateExecution(store.ExecutionRecord{
		ID:        execID,
		ChainID:   chainID,
		SessionID: "sess",
		Status:    "done",
		StartedAt: time.Now(),
	})
	st.LogStep(execID, "s1", "done", "output", "", 0, "", 100)
}

func TestGetExecutionWithSteps(t *testing.T) {
	ts, st := newTestServer(t)
	seedExecution(t, st, "c1", "e1")

	resp := get(t, ts, "/executions/e1")
	assertStatus(t, resp, 200)
	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["execution"] == nil {
		t.Error("missing execution key")
	}
	steps, _ := result["steps"].([]any)
	if len(steps) == 0 {
		t.Error("expected at least one step log")
	}
}

func TestGetExecutionNotFound(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/executions/nope")
	assertStatus(t, resp, 404)
}

func TestListExecutionsFilterByChainID(t *testing.T) {
	ts, st := newTestServer(t)
	seedExecution(t, st, "ca", "ea1")
	seedExecution(t, st, "cb", "eb1")
	seedExecution(t, st, "ca", "ea2")

	resp := get(t, ts, "/executions?chain_id=ca")
	assertStatus(t, resp, 200)
	var items []any
	decodeJSON(t, resp, &items)
	if len(items) != 2 {
		t.Errorf("want 2 for ca, got %d", len(items))
	}
}

// ─── Cancel ───────────────────────────────────────────────────────────────────

func TestCancelNotFound(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := postJSON(t, ts, "/executions/no-such/cancel", nil)
	assertStatus(t, resp, 404)
}

func TestCancelAlreadyDone(t *testing.T) {
	ts, st := newTestServer(t)
	seedExecution(t, st, "c1", "edone")

	resp := postJSON(t, ts, "/executions/edone/cancel", nil)
	// already done, no running cancel func registered → 409
	assertStatus(t, resp, 409)
}

// ─── Sessions ─────────────────────────────────────────────────────────────────

func TestListSessionsProxied(t *testing.T) {
	ts, _ := newTestServer(t)
	// stubRPC returns empty Sessions by default
	resp := get(t, ts, "/sessions")
	assertStatus(t, resp, 200)
	var items []any
	decodeJSON(t, resp, &items)
	if items == nil {
		t.Error("expected array, got nil")
	}
}

func TestListSessionsWithData(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	lib := atomic.NewLibrary()
	rpc := &stubRPC{
		getSessionsFn: func() (*clientpb.Sessions, error) {
			return &clientpb.Sessions{
				Sessions: []*clientpb.Session{
					{ID: "sess-1", Name: "GHOST", OS: "linux", Hostname: "victim"},
				},
			}, nil
		},
	}
	srv := api.NewServer(st, lib, rpc, "", "", "*")
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, _ := http.Get(ts.URL + "/api/v1/sessions")
	assertStatus(t, resp, 200)
	var sessions []map[string]any
	decodeJSON(t, resp, &sessions)
	if len(sessions) != 1 || sessions[0]["id"] != "sess-1" {
		t.Errorf("unexpected sessions: %v", sessions)
	}
}

// ─── SSE stream (replay already-done execution) ───────────────────────────────

func TestStreamReplaysDoneExecution(t *testing.T) {
	ts, st := newTestServer(t)
	seedExecution(t, st, "c1", "edone2")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ts.URL + "/api/v1/executions/edone2/stream")
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()
	assertStatus(t, resp, 200)

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type=%q want text/event-stream", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if !strings.Contains(s, "event: step_log") {
		t.Errorf("missing step_log event in: %q", s)
	}
	if !strings.Contains(s, "event: done") {
		t.Errorf("missing done event in: %q", s)
	}
}

func TestStreamNotFound(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := get(t, ts, "/executions/nope/stream")
	assertStatus(t, resp, 404)
}

// ─── Execute then check execution created ─────────────────────────────────────

func TestExecuteChainCreatesExecution(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := postJSON(t, ts, "/chains", map[string]any{
		"name": "exec-chain",
		"steps": []map[string]any{
			{"id": "s1", "action": map[string]any{
				"type": "command",
				"command": map[string]any{
					"interpreter": "sh", "cmd": "echo ok",
				},
			}},
		},
	})
	var created map[string]any
	decodeJSON(t, resp, &created)
	id := created["id"].(string)

	resp2 := postJSON(t, ts, "/chains/"+id+"/execute",
		map[string]any{"session_id": "test-session"})
	assertStatus(t, resp2, 202)
	var execResp map[string]any
	decodeJSON(t, resp2, &execResp)
	execID, _ := execResp["execution_id"].(string)
	if execID == "" {
		t.Fatal("expected execution_id in response")
	}

	// Verify execution record created
	time.Sleep(100 * time.Millisecond)
	resp3 := get(t, ts, "/executions/"+execID)
	assertStatus(t, resp3, 200)

	// Verify it appears in list
	resp4 := get(t, ts, "/executions?chain_id="+id)
	assertStatus(t, resp4, 200)
	var execs []any
	decodeJSON(t, resp4, &execs)
	if len(execs) == 0 {
		t.Error("expected execution in list")
	}
}

// ─── Chain with explicit ID (upsert path) ────────────────────────────────────

func TestCreateChainWithExplicitID(t *testing.T) {
	ts, _ := newTestServer(t)
	body := minimalChain("explicit-id-chain")
	body["id"] = "my-custom-id"

	resp := postJSON(t, ts, "/chains", body)
	assertStatus(t, resp, 201)
	var created map[string]any
	decodeJSON(t, resp, &created)
	if created["id"] != "my-custom-id" {
		t.Errorf("id=%v want my-custom-id", created["id"])
	}

	// Second POST with same ID should upsert
	body["name"] = "updated-name"
	resp2 := postJSON(t, ts, "/chains", body)
	assertStatus(t, resp2, 201)
	var updated map[string]any
	decodeJSON(t, resp2, &updated)
	if updated["name"] != "updated-name" {
		t.Errorf("name=%v want updated-name", updated["name"])
	}
}

// ─── Execution with chain executor (no-op step executor via nil RPC) ──────────

func TestExecuteChainWithDummyExecutor(t *testing.T) {
	ts, _ := newTestServer(t)

	// Create a chain with a command step
	ch := chain.Chain{
		Name: "dummy-exec",
		Steps: []chain.Step{
			{
				ID: "s1",
				Action: chain.Action{
					Type:    chain.ActionCommand,
					Command: &chain.CommandAction{Interpreter: "sh", Cmd: "echo hi"},
				},
			},
		},
	}
	b, _ := json.Marshal(ch)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/chains",
		bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	var created map[string]any
	decodeJSON(t, resp, &created)
	chainID := created["id"].(string)

	execResp := postJSON(t, ts, "/chains/"+chainID+"/execute",
		map[string]any{"session_id": "dummy-session"})
	assertStatus(t, execResp, 202)
}
