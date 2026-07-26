// Package initialaccess provides a pluggable framework for gaining an initial
// Sliver session on a target host ("initial access" / payload delivery).
//
// A Module exploits a target and installs a Sliver beacon by whatever means it
// likes (a Metasploit run, a custom exploit script, an SSH drop, ...). It does
// NOT need to know Sliver's session UUID — after a module reports success the
// caller (the sliver executor) correlates the newly registered Sliver session by
// diffing GetSessions. This keeps the module contract framework-agnostic.
//
// Modules are registered in a process-wide registry by name and referenced from a
// scenario's initial_access step (action.initial_access.module). One module ships
// built-in: "external" (see external.go), which shells out to an arbitrary
// executable, so Metasploit and custom exploit scripts work without recompiling.
package initialaccess

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Target is a host a module attacks. Attrs carries arbitrary module-specific
// key/values declared in the scenario's targets block.
type Target struct {
	Name  string            `json:"name"`
	Host  string            `json:"host"`
	Port  int               `json:"port"`
	Attrs map[string]string `json:"attrs"`
}

// Request is passed to a module's Run. For the built-in external module it is
// serialized to JSON and written to the child process's stdin.
type Request struct {
	Target Target `json:"target"`
	// Config is the module-specific parameter map from the scenario (already
	// template-substituted by the chain executor).
	Config map[string]string `json:"config"`
}

// Result reports the outcome of a module run. The optional correlation hints let
// the caller narrow which new Sliver session belongs to this breach when several
// could appear concurrently.
type Result struct {
	// Ok indicates the exploit succeeded and a beacon should be calling back.
	Ok bool `json:"ok"`
	// Note is a human-readable status line surfaced in step logs.
	Note string `json:"note"`
	// Hostname, when set, is a correlation hint for the expected new session.
	Hostname string `json:"hostname"`
}

// Module is the extensibility contract for an initial-access mechanism.
type Module interface {
	// Name is the registry key referenced from a scenario (initial_access.module).
	Name() string
	// Run attempts to breach req.Target and install a beacon. Returning a Result
	// with Ok=false (or an error) means no session should be expected.
	Run(ctx context.Context, req Request) (Result, error)
}

// Registry maps module names to implementations. The zero value is not usable;
// use NewRegistry or the package-level Default registry.
type Registry struct {
	mu      sync.RWMutex
	modules map[string]Module
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{modules: make(map[string]Module)}
}

// Register adds (or replaces) a module keyed by its Name.
func (r *Registry) Register(m Module) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modules[m.Name()] = m
}

// Get returns the module registered under name.
func (r *Registry) Get(name string) (Module, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.modules[name]
	if !ok {
		return nil, fmt.Errorf("unknown initial-access module %q; registered: %v", name, r.names())
	}
	return m, nil
}

// Names returns the sorted list of registered module names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.names()
}

func (r *Registry) names() []string {
	out := make([]string, 0, len(r.modules))
	for n := range r.modules {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// DefaultRegistry returns a registry pre-populated with the built-in modules.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&ExternalModule{})
	r.Register(&MetasploitModule{})
	return r
}
