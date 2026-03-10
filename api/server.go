// Package api implements the scenario orchestrator REST API.
// It uses Go 1.22+ net/http.ServeMux with method+pattern routing (no external router needed).
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/bishopfox/sliver/protobuf/rpcpb"
	"github.com/bishopfox/sliver/scenario/atomic"
	"github.com/bishopfox/sliver/scenario/chain"
	"github.com/bishopfox/sliver/scenario/sliver"
	"github.com/bishopfox/sliver/scenario/store"
)

// ExecManager tracks running executions so SSE clients can subscribe to events.
type ExecManager struct {
	mu    sync.RWMutex
	chans map[string][]chan chain.Event // executionID → subscriber channels
}

func newExecManager() *ExecManager {
	return &ExecManager{chans: make(map[string][]chan chain.Event)}
}

// subscribe registers a new subscriber channel for executionID.
func (m *ExecManager) subscribe(executionID string) chan chain.Event {
	ch := make(chan chain.Event, 128)
	m.mu.Lock()
	m.chans[executionID] = append(m.chans[executionID], ch)
	m.mu.Unlock()
	return ch
}

// unsubscribe removes a subscriber channel for executionID.
func (m *ExecManager) unsubscribe(executionID string, ch chan chain.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subs := m.chans[executionID]
	for i, s := range subs {
		if s == ch {
			m.chans[executionID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(m.chans[executionID]) == 0 {
		delete(m.chans, executionID)
	}
}

// broadcast sends an event to all subscribers for executionID.
func (m *ExecManager) broadcast(executionID string, ev chain.Event) {
	m.mu.RLock()
	subs := m.chans[executionID]
	m.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Server is the HTTP API server.
type Server struct {
	store   *store.Store
	atomics *atomic.Library
	rpc     rpcpb.SliverRPCClient
	cfgPath string // Sliver operator config path, forwarded to python steps
	c2Host  string // C2 host used as beacon callback address
	exec    *ExecManager
	cors    string
}

// NewServer creates a Server.
// cfgPath is the Sliver operator .cfg file path; it is injected into python step
// execution so scripts can use sliver-py without additional configuration.
// c2Host is the IP/hostname the scenario server advertises to beacon implants;
// leave empty to fall back to the C2_HOST environment variable.
func NewServer(st *store.Store, atomics *atomic.Library, rpc rpcpb.SliverRPCClient, cfgPath, c2Host, allowOrigin string) *Server {
	if c2Host == "" {
		c2Host = c2HostFromEnv()
	}
	return &Server{
		store:   st,
		atomics: atomics,
		rpc:     rpc,
		cfgPath: cfgPath,
		c2Host:  c2Host,
		exec:    newExecManager(),
		cors:    allowOrigin,
	}
}

// Handler builds and returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)

	// Sessions (proxied from Sliver)
	mux.HandleFunc("GET /api/v1/sessions", s.handleListSessions)

	// Implant delivery — generates a Sliver beacon on demand and serves the binary
	mux.HandleFunc("GET /api/v1/implant/linux", s.handleGetImplantLinux)

	// Atomics
	mux.HandleFunc("GET /api/v1/atomics", s.handleListAtomics)
	mux.HandleFunc("GET /api/v1/atomics/{id}", s.handleGetAtomic)

	// Chains
	mux.HandleFunc("GET /api/v1/chains", s.handleListChains)
	mux.HandleFunc("POST /api/v1/chains", s.handleCreateChain)
	mux.HandleFunc("GET /api/v1/chains/{id}", s.handleGetChain)
	mux.HandleFunc("PUT /api/v1/chains/{id}", s.handleUpdateChain)
	mux.HandleFunc("DELETE /api/v1/chains/{id}", s.handleDeleteChain)

	// Execute
	mux.HandleFunc("POST /api/v1/chains/{id}/execute", s.handleExecuteChain)

	// Executions
	mux.HandleFunc("GET /api/v1/executions", s.handleListExecutions)
	mux.HandleFunc("GET /api/v1/executions/{id}", s.handleGetExecution)
	mux.HandleFunc("GET /api/v1/executions/{id}/stream", s.handleStreamExecution)
	mux.HandleFunc("POST /api/v1/executions/{id}/cancel", s.handleCancelExecution)

	return s.corsMiddleware(mux)
}

// corsMiddleware adds CORS headers to every response.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.cors)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// cancelFuncs stores per-execution cancel functions so we can abort from the API.
var (
	cancelMu    sync.Mutex
	cancelFuncs = map[string]context.CancelFunc{}
)

func registerCancel(id string, cancel context.CancelFunc) {
	cancelMu.Lock()
	cancelFuncs[id] = cancel
	cancelMu.Unlock()
}

func doCancel(id string) bool {
	cancelMu.Lock()
	cancel, ok := cancelFuncs[id]
	cancelMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// handleHealth returns a trivial liveness response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

// startExecution launches chain execution in a goroutine and wires up event broadcasting.
func (s *Server) startExecution(ch chain.Chain, sessionID, executionID string) {
	stepExec := sliver.NewExecutor(s.rpc, s.cfgPath)
	exec := chain.NewExecutor(stepExec, s.atomics, s.store)

	ctx, cancel := context.WithCancel(context.Background())
	registerCancel(executionID, cancel)

	go func() {
		defer cancel()

		// Relay events to all SSE subscribers
		go func() {
			for ev := range exec.Events() {
				s.exec.broadcast(executionID, ev)
			}
			// Signal that the stream is done by closing all subscriber channels
			s.exec.mu.Lock()
			for _, subs := range s.exec.chans {
				for _, ch := range subs {
					select {
					case ch <- chain.Event{Type: chain.EventChainDone}:
					default:
					}
				}
			}
			s.exec.mu.Unlock()
		}()

		now := time.Now()
		err := exec.Run(ctx, ch, sessionID, executionID)

		status := "done"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
		}
		fin := time.Now()
		_ = s.store.UpdateExecutionStatus(executionID, status, errMsg, &fin)
		_ = now // suppress unused warning
	}()
}
