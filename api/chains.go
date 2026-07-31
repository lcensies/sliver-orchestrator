package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/bishopfox/sliver/scenario/chain"
	"github.com/bishopfox/sliver/scenario/store"
)

// persistScenario best-effort writes ch to <scenarioWriteDir>/<id>.yaml when
// write-back is enabled. Failures are logged, not fatal — the DB is source of record
// for the request; the file is a convenience mirror the discovery watcher re-reads.
func (s *Server) persistScenario(ch chain.Chain) {
	if s.scenarioWriteDir == "" {
		return
	}
	if err := os.MkdirAll(s.scenarioWriteDir, 0o755); err != nil {
		log.Printf("WARNING: scenario write-back mkdir %q: %v", s.scenarioWriteDir, err)
		return
	}
	data, err := yaml.Marshal(ch)
	if err != nil {
		log.Printf("WARNING: scenario write-back marshal %q: %v", ch.ID, err)
		return
	}
	path := filepath.Join(s.scenarioWriteDir, ch.ID+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("WARNING: scenario write-back %q: %v", path, err)
		return
	}
	log.Printf("Scenario %q written to %s", ch.ID, path)
}

func (s *Server) handleListChains(w http.ResponseWriter, r *http.Request) {
	records, err := s.store.ListChains()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type item struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		CreatedAt   time.Time `json:"created_at"`
	}
	out := make([]item, 0, len(records))
	for _, r := range records {
		out = append(out, item{r.ID, r.Name, r.Description, r.CreatedAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateChain(w http.ResponseWriter, r *http.Request) {
	var ch chain.Chain
	if err := decodeChain(r, &ch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if ch.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if _, err := chain.Resolve(ch.Steps); err != nil {
		writeError(w, http.StatusBadRequest, "invalid step graph: "+err.Error())
		return
	}
	idWasExplicit := ch.ID != ""
	if !idWasExplicit {
		ch.ID = uuid.NewString()
	}
	data, _ := json.Marshal(ch)
	rec := store.ChainRecord{
		ID:          ch.ID,
		Name:        ch.Name,
		Description: ch.Description,
		Data:        string(data),
	}
	var storeErr error
	if idWasExplicit {
		storeErr = s.store.UpdateChain(rec)
	} else {
		storeErr = s.store.CreateChain(rec)
	}
	if storeErr != nil {
		writeError(w, http.StatusInternalServerError, storeErr.Error())
		return
	}
	s.persistScenario(ch)
	writeJSON(w, http.StatusCreated, ch)
}

func (s *Server) handleGetChain(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.PathValue("id"), "")
	rec, err := s.store.GetChain(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "chain not found")
		return
	}
	var ch chain.Chain
	if err := json.Unmarshal([]byte(rec.Data), &ch); err != nil {
		writeError(w, http.StatusInternalServerError, "corrupted chain data")
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

func (s *Server) handleUpdateChain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetChain(id); err != nil {
		writeError(w, http.StatusNotFound, "chain not found")
		return
	}
	var ch chain.Chain
	if err := decodeChain(r, &ch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ch.ID = id
	if _, err := chain.Resolve(ch.Steps); err != nil {
		writeError(w, http.StatusBadRequest, "invalid step graph: "+err.Error())
		return
	}
	data, _ := json.Marshal(ch)
	rec := store.ChainRecord{
		ID:          id,
		Name:        ch.Name,
		Description: ch.Description,
		Data:        string(data),
	}
	if err := s.store.UpdateChain(rec); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.persistScenario(ch)
	writeJSON(w, http.StatusOK, ch)
}

func (s *Server) handleDeleteChain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteChain(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleExecuteChain starts a chain execution against a target Sliver session.
func (s *Server) handleExecuteChain(w http.ResponseWriter, r *http.Request) {
	chainID := r.PathValue("id")
	rec, err := s.store.GetChain(chainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "chain not found")
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		DryRun    bool   `json:"dry_run"`
	}
	// An empty body is allowed: initial_access chains obtain their own session, so
	// no pre-existing session_id is required. readJSON tolerates an empty body.
	if r.ContentLength != 0 {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}
	// session_id is optional. When empty, steps without an explicit session_id have
	// no default target and must rely on an earlier initial_access step (via
	// session_id: "{{var}}"). Steps that need a session but have none will fail
	// individually, which surfaces clearly in the step logs.

	var ch chain.Chain
	if err := json.Unmarshal([]byte(rec.Data), &ch); err != nil {
		writeError(w, http.StatusInternalServerError, "corrupted chain data")
		return
	}

	if req.DryRun {
		// Validate the DAG and return the resolved order without executing
		order, err := chain.Resolve(ch.Steps)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		ids := make([]string, len(order))
		for i, s := range order {
			ids[i] = s.ID
		}
		writeJSON(w, http.StatusOK, map[string]any{"dry_run": true, "order": ids})
		return
	}

	executionID := uuid.NewString()
	execRec := store.ExecutionRecord{
		ID:        executionID,
		ChainID:   chainID,
		SessionID: req.SessionID,
		Status:    "running",
		StartedAt: time.Now(),
	}
	if err := s.store.CreateExecution(execRec); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.startExecution(ch, req.SessionID, executionID)

	writeJSON(w, http.StatusAccepted, map[string]string{"execution_id": executionID})
}

// decodeChain deserializes a Chain from the request body.
// It accepts both JSON (default) and YAML when Content-Type contains "yaml".
func decodeChain(r *http.Request, ch *chain.Chain) error {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "yaml") {
		if err := yaml.NewDecoder(r.Body).Decode(ch); err != nil {
			return fmt.Errorf("invalid YAML: %w", err)
		}
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(ch); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
