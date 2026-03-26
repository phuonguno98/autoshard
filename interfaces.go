// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package autoshard

import (
	"context"
	"time"
)

// MemberInfo contains state information for a member in the cluster,
// returned by the Registry, used to check the Convergence Barrier.
type MemberInfo struct {
	ID               string
	PerceivedVersion int
}

// Registry defines the contract that Storage Adapters must implement.
// Ensure that no external dependencies (like sql.DB, redis.Client) leak into the Core.
type Registry interface {
	// Heartbeat updates the lifetime (last_heartbeat) of the Member.
	// At the same time, it updates the Member's current "view" of the cluster size (perceived_version)
	// to contribute to the Convergence Barrier.
	Heartbeat(ctx context.Context, memberID string, perceivedVersion int) error

	// GetActiveMembers retrieves the list of active members within the safe time window.
	// Returns a slice containing MemberInfo so the Partitioner can verify the Convergence state.
	GetActiveMembers(ctx context.Context, activeWindow time.Duration) ([]MemberInfo, error)

	// Deregister proactively removes the Member from the system upon receiving a SIGTERM signal.
	// This triggers Auto-rebalancing immediately without waiting for the Heartbeat to expire.
	Deregister(ctx context.Context, memberID string) error

	// StartGarbageCollector launches a background process (Goroutine) to periodically clean up Zombie Members.
	// It must return an error if it fails to initialize and should support graceful shutdown via context.
	StartGarbageCollector(ctx context.Context, memberID string, checkInterval, deadThreshold time.Duration) error
}

// JobID leverages Generics to support load partitioning across various ID types.
// These types will be hashed into uint64 before modulo is applied.
type JobID interface {
	~string | ~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64
}
