// MIT License
//
// Copyright (c) 2026 phuonguno98
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package memory

import (
	"context"
	"sync"
	"time"

	"github.com/phuonguno98/autoshard"
)

// memberRecord stores internal state for the memory registry.
type memberRecord struct {
	PerceivedVersion int
	LastHeartbeat    time.Time
}

// Registry implements autoshard.Registry using a thread-safe memory map.
// It is intended for Unit Testing and single-node setups.
type Registry struct {
	mu      sync.RWMutex
	members map[string]memberRecord
}

// NewRegistry creates a new in-memory Registry ready for use.
func NewRegistry() (*Registry, error) {
	return &Registry{
		members: make(map[string]memberRecord),
	}, nil
}

// Heartbeat updates the lifetime and convergence view (perceivedVersion) of a member.
func (r *Registry) Heartbeat(ctx context.Context, memberID string, perceivedVersion int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.members[memberID] = memberRecord{
		PerceivedVersion: perceivedVersion,
		LastHeartbeat:    time.Now(),
	}
	return nil
}

// GetActiveMembers retrieves the list of active members within the defined ActiveWindow.
func (r *Registry) GetActiveMembers(ctx context.Context, activeWindow time.Duration) ([]autoshard.MemberInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var active []autoshard.MemberInfo
	now := time.Now()

	for id, record := range r.members {
		// Only select members whose LastHeartbeat is within the activeWindow threshold
		if now.Sub(record.LastHeartbeat) <= activeWindow {
			active = append(active, autoshard.MemberInfo{
				ID:               id,
				PerceivedVersion: record.PerceivedVersion,
			})
		}
	}

	return active, nil
}

// Deregister proactively removes a member to trigger immediate rebalancing.
func (r *Registry) Deregister(ctx context.Context, memberID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.members, memberID)
	return nil
}

// StartGarbageCollector runs a background goroutine to physically drop dead members.
func (r *Registry) StartGarbageCollector(ctx context.Context, memberID string, checkInterval, deadThreshold time.Duration) error {
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.mu.Lock()
				now := time.Now()
				for id, record := range r.members {
					if now.Sub(record.LastHeartbeat) > deadThreshold {
						delete(r.members, id)
					}
				}
				r.mu.Unlock()
			}
		}
	}()
	return nil
}
