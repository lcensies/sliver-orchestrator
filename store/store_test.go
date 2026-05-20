package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bishopfox/sliver/scenario/store"
)

func openTestDB(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return st
}

// ── Chain CRUD ────────────────────────────────────────────────────────────────

func TestChainCRUD(t *testing.T) {
	st := openTestDB(t)

	rec := store.ChainRecord{ID: "c1", Name: "Chain 1", Description: "desc", Data: `{"id":"c1"}`}
	if err := st.CreateChain(rec); err != nil {
		t.Fatalf("CreateChain: %v", err)
	}

	got, err := st.GetChain("c1")
	if err != nil {
		t.Fatalf("GetChain: %v", err)
	}
	if got.Name != "Chain 1" {
		t.Errorf("got name %q, want %q", got.Name, "Chain 1")
	}

	rec.Name = "Updated"
	if err := st.UpdateChain(rec); err != nil {
		t.Fatalf("UpdateChain: %v", err)
	}
	got, _ = st.GetChain("c1")
	if got.Name != "Updated" {
		t.Errorf("after update name=%q", got.Name)
	}

	if err := st.DeleteChain("c1"); err != nil {
		t.Fatalf("DeleteChain: %v", err)
	}
	if _, err := st.GetChain("c1"); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestChainListOrdering(t *testing.T) {
	st := openTestDB(t)

	for _, id := range []string{"a", "b", "c"} {
		if err := st.CreateChain(store.ChainRecord{ID: id, Name: id, Data: "{}"}); err != nil {
			t.Fatalf("CreateChain %s: %v", id, err)
		}
	}

	records, err := st.ListChains()
	if err != nil {
		t.Fatalf("ListChains: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("want 3 chains, got %d", len(records))
	}
	// newest first
	if records[0].ID != "c" {
		t.Errorf("first record should be newest 'c', got %q", records[0].ID)
	}
}

func TestGetChainNotFound(t *testing.T) {
	st := openTestDB(t)
	if _, err := st.GetChain("nonexistent"); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestListChainsEmpty(t *testing.T) {
	st := openTestDB(t)
	records, err := st.ListChains()
	if err != nil {
		t.Fatalf("ListChains: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("want 0, got %d", len(records))
	}
}

// ── Execution CRUD ────────────────────────────────────────────────────────────

func TestExecutionCRUD(t *testing.T) {
	st := openTestDB(t)

	if err := st.CreateChain(store.ChainRecord{ID: "ch1", Name: "c", Data: "{}"}); err != nil {
		t.Fatal(err)
	}

	rec := store.ExecutionRecord{
		ID:        "e1",
		ChainID:   "ch1",
		SessionID: "sess1",
		Status:    "running",
		StartedAt: time.Now(),
	}
	if err := st.CreateExecution(rec); err != nil {
		t.Fatalf("CreateExecution: %v", err)
	}

	got, err := st.GetExecution("e1")
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("status=%q want running", got.Status)
	}

	fin := time.Now()
	if err := st.UpdateExecutionStatus("e1", "done", "", &fin); err != nil {
		t.Fatalf("UpdateExecutionStatus: %v", err)
	}
	got, _ = st.GetExecution("e1")
	if got.Status != "done" {
		t.Errorf("after update status=%q", got.Status)
	}
	if got.FinishedAt == nil {
		t.Error("FinishedAt should be set")
	}
}

func TestListExecutionsFilterByChainID(t *testing.T) {
	st := openTestDB(t)

	for _, cid := range []string{"ca", "cb"} {
		st.CreateChain(store.ChainRecord{ID: cid, Name: cid, Data: "{}"})
	}
	st.CreateExecution(store.ExecutionRecord{ID: "e1", ChainID: "ca", SessionID: "s", Status: "done", StartedAt: time.Now()})
	st.CreateExecution(store.ExecutionRecord{ID: "e2", ChainID: "cb", SessionID: "s", Status: "done", StartedAt: time.Now()})
	st.CreateExecution(store.ExecutionRecord{ID: "e3", ChainID: "ca", SessionID: "s", Status: "done", StartedAt: time.Now()})

	all, _ := st.ListExecutions("")
	if len(all) != 3 {
		t.Errorf("want 3 all, got %d", len(all))
	}

	filtered, _ := st.ListExecutions("ca")
	if len(filtered) != 2 {
		t.Errorf("want 2 for ca, got %d", len(filtered))
	}
}

// ── Step Logs ─────────────────────────────────────────────────────────────────

func TestStepLogUpsert(t *testing.T) {
	st := openTestDB(t)
	st.CreateChain(store.ChainRecord{ID: "c1", Name: "c", Data: "{}"})
	st.CreateExecution(store.ExecutionRecord{ID: "ex1", ChainID: "c1", SessionID: "s", Status: "running", StartedAt: time.Now()})

	// First insert: running
	if err := st.LogStep("ex1", "step1", "running", "", "", 0, "", 0); err != nil {
		t.Fatalf("LogStep (running): %v", err)
	}
	// Second insert for same step: done — should upsert
	if err := st.LogStep("ex1", "step1", "done", "output", "", 0, "", 100); err != nil {
		t.Fatalf("LogStep (done): %v", err)
	}

	logs, err := st.GetStepLogs("ex1")
	if err != nil {
		t.Fatalf("GetStepLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("want 1 log (upserted), got %d", len(logs))
	}
	if logs[0].Status != "done" {
		t.Errorf("status=%q want done", logs[0].Status)
	}
	if logs[0].Stdout != "output" {
		t.Errorf("stdout=%q want output", logs[0].Stdout)
	}
}

func TestStepLogOrdering(t *testing.T) {
	st := openTestDB(t)
	st.CreateChain(store.ChainRecord{ID: "c1", Name: "c", Data: "{}"})
	st.CreateExecution(store.ExecutionRecord{ID: "ex1", ChainID: "c1", SessionID: "s", Status: "running", StartedAt: time.Now()})

	for _, id := range []string{"s1", "s2", "s3"} {
		st.LogStep("ex1", id, "done", id+"_out", "", 0, "", 0)
	}

	logs, _ := st.GetStepLogs("ex1")
	if len(logs) != 3 {
		t.Fatalf("want 3, got %d", len(logs))
	}
	for i, want := range []string{"s1_out", "s2_out", "s3_out"} {
		if logs[i].Stdout != want {
			t.Errorf("logs[%d].Stdout=%q want %q", i, logs[i].Stdout, want)
		}
	}
}

func TestCountStepLogs(t *testing.T) {
	st := openTestDB(t)
	st.CreateChain(store.ChainRecord{ID: "c1", Name: "c", Data: "{}"})
	st.CreateExecution(store.ExecutionRecord{ID: "ex1", ChainID: "c1", SessionID: "s", Status: "running", StartedAt: time.Now()})

	st.LogStep("ex1", "s1", "done", "", "", 0, "", 0)
	st.LogStep("ex1", "s2", "done", "", "", 0, "", 0)
	st.LogStep("ex1", "s3", "done", "", "", 0, "", 0)

	logs, _ := st.GetStepLogs("ex1")
	if len(logs) == 0 {
		t.Fatal("expected logs")
	}
	firstID := logs[0].ID

	count, err := st.CountStepLogs("ex1", firstID)
	if err != nil {
		t.Fatalf("CountStepLogs: %v", err)
	}
	if count != 2 {
		t.Errorf("want 2 after first, got %d", count)
	}
}
