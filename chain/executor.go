package chain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bishopfox/sliver/scenario/store"
)

// StepStatus describes the lifecycle state of a single step during execution.
type StepStatus string

const (
	StatusPending  StepStatus = "pending"
	StatusRunning  StepStatus = "running"
	StatusDone     StepStatus = "done"
	StatusFailed   StepStatus = "failed"
	StatusSkipped  StepStatus = "skipped"
)

// StepResult is the output of a completed (or failed/skipped) step.
type StepResult struct {
	StepID   string
	Status   StepStatus
	Stdout   string
	Stderr   string
	ExitCode int
	Error    string
	Duration time.Duration
}

// EventType identifies what happened at a particular point in execution.
type EventType string

const (
	EventStepStart    EventType = "step_start"
	EventStepDone     EventType = "step_done"
	EventStepFailed   EventType = "step_failed"
	EventStepSkipped  EventType = "step_skipped"
	EventChainDone    EventType = "chain_done"
	EventChainFailed  EventType = "chain_failed"
)

// Event is emitted by the Executor for each significant state change.
// Consumers (e.g. SSE handlers) subscribe to the Events() channel.
type Event struct {
	Type    EventType
	StepID  string
	Result  *StepResult
	Message string
}

// StepExecutor is the interface the chain Executor uses to run individual step actions.
// Implementations handle the actual remote communication (e.g. via Sliver gRPC).
type StepExecutor interface {
	Execute(ctx context.Context, sessionID string, action Action) (stdout, stderr string, exitCode int, err error)
}

// AtomicResolver resolves an AtomicRef into a CommandAction before dispatch.
// The test is located by GUID > Name > Test index — matching GoART priority.
type AtomicResolver interface {
	Resolve(techniqueID, name, guid string, testIdx int, args map[string]string) (interpreter, command string, err error)
}

// Executor runs a Chain against a target Sliver session, emitting Events
// for each step transition and persisting results to the store.
type Executor struct {
	stepExec StepExecutor
	atomics  AtomicResolver
	store    *store.Store
	events   chan Event
}

// NewExecutor creates an Executor.  atomics may be nil (atomic steps will fail gracefully).
func NewExecutor(stepExec StepExecutor, atomics AtomicResolver, st *store.Store) *Executor {
	return &Executor{
		stepExec: stepExec,
		atomics:  atomics,
		store:    st,
		events:   make(chan Event, 256),
	}
}

// Events returns a read-only channel of execution events.
// The channel is closed when Run returns.
func (e *Executor) Events() <-chan Event {
	return e.events
}

// Run executes chain against sessionID, recording all progress under executionID.
// It blocks until all steps have completed (or are aborted/cancelled) and then
// closes the Events() channel.
func (e *Executor) Run(ctx context.Context, ch Chain, sessionID, executionID string) error {
	defer close(e.events)

	if _, err := Resolve(ch.Steps); err != nil {
		return fmt.Errorf("invalid chain: %w", err)
	}

	stepIndex := make(map[string]Step, len(ch.Steps))
	for _, s := range ch.Steps {
		stepIndex[s.ID] = s
	}

	var mu sync.Mutex
	vars := make(VarMap)
	completed := make(map[string]bool)   // successfully finished
	failed := make(map[string]bool)     // step failed (used for dependency settlement)
	failedOptional := make(map[string]bool) // failed with on_fail: continue_no_err — do not fail chain
	skipped := make(map[string]bool)
	scheduled := make(map[string]bool) // dispatched to a goroutine

	abortCh := make(chan struct{})
	var abortOnce sync.Once
	abort := func() { abortOnce.Do(func() { close(abortCh) }) }

	var wg sync.WaitGroup

	for {
		// Check for external cancellation or abort signal
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-abortCh:
			wg.Wait()
			return fmt.Errorf("chain aborted")
		default:
		}

		mu.Lock()
		total := len(ch.Steps)
		settled := len(completed) + len(failed) + len(skipped)

		if settled >= total {
			mu.Unlock()
			break
		}

		ready := ReadySteps(ch.Steps, completed, failed, skipped)
		var toRun []string
		for _, id := range ready {
			if !scheduled[id] {
				scheduled[id] = true
				toRun = append(toRun, id)
			}
		}

		// Detect deadlock: nothing left to schedule and not all steps settled.
		if len(toRun) == 0 {
			remaining := total - settled
			if remaining > 0 {
				// Guard: if any goroutines are still in-flight (scheduled but not yet
				// settled), we are not in a true deadlock — we are just waiting for an
				// {any:[...]} group member to complete.  Let the loop yield and retry.
				inFlight := false
				for id := range scheduled {
					if !completed[id] && !failed[id] && !skipped[id] {
						inFlight = true
						break
					}
				}
				if !inFlight {
					// True deadlock: all deps for remaining steps have settled with no
					// hope of completion.  Mark them skipped to unblock the chain.
					for _, s := range ch.Steps {
						if !completed[s.ID] && !failed[s.ID] && !skipped[s.ID] && !scheduled[s.ID] {
							skipped[s.ID] = true
							e.emit(Event{Type: EventStepSkipped, StepID: s.ID, Message: "dependency failed or skipped"})
							e.logStep(executionID, s.ID, string(StatusSkipped), "", "", 0, "dependency failed or skipped", 0)
						}
					}
				}
				mu.Unlock()
				continue
			}
			mu.Unlock()
			break
		}
		mu.Unlock()

		for _, id := range toRun {
			step := stepIndex[id]
			wg.Add(1)
			go func(s Step) {
				defer wg.Done()
				e.runStep(ctx, s, sessionID, executionID, &mu, vars, completed, failed, failedOptional, skipped, abort, abortCh)
			}(step)
		}

		// Yield briefly so goroutines can make progress before we loop
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case <-abortCh:
			wg.Wait()
			return fmt.Errorf("chain aborted")
		case <-time.After(50 * time.Millisecond):
		}
	}

	wg.Wait()

	// Only treat as chain failure if at least one step failed without continue_no_err.
	var fatalFailed []string
	for id := range failed {
		if !failedOptional[id] {
			fatalFailed = append(fatalFailed, id)
		}
	}
	if len(fatalFailed) > 0 {
		msg := fmt.Sprintf("%d step(s) failed: %s", len(fatalFailed), strings.Join(fatalFailed, ", "))
		e.emit(Event{Type: EventChainFailed, Message: msg})
		return errors.New(msg)
	}

	e.emit(Event{Type: EventChainDone, Message: "all steps completed"})
	return nil
}

func (e *Executor) runStep(
	ctx context.Context,
	s Step,
	sessionID, executionID string,
	mu *sync.Mutex,
	vars VarMap,
	completed, failed, failedOptional, skipped map[string]bool,
	abort func(),
	abortCh <-chan struct{},
) {
	// Snapshot variables under the lock
	mu.Lock()
	localVars := make(VarMap, len(vars))
	for k, v := range vars {
		localVars[k] = v
	}
	mu.Unlock()

	// Evaluate conditions — skip (not fail) on false
	condOk, condErr := EvalConditions(s.Conditions, localVars, 0)
	if condErr != nil || !condOk {
		reason := "conditions not met"
		if condErr != nil {
			reason = condErr.Error()
		}
		mu.Lock()
		skipped[s.ID] = true
		mu.Unlock()
		e.emit(Event{Type: EventStepSkipped, StepID: s.ID, Message: reason})
		e.logStep(executionID, s.ID, string(StatusSkipped), "", "", 0, reason, 0)
		return
	}

	// Substitute variables into the action
	action := SubstituteAction(s.Action, localVars)

	// Resolve atomic refs into a concrete CommandAction before dispatch
	if action.Type == ActionAtomic {
		resolved, err := e.resolveAtomic(action.AtomicRef)
		if err != nil {
			e.handleFailure(s, executionID, "", "", 1, err.Error(), 0, mu, failed, failedOptional, skipped, abort)
			return
		}
		action = resolved
	}

	e.emit(Event{Type: EventStepStart, StepID: s.ID})
	e.logStep(executionID, s.ID, string(StatusRunning), "", "", 0, "", 0)

	stepCtx, cancel := context.WithTimeout(ctx, s.TimeoutDuration())
	defer cancel()

	start := time.Now()
	stdout, stderr, exitCode, execErr := e.stepExec.Execute(stepCtx, sessionID, action)
	dur := time.Since(start)

	if execErr != nil || exitCode != 0 {
		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		}
		e.handleFailure(s, executionID, stdout, stderr, exitCode, errMsg, dur, mu, failed, failedOptional, skipped, abort)
		return
	}

	mu.Lock()
	completed[s.ID] = true
	if s.OutputVar != "" {
		val := stdout
		if filtered, ok := ApplyFilter(stdout, s.OutputFilter); ok {
			val = filtered
		}
		vars[s.OutputVar] = strings.TrimRight(val, "\r\n")
	}
	for k, v := range ExtractCaptures(stdout, s.OutputExtract) {
		vars[k] = v
	}
	mu.Unlock()

	result := &StepResult{
		StepID:   s.ID,
		Status:   StatusDone,
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
		Duration: dur,
	}
	e.emit(Event{Type: EventStepDone, StepID: s.ID, Result: result})
	e.logStep(executionID, s.ID, string(StatusDone), stdout, stderr, exitCode, "", dur.Milliseconds())
}

func (e *Executor) handleFailure(
	s Step,
	executionID, stdout, stderr string,
	exitCode int,
	errMsg string,
	dur time.Duration,
	mu *sync.Mutex,
	failed, failedOptional, skipped map[string]bool,
	abort func(),
) {
	mu.Lock()
	failed[s.ID] = true
	if s.FailurePolicy() == FailContinueOK {
		failedOptional[s.ID] = true
	}
	mu.Unlock()

	result := &StepResult{
		StepID:   s.ID,
		Status:   StatusFailed,
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
		Error:    errMsg,
		Duration: dur,
	}
	e.emit(Event{Type: EventStepFailed, StepID: s.ID, Result: result})
	e.logStep(executionID, s.ID, string(StatusFailed), stdout, stderr, exitCode, errMsg, dur.Milliseconds())

	switch s.FailurePolicy() {
	case FailAbort:
		abort()
	case FailSkipDependents:
		// The main loop will detect the step is in failed[] and skip its dependents
	}
}

func (e *Executor) resolveAtomic(ref *AtomicRef) (Action, error) {
	if ref == nil {
		return Action{}, fmt.Errorf("nil atomic ref")
	}
	if e.atomics == nil {
		return Action{}, fmt.Errorf("no atomic resolver configured")
	}
	interp, cmd, err := e.atomics.Resolve(ref.ID, ref.Name, ref.GUID, ref.Test, ref.Args)
	if err != nil {
		return Action{}, fmt.Errorf("resolving atomic %s (name=%q guid=%q idx=%d): %w",
			ref.ID, ref.Name, ref.GUID, ref.Test, err)
	}
	return Action{
		Type: ActionCommand,
		Command: &CommandAction{
			Interpreter: interp,
			Cmd:         cmd,
		},
	}, nil
}

func (e *Executor) emit(ev Event) {
	select {
	case e.events <- ev:
	default:
		// Drop event if consumer is not keeping up; step logs in SQLite are authoritative
	}
}

func (e *Executor) logStep(executionID, stepID, status, stdout, stderr string, exitCode int, errMsg string, durationMs int64) {
	if e.store == nil {
		return
	}
	_ = e.store.LogStep(executionID, stepID, status, stdout, stderr, exitCode, errMsg, durationMs)
}
