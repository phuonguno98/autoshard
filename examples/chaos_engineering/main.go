// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package main

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/phuonguno98/autoshard"
	"github.com/phuonguno98/autoshard/adapters/memory"
)

var startTime = time.Now()

// Format elapsed time from script start
func ts() string {
	return fmt.Sprintf("[%06.3fs]", time.Since(startTime).Seconds())
}

// ChaosProxy wraps the Registry to inject Network/DB failures
type ChaosProxy struct {
	realReg       autoshard.Registry
	mu            sync.RWMutex
	globalDown    bool
	isolatedNodes map[string]bool
}

func NewChaosProxy(r autoshard.Registry) *ChaosProxy {
	return &ChaosProxy{
		realReg:       r,
		isolatedNodes: make(map[string]bool),
	}
}

func (c *ChaosProxy) SetIsolate(memberID string, val bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.isolatedNodes[memberID] = val
}

func (c *ChaosProxy) SetGlobalDown(val bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.globalDown = val
}

func (c *ChaosProxy) Heartbeat(ctx context.Context, memberID string, perceivedVersion int) error {
	c.mu.RLock()
	down := c.globalDown
	isolated := c.isolatedNodes[memberID]
	c.mu.RUnlock()

	if down || isolated {
		return fmt.Errorf("connection refused")
	}
	return c.realReg.Heartbeat(ctx, memberID, perceivedVersion)
}

func (c *ChaosProxy) GetActiveMembers(ctx context.Context, activeWindow time.Duration) ([]autoshard.MemberInfo, error) {
	c.mu.RLock()
	down := c.globalDown
	isolated := false

	// Extract ID from context to identify the calling node
	if memberID, ok := ctx.Value(contextKeyMemberID).(string); ok {
		isolated = c.isolatedNodes[memberID]
	}
	c.mu.RUnlock()

	if down || isolated {
		return nil, fmt.Errorf("connection refused")
	}
	return c.realReg.GetActiveMembers(ctx, activeWindow)
}

func (c *ChaosProxy) Deregister(ctx context.Context, memberID string) error {
	c.mu.RLock()
	down := c.globalDown
	isolated := c.isolatedNodes[memberID]
	c.mu.RUnlock()

	if down || isolated {
		return fmt.Errorf("connection refused")
	}
	return c.realReg.Deregister(ctx, memberID)
}

func (c *ChaosProxy) StartGarbageCollector(ctx context.Context, memberID string, checkInterval, deadThreshold time.Duration) error {
	return c.realReg.StartGarbageCollector(ctx, memberID, checkInterval, deadThreshold)
}

type ctxKey string

const contextKeyMemberID ctxKey = "chaos_memberID"

func createNode(id string, reg autoshard.Registry) *autoshard.Partitioner {
	p, _ := autoshard.NewPartitioner(id, reg,
		// Using an artificial SyncInterval because we trigger Sync() manually
		autoshard.WithSyncInterval(1*time.Hour),
		autoshard.WithActiveWindow(400*time.Millisecond),
	)
	return p
}

// Synchronous Sync Tick
func tickCycle(cycle int, msg string, nodes []*autoshard.Partitioner, jobs []int) {
	fmt.Printf("\n%s =========================================================================\n", ts())
	fmt.Printf("%s 🔄 CYCLE %d: %s\n", ts(), cycle, msg)
	fmt.Printf("%s =========================================================================\n", ts())

	for _, p := range nodes {
		ctxWithID := context.WithValue(context.Background(), contextKeyMemberID, p.Status().MemberID)
		_ = p.Sync(ctxWithID)
	}

	totalAssigned := 0
	overlapJobs := make(map[int]int)
	minJobs := len(jobs)
	maxJobs := 0
	convergedCount := 0

	for _, p := range nodes {
		var assignedJobs []int
		for _, jobID := range jobs {
			if autoshard.IsMyJob(p, jobID) {
				assignedJobs = append(assignedJobs, jobID)
				overlapJobs[jobID]++
			}
		}

		status := p.Status()
		statusStr := "🔴 WAITING   "
		if status.IsConverged {
			statusStr = "🟢 CONVERGED "
			convergedCount++
		}

		c := len(assignedJobs)
		totalAssigned += c
		if c < minJobs {
			minJobs = c
		}
		if c > maxJobs {
			maxJobs = c
		}

		jobListStr := fmt.Sprintf("%v", assignedJobs)
		if len(assignedJobs) == 0 {
			jobListStr = "[]"
		}

		fmt.Printf("%s    ├─ Node [%-10s] | %s | Cluster Size: %2d/%2d | Assigned: %2d jobs -> %s\n",
			ts(), status.MemberID, statusStr, status.MyIndex, status.TotalMembers, c, jobListStr)
	}

	dropped := 0
	overlapped := 0
	for _, jobID := range jobs {
		count := overlapJobs[jobID]
		if count == 0 {
			dropped++
		} else if count > 1 {
			overlapped++
		}
	}

	fmt.Printf("%s    └─ Validation (Quality Gate): Converged Nodes: %d/%d | Dropped Jobs: %d | Overlapped Jobs: %d\n", ts(), convergedCount, len(nodes), dropped, overlapped)

	if dropped == 0 && overlapped == 0 && len(nodes) > 0 {
		if maxJobs == 0 {
			fmt.Printf("%s       ⚠️ SYSTEM HALTED: Entire system is in defense mode (All jobs are frozen).\n", ts())
		} else {
			fmt.Printf("%s       ✅ PERFECT MATCH: Perfect convergence. Nodes evenly divided %d jobs (max deviation: %d jobs).\n", ts(), len(jobs), maxJobs-minJobs)
		}
	} else if dropped > 0 || overlapped > 0 {
		fmt.Printf("%s       ⚠️ TRANSITION/AP-MODE: System is actively resizing or serving via Stale State.\n", ts())
	}
}

func runConvergencePhases(nodes []*autoshard.Partitioner, jobs []int, prefix string) {
	tickCycle(1, prefix+": Propagating heartbeat and identifying actual cluster size.", nodes, jobs)
	tickCycle(2, prefix+": Synchronizing Perceived_Version.", nodes, jobs)
	tickCycle(3, prefix+": Consensus reached. Convergence barrier established.", nodes, jobs)
}

func main() {
	startTime = time.Now()
	fmt.Printf("%s 🚀 AUTOSHARD TIME-SYNC TEST SUITE (TIER-1 VALIDATION)\n", ts())
	fmt.Printf("%s =======================================================\n", ts())

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var jobs []int
	for i := 0; i < 100; i++ {
		jobs = append(jobs, 10000+rng.Intn(90000))
	}
	sort.Ints(jobs)
	fmt.Printf("%s 📦 Generated 100 random Job IDs for workload distribution\n\n", ts())

	ctx, cancelGlobal := context.WithCancel(context.Background())
	defer cancelGlobal()

	// Initialize Memory DB (Simulating MySQL/Redis)
	memoryReg, _ := memory.NewRegistry()
	_ = memoryReg.StartGarbageCollector(ctx, "system-gc", 50*time.Millisecond, 400*time.Millisecond)
	chaosReg := NewChaosProxy(memoryReg)

	// =========================================================
	fmt.Printf("%s ▶️ [SCENARIO 1] THUNDERING HERD (Massive startup of 50 workers)\n", ts())
	var nodes1 []*autoshard.Partitioner
	for i := 1; i <= 50; i++ {
		nodes1 = append(nodes1, createNode(fmt.Sprintf("node-%02d", i), chaosReg))
	}
	runConvergencePhases(nodes1, jobs, "Injecting 50 workers")

	nodes1 = nil
	time.Sleep(500 * time.Millisecond) // Wait for GC to clear DB for next scenario

	// =========================================================
	fmt.Printf("\n%s ▶️ [SCENARIO 2] GRACEFUL SCALE-DOWN (Safe cluster termination with SIGTERM)\n", ts())
	var nodes2 []*autoshard.Partitioner
	for i := 1; i <= 5; i++ {
		nodes2 = append(nodes2, createNode(fmt.Sprintf("node-%02d", i), chaosReg))
	}
	runConvergencePhases(nodes2, jobs, "Initializing 5 Nodes")

	fmt.Printf("\n%s    🔽 ACTION: Shutting down node-04 and node-05 (Calling Shutdown()...)\n", ts())
	nodes2[3].Shutdown(context.Background())
	nodes2[4].Shutdown(context.Background())
	runConvergencePhases(nodes2[:3], jobs, "Remaining 3 Nodes rebalancing")

	nodes2 = nil
	time.Sleep(500 * time.Millisecond)

	// =========================================================
	fmt.Printf("\n%s ▶️ [SCENARIO 3] HARD CRASH (Sudden hardware failure)\n", ts())
	var nodes3 []*autoshard.Partitioner
	for i := 1; i <= 3; i++ {
		nodes3 = append(nodes3, createNode(fmt.Sprintf("node-%02d", i), chaosReg))
	}
	runConvergencePhases(nodes3, jobs, "Initializing 3 Nodes")

	fmt.Printf("\n%s    🔌 ACTION: Hardware failure. node-03 died abruptly (No Shutdown)!\n", ts())
	chaosReg.SetIsolate("node-03", true)

	tickCycle(1, "Isolated State: ActiveWindow (400ms) not reached, maintaining node-03's DB cache. Activating AP-Mode.", nodes3[:2], jobs)

	fmt.Printf("\n%s    ⏳ Waiting for ActiveWindow (450ms). DB Garbage Collector cleaning up...\n", ts())
	time.Sleep(450 * time.Millisecond)

	runConvergencePhases(nodes3[:2], jobs, "GC cleanup complete. Cluster reconverged to N=2")

	nodes3 = nil
	chaosReg.SetIsolate("node-03", false)
	time.Sleep(500 * time.Millisecond)

	// =========================================================
	fmt.Printf("\n%s ▶️ [SCENARIO 4] NETWORK PARTITION (Split-Brain Prevention)\n", ts())
	var nodes4 []*autoshard.Partitioner
	for i := 1; i <= 5; i++ {
		nodes4 = append(nodes4, createNode(fmt.Sprintf("node-%02d", i), chaosReg))
	}
	runConvergencePhases(nodes4, jobs, "Initializing 5 Nodes")

	fmt.Printf("\n%s    ✂️ ACTION: Network cable cut. node-01 and node-02 are isolated from the cluster.\n", ts())
	chaosReg.SetIsolate("node-01", true)
	chaosReg.SetIsolate("node-02", true)

	tickCycle(1, "ISOLATED: Isolated side (1,2) lost network, detected DB error, immediately Fail-Open (Stale State N=5).", nodes4[:2], jobs)
	fmt.Printf("\n%s    ⏳ Waiting for GC to clear heartbeats of the isolated group from the Database...\n", ts())
	time.Sleep(450 * time.Millisecond)

	runConvergencePhases(nodes4[2:], jobs, "Healthy side (3,4,5) forms new cluster N=3")

	fmt.Printf("\n%s    🔌 ACTION: Network restored. node-01 and node-02 reconnected to DB.\n", ts())
	chaosReg.SetIsolate("node-01", false)
	chaosReg.SetIsolate("node-02", false)

	runConvergencePhases(nodes4, jobs, "Merging 2 clusters (Reunion)")
}
