// MIT License
//
// Copyright (c) 2026 phuonguno98
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package autoshard

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Partitioner distributes jobs across active members using modulo hashing
// and a zero-communication consensus barrier (perceived_version).
// It maintains thread-safe access to indices and states.
type Partitioner struct {
	memberID string
	registry Registry
	opts     *Options

	mu           sync.RWMutex // Protects myIndex, totalMembers, and isConverged
	myIndex      int
	totalMembers int
	isConverged  bool
	shutdownOnce sync.Once
}

// NewPartitioner creates a new Partitioner instance to be used by the Member.
// Returns an error if memberID is empty or registry is nil (Defensive Programming).
func NewPartitioner(memberID string, registry Registry, opts ...Option) (*Partitioner, error) {
	if memberID == "" {
		return nil, fmt.Errorf("autoshard: memberID must not be empty")
	}
	if registry == nil {
		return nil, fmt.Errorf("autoshard: registry must not be nil")
	}

	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	return &Partitioner{
		memberID:     memberID,
		registry:     registry,
		opts:         options,
		mu:           sync.RWMutex{},
		myIndex:      -1,
		totalMembers: 0,
		isConverged:  false,
		shutdownOnce: sync.Once{},
	}, nil
}

// Sync performs the State Machine logic for the Convergence Barrier.
// 1. Fetches active members and counts them.
// 2. Updates its own perceived version in the datastore.
// 3. Checks if all active members share the same perspective of the cluster.
// 4. If converged, sorts member IDs lexicographically to determine its deterministic Job index.
// 5. If not converged, enters Asymmetric Safe Mode (retains old state to avoid interruption).
func (p *Partitioner) Sync(ctx context.Context) error {
	// 1. Fetch active members
	members, err := p.registry.GetActiveMembers(ctx, p.opts.ActiveWindow)
	if err != nil {
		// Fail-open: Asymmetric Safe Mode prevents resetting state purely on temporary DB error
		return fmt.Errorf("fetch active members from registry: %w", err)
	}

	// 2. Count actual active members (N)
	actualCount := len(members)

	// 3. Update own perceived version in datastore via Heartbeat
	err = p.registry.Heartbeat(ctx, p.memberID, actualCount)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}

	// 4. Barrier Check: Verify if all members see the identical number of members
	converged := true
	var activeIDs []string

	for _, m := range members {
		activeIDs = append(activeIDs, m.ID)
		if m.ID == p.memberID {
			continue // I just updated myself via Heartbeat, so I inherently agree with actualCount
		}
		if m.PerceivedVersion != actualCount {
			converged = false
		}
	}

	// 5. State Machine Local Update (RAM)
	p.mu.Lock()
	p.isConverged = converged
	if converged {
		// Sort lexicographically to ensure stable indices across the cluster uniformly
		sort.Strings(activeIDs)

		// Find own index
		foundIndex := -1
		for i, id := range activeIDs {
			if id == p.memberID {
				foundIndex = i
				break
			}
		}

		if foundIndex != -1 {
			p.myIndex = foundIndex
			p.totalMembers = actualCount
		}
	} else {
		// Asymmetric Safe Mode
		// If totalMembers == 0 (newly started member), it remains 0 (waiting).
		// If totalMembers > 0 (existing member), it keeps old state and index intact (serving old jobs).
	}
	
	total := p.totalMembers
	index := p.myIndex
	isConv := p.isConverged
	p.mu.Unlock()

	// Trigger State Change Hook outside of lock
	if p.opts.OnStateChange != nil {
		p.opts.OnStateChange(isConv, total, index)
	}

	return nil
}

// IsMyJob determines if a given job belongs to this member using Modulo hashing.
// Built with Golang Generics to accept any integer or string-like type gracefully.
// Note: Implemented as a package-level function because Go does not support generic methods.
func IsMyJob[T JobID](p *Partitioner, jobID T) bool {
	p.mu.RLock()
	total := p.totalMembers
	index := p.myIndex
	hasher := p.opts.Hasher
	p.mu.RUnlock()

	// If the member has just started and the cluster has never converged at least once
	if total == 0 {
		return false
	}

	var hashVal uint64
	switch v := any(jobID).(type) {
	case string:
		hashVal = hasher([]byte(v))
	case int:
		hashVal = uint64(v)
	case int32:
		hashVal = uint64(v)
	case int64:
		hashVal = uint64(v)
	case uint:
		hashVal = uint64(v)
	case uint32:
		hashVal = uint64(v)
	case uint64:
		hashVal = v
	}

	return int(hashVal%uint64(total)) == index
}

// Shutdown gracefully stops the partitioner by deregistering itself from the registry.
// Triggering an immediate cluster Auto-rebalancing without waiting for GC delays.
// It uses sync.Once to ensure deregistration is performed exactly one time.
func (p *Partitioner) Shutdown(ctx context.Context) error {
	var err error
	p.shutdownOnce.Do(func() {
		if deregErr := p.registry.Deregister(ctx, p.memberID); deregErr != nil {
			err = fmt.Errorf("deregister member: %w", deregErr)
		}
	})
	return err
}

// Options exposes the applied Options.
func (p *Partitioner) Options() *Options {
	return p.opts
}

// PartitionerStatus holds a read-only snapshot of the Partitioner's internal state.
type PartitionerStatus struct {
	MemberID     string
	MyIndex      int
	TotalMembers int
	IsConverged  bool
}

// Status returns a thread-safe snapshot of the current partitioning state.
func (p *Partitioner) Status() PartitionerStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return PartitionerStatus{
		MemberID:     p.memberID,
		MyIndex:      p.myIndex,
		TotalMembers: p.totalMembers,
		IsConverged:  p.isConverged,
	}
}

// RunSyncLoop is a convenience function that loops continuously in a goroutine
// to perform the Sync operation according to the Partitioner's configured SyncInterval.
// It leverages Jitter (if enabled) to protect the database layer from Thundering Herd.
func (p *Partitioner) RunSyncLoop(ctx context.Context) {
	interval := p.opts.GetJitteredInterval(p.opts.SyncInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Perform initial fast sync
	_ = p.Sync(ctx)

	for {
		select {
		case <-ctx.Done():
			// Automatically attempt a graceful deregistration upon context cancellation.
			// We use a dedicated timeout context to avoid hanging the process if the DB is unresponsive.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_ = p.Shutdown(shutdownCtx)
			return
		case <-ticker.C:
			if err := p.Sync(ctx); err != nil && p.opts.OnSyncError != nil {
				p.opts.OnSyncError(err)
			}
			// Reset ticker with a new jittered interval for dynamic distribution
			ticker.Reset(p.opts.GetJitteredInterval(p.opts.SyncInterval))
		}
	}
}
