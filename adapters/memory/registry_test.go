// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package memory

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRegistry(t *testing.T) {
	ctx := context.Background()
	reg, _ := NewRegistry()

	memberID := "node-1"

	// Test Heartbeat
	err := reg.Heartbeat(ctx, memberID, 1)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	// Test GetActiveMembers
	members, err := reg.GetActiveMembers(ctx, 10*time.Second)
	if err != nil {
		t.Fatalf("GetActiveMembers failed: %v", err)
	}

	if len(members) != 1 || members[0].ID != memberID {
		t.Errorf("Expected 1 member with ID %s, got %v", memberID, members)
	}

	// Test Deregister
	err = reg.Deregister(ctx, memberID)
	if err != nil {
		t.Fatalf("Deregister failed: %v", err)
	}

	_, _ = reg.GetActiveMembers(ctx, 10*time.Second)
	if len(reg.members) != 0 {
		t.Errorf("Expected 0 members after deregister, got %d", len(reg.members))
	}
}

func TestMemoryRegistry_GarbageCollector(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg, _ := NewRegistry()

	// Record an old heartbeat (manual insertion if possible, or just wait)
	reg.mu.Lock()
	reg.members["old-node"] = memberRecord{
		PerceivedVersion: 1,
		LastHeartbeat:    time.Now().Add(-1 * time.Minute),
	}
	reg.mu.Unlock()

	// Start GC
	_ = reg.StartGarbageCollector(ctx, "caller", 100*time.Millisecond, 500*time.Millisecond)

	// Wait for GC cycle
	time.Sleep(300 * time.Millisecond)

	_, _ = reg.GetActiveMembers(ctx, 10*time.Second)

	// The member should be removed by GC
	reg.mu.RLock()
	_, exists := reg.members["old-node"]
	reg.mu.RUnlock()

	if exists {
		t.Errorf("GC failed to remove dead member")
	}
}
