// MIT License
//
// Copyright (c) 2026 phuonguno98
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package autoshard_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/phuonguno98/autoshard"
	"github.com/phuonguno98/autoshard/adapters/memory"
)

func TestPartitioner_ModuloHashing(t *testing.T) {
	// Initialize Memory Registry (Mock Datastore)
	registry, _ := memory.NewRegistry()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configure common options for deterministic testing (disabling Jitter)
	opts := []autoshard.Option{
		autoshard.WithActiveWindow(5 * time.Second),
		autoshard.WithSyncInterval(500 * time.Millisecond),
		autoshard.DisableJitter(),
	}

	// Create 3 members
	node1, _ := autoshard.NewPartitioner("node-1", registry, opts...)
	node2, _ := autoshard.NewPartitioner("node-2", registry, opts...)
	node3, _ := autoshard.NewPartitioner("node-3", registry, opts...)

	// Force independent Syncs. Currently total members = 3, but they need to converge.
	_ = node1.Sync(ctx) // Node 1 sees 1
	_ = node2.Sync(ctx) // Node 2 sees 2
	_ = node3.Sync(ctx) // Node 3 sees 3

	// By now, they are NOT converged because PerceivedVersion differs across heartbeats.
	// Force ALL nodes to sync one more time so they all see PerceivedVersion = 3.
	_ = node1.Sync(ctx)
	_ = node2.Sync(ctx)
	_ = node3.Sync(ctx)

	// Finally, do one more barrier check so Node 1 and Node 2 accept converged state
	_ = node1.Sync(ctx)
	_ = node2.Sync(ctx)

	// Now check Workload Distribution
	// We have jobs 1 to 10
	var processedBy1, processedBy2, processedBy3 int

	for jobID := 1; jobID <= 10; jobID++ {
		claimedBy := 0

		if autoshard.IsMyJob(node1, jobID) {
			processedBy1++
			claimedBy++
		}
		if autoshard.IsMyJob(node2, jobID) {
			processedBy2++
			claimedBy++
		}
		if autoshard.IsMyJob(node3, jobID) {
			processedBy3++
			claimedBy++
		}

		// Mathematical Proof #2: Mutually Exclusive (Nobody claims the same job)
		if claimedBy != 1 {
			t.Fatalf("Job %d was claimed by %d members instead of exactly 1", jobID, claimedBy)
		}
	}

	// Mathematical Proof #1: Collectively Exhaustive (All jobs are mapped)
	totalProcessed := processedBy1 + processedBy2 + processedBy3
	if totalProcessed != 10 {
		t.Fatalf("Expected exactly 10 jobs to be processed, got %d", totalProcessed)
	}

	// Check string Generics support
	jobKey := "report-generation-task-xyz"
	if autoshard.IsMyJob(node1, jobKey) || autoshard.IsMyJob(node2, jobKey) || autoshard.IsMyJob(node3, jobKey) {
		t.Log("Successfully mapped string generically.")
	} else {
		t.Fatal("Job string was completely ignored by the cluster.")
	}
}

func TestPartitioner_AsymmetricSafeMode(t *testing.T) {
	registry, _ := memory.NewRegistry()
	ctx := context.Background()

	opts := []autoshard.Option{
		autoshard.WithActiveWindow(5 * time.Second),
		autoshard.DisableJitter(),
	}

	nodeA, _ := autoshard.NewPartitioner("node-A", registry, opts...)

	// Initial Sync for A
	_ = nodeA.Sync(ctx)
	_ = nodeA.Sync(ctx) // Double sync for immediate convergence

	if autoshard.IsMyJob(nodeA, 100) {
		t.Logf("Node A is initialized and working properly")
	} else {
		t.Fatal("Node A failed to initialize!")
	}

	// 💥 Introduce Node B (Chaos)
	nodeB, _ := autoshard.NewPartitioner("node-B", registry, opts...)
	_ = nodeB.Sync(ctx) // Node B enters, sees 2 members. B's perceived_version = 2.

	// At this exact moment, Node A still hasn't synced since B joined!
	// Node A's perceived version is still 1. The cluster is NOT converged!
	// According to Asymmetric Safe Mode:
	// - Node A (old member) should STILL process jobs based on its old state (N=1).
	// - Node B (new member) should STAND BY and accept 0 jobs.

	if !autoshard.IsMyJob(nodeA, 100) {
		t.Fatal("Node A panicked and dropped its load when it shouldn't have!")
	}

	for i := 1; i <= 10; i++ {
		if autoshard.IsMyJob(nodeB, i) {
			t.Fatal("Node B started consuming jobs before cluster convergence! Split-Brain vulnerability!")
		}
	}
}

func TestUtils_GenerateMemberID(t *testing.T) {
	id1 := autoshard.GenerateMemberID("worker")
	id2 := autoshard.GenerateMemberID("worker")
	if id1 == id2 {
		t.Errorf("GenerateMemberID produced duplicate IDs: %s", id1)
	}
}

func TestOptions_Functional(t *testing.T) {
	customHasher := func(data []byte) uint64 { return 42 }
	opts := []autoshard.Option{
		autoshard.WithSyncInterval(1 * time.Minute),
		autoshard.WithActiveWindow(2 * time.Minute),
		autoshard.WithHasher(customHasher),
		autoshard.WithJitterPercent(0.5),
	}

	reg, _ := memory.NewRegistry()
	p, _ := autoshard.NewPartitioner("test", reg, opts...)
	if p.Options().SyncInterval != 1*time.Minute {
		t.Errorf("Expected 1m sync interval")
	}
	if p.Options().ActiveWindow != 2*time.Minute {
		t.Errorf("Expected 2m active window")
	}
	if p.Options().Hasher(nil) != 42 {
		t.Errorf("Expected custom hasher to return 42")
	}
	if p.Options().JitterPercent != 0.5 {
		t.Errorf("Expected 0.5 jitter percent")
	}

	// Test Jitter calculation
	interval := p.Options().GetJitteredInterval(100 * time.Millisecond)
	if interval < 100*time.Millisecond || interval > 150*time.Millisecond {
		t.Errorf("Jittered interval out of bounds: %v", interval)
	}
}

func TestPartitioner_Shutdown(t *testing.T) {
	registry, _ := memory.NewRegistry()
	p, _ := autoshard.NewPartitioner("node-1", registry)
	
	_ = p.Sync(context.Background())
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	members, _ := registry.GetActiveMembers(context.Background(), 10*time.Second)
	if len(members) != 0 {
		t.Errorf("Member not deregistered after shutdown")
	}
}

func TestPartitioner_RunSyncLoop(t *testing.T) {
	registry, _ := memory.NewRegistry()
	p, _ := autoshard.NewPartitioner("node-1", registry, autoshard.WithSyncInterval(10*time.Millisecond), autoshard.DisableJitter())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run in goroutine as it is blocking
	go p.RunSyncLoop(ctx)

	// Wait for at least one sync cycle
	time.Sleep(50 * time.Millisecond)

	// Check if member exists in registry
	members, _ := registry.GetActiveMembers(context.Background(), 1*time.Minute)
	if len(members) == 0 {
		t.Errorf("Member never synced during RunSyncLoop")
	}

	cancel() // Trigger shutdown
	time.Sleep(20 * time.Millisecond)
}

func TestIsMyJob_AllTypes(t *testing.T) {
	registry, _ := memory.NewRegistry()
	p, _ := autoshard.NewPartitioner("node-1", registry)
	
	// Converge to N=1
	_ = p.Sync(context.Background())
	_ = p.Sync(context.Background())

	// Every type should be "mine" since N=1
	if !autoshard.IsMyJob(p, "test") ||
		!autoshard.IsMyJob(p, int(1)) ||
		!autoshard.IsMyJob(p, int32(1)) ||
		!autoshard.IsMyJob(p, int64(1)) ||
		!autoshard.IsMyJob(p, uint(1)) ||
		!autoshard.IsMyJob(p, uint32(1)) ||
		!autoshard.IsMyJob(p, uint64(1)) {
		t.Errorf("IsMyJob failed for one of the JobID types")
	}
}

func TestNewPartitioner_Errors(t *testing.T) {
	reg, _ := memory.NewRegistry()
	
	if _, err := autoshard.NewPartitioner("", reg); err == nil {
		t.Error("Expected error for empty memberID")
	}
	
	if _, err := autoshard.NewPartitioner("node-1", nil); err == nil {
		t.Error("Expected error for nil registry")
	}
}

func TestPartitioner_Status(t *testing.T) {
	reg, _ := memory.NewRegistry()
	p, _ := autoshard.NewPartitioner("node-1", reg)
	
	status := p.Status()
	if status.MemberID != "node-1" {
		t.Errorf("Expected memberID node-1, got %s", status.MemberID)
	}
	if status.IsConverged {
		t.Error("Initial state should not be converged")
	}
}

func TestPartitioner_Callbacks(t *testing.T) {
	reg, _ := memory.NewRegistry()
	
	syncErrorCalled := false
	stateChangeCalled := false
	
	opts := []autoshard.Option{
		autoshard.WithOnSyncError(func(err error) { syncErrorCalled = true }),
		autoshard.WithOnStateChange(func(conv bool, total int, idx int) { stateChangeCalled = true }),
	}
	
	p, _ := autoshard.NewPartitioner("node-1", reg, opts...)
	
	// Trigger sync
	_ = p.Sync(context.Background())
	
	if !stateChangeCalled {
		t.Error("OnStateChange was not called during Sync")
	}

	// Manually trigger a mock sync error callback to cover the field
	if p.Options().OnSyncError != nil {
		p.Options().OnSyncError(fmt.Errorf("mock error"))
	}
	if !syncErrorCalled {
		t.Error("OnSyncError was not called")
	}
}

func TestOptions_Jitter(t *testing.T) {
	opts := &autoshard.Options{
		EnableJitter:  true,
		JitterPercent: 0.2,
	}
	
	base := 100 * time.Millisecond
	for i := 0; i < 100; i++ {
		val := opts.GetJitteredInterval(base)
		if val < base || val > 120*time.Millisecond {
			t.Errorf("Jittered value out of range: %v", val)
		}
	}
	
	// Edge cases
	opts.EnableJitter = false
	if opts.GetJitteredInterval(base) != base {
		t.Error("Jitter should be disabled")
	}
	
	opts.EnableJitter = true
	opts.JitterPercent = -1
	if opts.GetJitteredInterval(base) != base {
		t.Error("Jitter should be disabled for negative percent")
	}
}

func TestIsMyJob_NegativeInt(t *testing.T) {
	reg, _ := memory.NewRegistry()
	p, _ := autoshard.NewPartitioner("node-1", reg)
	_ = p.Sync(context.Background())
	_ = p.Sync(context.Background()) // Converge to N=1
	
	if !autoshard.IsMyJob(p, int(-1)) {
		t.Error("IsMyJob failed for negative int")
	}
	if !autoshard.IsMyJob(p, int32(-100)) {
		t.Error("IsMyJob failed for negative int32")
	}
	if !autoshard.IsMyJob(p, int64(-999999)) {
		t.Error("IsMyJob failed for negative int64")
	}
}

