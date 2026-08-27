package api

import (
	"net/http"
	"sort"

	"github.com/bishopfox/sliver/scenario/atomic"
)

// cleanupCommandOf returns the test's cleanup command, preferring the top-level
// ART field and falling back to the executor's. ART files put it in either
// place depending on vintage, and a consumer that skips VM reverts relies on it
// to restore state, so both spellings must resolve.
func cleanupCommandOf(t atomic.Test) string {
	if t.CleanupCommand != "" {
		return t.CleanupCommand
	}
	if t.Executor != nil {
		return t.Executor.CleanupCommand
	}
	return ""
}

func (s *Server) handleListAtomics(w http.ResponseWriter, r *http.Request) {
	tactic := r.URL.Query().Get("tactic")
	platform := r.URL.Query().Get("platform")

	techniques := s.atomics.Filter(tactic, platform)

	// Platforms is the test's OWN supported_platforms — distinct from the
	// technique-level Platforms below, which is their UNION. A technique passes
	// a ?platform=linux filter if ANY of its tests is linux, so a consumer that
	// wants only the linux tests of a mixed-OS technique (T1082 spans
	// linux/macos/windows) must re-filter per test on this field; without it a
	// linux sweep also dispatches that technique's Windows tests.
	// CleanupCommand lets a consumer restore state without a VM revert.
	type testSummary struct {
		Index          int      `json:"index"`
		Name           string   `json:"name"`
		Platforms      []string `json:"platforms,omitempty"`
		CleanupCommand string   `json:"cleanup_command,omitempty"`
	}
	type item struct {
		ID          string        `json:"id"`
		DisplayName string        `json:"display_name"`
		Tactic      string        `json:"tactic"`
		Platforms   []string      `json:"platforms"`
		Tests       []testSummary `json:"tests"`
	}

	out := make([]item, 0, len(techniques))
	for _, t := range techniques {
		tests := make([]testSummary, len(t.Tests))
		for i, test := range t.Tests {
			tests[i] = testSummary{
				Index:          i,
				Name:           test.Name,
				Platforms:      test.SupportedPlatforms,
				CleanupCommand: cleanupCommandOf(test),
			}
		}
		out = append(out, item{
			ID:          t.ID,
			DisplayName: t.DisplayName,
			Tactic:      t.Tactic,
			Platforms:   t.Platforms,
			Tests:       tests,
		})
	}

	// Stable sort by technique ID
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetAtomic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, ok := s.atomics.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "technique not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, t)
}
