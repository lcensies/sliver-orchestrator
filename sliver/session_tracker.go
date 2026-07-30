package sliver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/rpcpb"
	"github.com/bishopfox/sliver/scenario/chain"
)

// sessionPollInterval is how often WaitForNewSession re-queries GetSessions.
const sessionPollInterval = 2 * time.Second

// snapshotSessionIDs returns the set of currently registered Sliver session IDs.
// It is called just before an initial-access module runs so a session that appears
// afterwards can be identified by diff.
func snapshotSessionIDs(ctx context.Context, rpc rpcpb.SliverRPCClient) (map[string]bool, error) {
	resp, err := rpc.GetSessions(ctx, &commonpb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("GetSessions: %w", err)
	}
	ids := make(map[string]bool, len(resp.GetSessions()))
	for _, s := range resp.GetSessions() {
		ids[s.ID] = true
	}
	return ids, nil
}

// waitForNewSession polls GetSessions until a session whose ID is not in `before`
// appears and satisfies the optional hostname/OS filters, or the context / wait
// timeout elapses. It returns the new session's UUID.
func waitForNewSession(ctx context.Context, rpc rpcpb.SliverRPCClient, before map[string]bool, wait chain.WaitSpec) (string, error) {
	deadline := time.Now().Add(wait.WaitTimeout())
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	ticker := time.NewTicker(sessionPollInterval)
	defer ticker.Stop()

	for {
		id, err := findNewSession(ctx, rpc, before, wait)
		if err != nil {
			return "", err
		}
		if id != "" {
			return id, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out after %s waiting for a new Sliver session (module ran but no beacon called back / matched filters)", wait.WaitTimeout())
		case <-ticker.C:
		}
	}
}

// findNewSession returns the first session ID not present in `before` that matches
// the wait filters, or "" if none is present yet.
func findNewSession(ctx context.Context, rpc rpcpb.SliverRPCClient, before map[string]bool, wait chain.WaitSpec) (string, error) {
	resp, err := rpc.GetSessions(ctx, &commonpb.Empty{})
	if err != nil {
		return "", fmt.Errorf("GetSessions: %w", err)
	}
	for _, s := range resp.GetSessions() {
		if before[s.ID] {
			continue
		}
		if wait.MatchHostname != "" && !strings.Contains(s.Hostname, wait.MatchHostname) {
			continue
		}
		if wait.MatchOS != "" && !strings.Contains(strings.ToLower(s.OS), strings.ToLower(wait.MatchOS)) {
			continue
		}
		return s.ID, nil
	}
	return "", nil
}
