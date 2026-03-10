package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bishopfox/sliver/scenario/chain"
)

func (s *Server) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	chainID := r.URL.Query().Get("chain_id")
	records, err := s.store.ListExecutions(chainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	exec, err := s.store.GetExecution(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}
	logs, err := s.store.GetStepLogs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"execution": exec,
		"steps":     logs,
	})
}

// handleStreamExecution streams execution events as Server-Sent Events.
// It first replays all step logs already in the database, then delivers live
// events as they arrive.  Clients reconnecting after a disconnect can replay
// using the GET /executions/{id} endpoint.
func (s *Server) handleStreamExecution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	exec, err := s.store.GetExecution(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	flusher.Flush()

	ctx := r.Context()

	// Replay existing step logs first
	logs, _ := s.store.GetStepLogs(id)
	for _, l := range logs {
		sseWrite(w, "step_log", map[string]any{
			"step_id":  l.StepID,
			"status":   l.Status,
			"stdout":   l.Stdout,
			"stderr":   l.Stderr,
			"exit_code": l.ExitCode,
			"error":    l.Error,
		})
		flusher.Flush()
	}

	// If already finished, close the stream
	if exec.Status == "done" || exec.Status == "failed" || exec.Status == "cancelled" {
		sseWrite(w, "done", map[string]string{"status": exec.Status})
		flusher.Flush()
		return
	}

	// Subscribe to live events
	ch := s.exec.subscribe(id)
	defer s.exec.unsubscribe(id, ch)

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Keepalive comment to prevent proxy timeouts
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		case ev, open := <-ch:
			if !open {
				sseWrite(w, "done", map[string]string{"status": "closed"})
				flusher.Flush()
				return
			}
			if err := sseWriteEvent(w, ev); err != nil {
				return
			}
			flusher.Flush()
			if ev.Type == chain.EventChainDone || ev.Type == chain.EventChainFailed {
				return
			}
		}
	}
}

func (s *Server) handleCancelExecution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetExecution(id); err != nil {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}
	if !doCancel(id) {
		writeError(w, http.StatusConflict, "execution is not running or already cancelled")
		return
	}
	fin := time.Now()
	_ = s.store.UpdateExecutionStatus(id, "cancelled", "cancelled by user", &fin)
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// handleListSessions proxies GetSessions from the Sliver gRPC API.
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	resp, err := s.rpc.GetSessions(context.Background(), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "sliver GetSessions: "+err.Error())
		return
	}
	type session struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		OS       string `json:"os"`
		Hostname string `json:"hostname"`
		Username string `json:"username"`
		PID      uint32 `json:"pid"`
	}
	out := make([]session, 0, len(resp.Sessions))
	for _, sess := range resp.Sessions {
		out = append(out, session{
			ID:       sess.ID,
			Name:     sess.Name,
			OS:       sess.OS,
			Hostname: sess.Hostname,
			Username: sess.Username,
			PID:      uint32(sess.PID),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ── SSE helpers ───────────────────────────────────────────────────────────────

func sseWrite(w http.ResponseWriter, event string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(b))
}

func sseWriteEvent(w http.ResponseWriter, ev chain.Event) error {
	eventName := string(ev.Type)
	payload := map[string]any{"step_id": ev.StepID, "message": ev.Message}
	if ev.Result != nil {
		payload["stdout"]    = ev.Result.Stdout
		payload["stderr"]    = ev.Result.Stderr
		payload["exit_code"] = ev.Result.ExitCode
		payload["error"]     = ev.Result.Error
		payload["duration_ms"] = ev.Result.Duration.Milliseconds()
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, string(b))
	return err
}
