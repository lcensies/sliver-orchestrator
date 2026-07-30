package chain

import "fmt"

type markState int

const (
	unvisited markState = iota
	inProgress
	done
)

// Resolve validates a chain's step DAG and returns all steps in topological order.
// It returns an error if any step references an unknown dependency or if the graph
// contains a cycle.
func Resolve(steps []Step) ([]Step, error) {
	index := make(map[string]*Step, len(steps))
	for i := range steps {
		if steps[i].ID == "" {
			return nil, fmt.Errorf("step at index %d has an empty ID", i)
		}
		if _, exists := index[steps[i].ID]; exists {
			return nil, fmt.Errorf("duplicate step ID %q", steps[i].ID)
		}
		index[steps[i].ID] = &steps[i]
	}

	// Validate all dependency references (including inside any/all groups).
	for _, s := range steps {
		for _, id := range s.AllDepIDs() {
			if _, ok := index[id]; !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", s.ID, id)
			}
		}
	}

	marks := make(map[string]markState, len(steps))
	var order []Step

	var visit func(id string) error
	visit = func(id string) error {
		switch marks[id] {
		case done:
			return nil
		case inProgress:
			return fmt.Errorf("cycle detected at step %q", id)
		}
		marks[id] = inProgress
		for _, depID := range index[id].AllDepIDs() {
			if err := visit(depID); err != nil {
				return err
			}
		}
		marks[id] = done
		order = append(order, *index[id])
		return nil
	}

	for _, s := range steps {
		if err := visit(s.ID); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// ReadySteps returns IDs of steps that are not yet started/settled and whose
// every dependency entry has settled according to depSettled.
func ReadySteps(steps []Step, completed, failed, skipped map[string]bool) []string {
	var ready []string
	for _, s := range steps {
		if completed[s.ID] || failed[s.ID] || skipped[s.ID] {
			continue
		}
		if depsSettled(s.DependsOn, completed, failed, skipped) {
			ready = append(ready, s.ID)
		}
	}
	return ready
}

// depsSettled reports whether every dependency entry in deps has settled.
func depsSettled(deps []Dep, completed, failed, skipped map[string]bool) bool {
	for _, dep := range deps {
		if !depSettled(dep, completed, failed, skipped) {
			return false
		}
	}
	return true
}

// depSettled reports whether a single Dep entry has settled:
//
//   - Plain ID: settled when completed, failed, or skipped.
//   - {any:[...]}: settled eagerly when the first member completes, or when all
//     members have settled with none completing (hopeless — group will never satisfy).
//   - {all:[...]}: settled when every member has settled (same semantics as flat deps).
func depSettled(dep Dep, completed, failed, skipped map[string]bool) bool {
	if dep.ID != "" {
		return completed[dep.ID] || failed[dep.ID] || skipped[dep.ID]
	}

	if len(dep.Any) > 0 {
		// Settle eagerly as soon as one member completes.
		for _, id := range dep.Any {
			if completed[id] {
				return true
			}
		}
		// Settle (hopeless) when all members have settled without any completing.
		for _, id := range dep.Any {
			if !completed[id] && !failed[id] && !skipped[id] {
				return false // still in-flight
			}
		}
		return true
	}

	// {all:[...]} — every member must settle.
	for _, id := range dep.All {
		if !completed[id] && !failed[id] && !skipped[id] {
			return false
		}
	}
	return true
}
