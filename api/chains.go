package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/bishopfox/sliver/scenario/chain"
	"github.com/bishopfox/sliver/scenario/store"
)

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
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

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
