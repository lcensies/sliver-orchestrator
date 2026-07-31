package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/bishopfox/sliver/scenario/chain"
	"github.com/bishopfox/sliver/scenario/store"
)

// seedScenarios discovers scenarios in dirs and upserts any whose serialized form
// changed since it was last seen (tracked in `seen`, keyed by chain id). This makes
// re-scans cheap and avoids touching unrelated chains (e.g. GUI-authored ones).
// Returns the number of definitions written this pass.
func seedScenarios(st *store.Store, dirs []string, seen map[string]string) int {
	discovered, derrs := chain.DiscoverScenarios(dirs)
	for _, e := range derrs {
		log.Printf("WARNING: scenario discovery: %v", e)
	}
	written := 0
	for _, ch := range discovered {
		if _, err := chain.Resolve(ch.Steps); err != nil {
			log.Printf("WARNING: skipping scenario %q (%s): invalid graph: %v", ch.Name, ch.ID, err)
			continue
		}
		data, _ := json.Marshal(ch)
		if seen[ch.ID] == string(data) {
			continue // unchanged
		}
		rec := store.ChainRecord{ID: ch.ID, Name: ch.Name, Description: ch.Description, Data: string(data)}
		if err := st.UpsertChain(rec); err != nil {
			log.Printf("WARNING: seeding scenario %q: %v", ch.ID, err)
			continue
		}
		seen[ch.ID] = string(data)
		written++
	}
	return written
}

// watchScenarios re-scans dirs every interval and seeds changed definitions, so
// edits on disk are picked up without a restart. Blocks; run in a goroutine.
func watchScenarios(st *store.Store, dirs []string, interval time.Duration, seen map[string]string) {
	for {
		time.Sleep(interval)
		if n := seedScenarios(st, dirs, seen); n > 0 {
			log.Printf("Scenario watch: re-seeded %d changed scenario(s)", n)
		}
	}
}
