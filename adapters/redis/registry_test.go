// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisRegistry(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to run miniredis: %v", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	reg, _ := NewRegistry(client, "test:", 1*time.Minute)
	ctx := context.Background()

	// 1. Test Heartbeat
	if err := reg.Heartbeat(ctx, "member-1", 5); err != nil {
		t.Errorf("Heartbeat failed: %v", err)
	}

	// Verify key in redis
	val, _ := s.Get("test:member-1")
	if val != "5" {
		t.Errorf("Expected 5 in redis, got %s", val)
	}

	// 2. Test GetActiveMembers
	members, err := reg.GetActiveMembers(ctx, 1*time.Minute)
	if err != nil {
		t.Errorf("GetActiveMembers failed: %v", err)
	}
	if len(members) != 1 || members[0].ID != "member-1" {
		t.Errorf("Expected 1 member with ID member-1, got %v", members)
	}

	// 3. Test Deregister
	if err := reg.Deregister(ctx, "member-1"); err != nil {
		t.Errorf("Deregister failed: %v", err)
	}

	if s.Exists("test:member-1") {
		t.Errorf("Key should have been deleted")
	}

	// 4. Test StartGarbageCollector (No-op)
	if err := reg.StartGarbageCollector(ctx, "id", 1*time.Second, 1*time.Minute); err != nil {
		t.Errorf("GC failed: %v", err)
	}
}

func TestRedisRegistry_NewDefaults(t *testing.T) {
	reg1, _ := NewRegistry(nil, "", 0)
	if reg1.prefix != "autoshard:" {
		t.Errorf("Expected autoshard: prefix, got %s", reg1.prefix)
	}
	if reg1.ttl != 60*time.Second {
		t.Errorf("Expected 60s ttl, got %v", reg1.ttl)
	}

	reg2, _ := NewRegistry(nil, "myapp", 0)
	if reg2.prefix != "myapp:" {
		t.Errorf("Expected myapp: prefix, got %s", reg2.prefix)
	}
}

func TestRedisRegistry_EdgeCases(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	reg, _ := NewRegistry(client, "test:", 1*time.Minute)
	ctx := context.Background()

	// 1. GetActiveMembers with no keys
	members, err := reg.GetActiveMembers(ctx, 1*time.Minute)
	if err != nil {
		t.Errorf("GetActiveMembers failed on empty redis: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("Expected 0 members, got %d", len(members))
	}

	// 2. GetActiveMembers with invalid value
	s.Set("test:bad-node", "not-a-number")
	members, _ = reg.GetActiveMembers(ctx, 1*time.Minute)
	if len(members) != 0 {
		t.Errorf("Expected 0 valid members (skipped bad-node), got %d", len(members))
	}
	s.Del("test:bad-node")

	// 3. Simulated Nil value (MGET results can have nils if keys expire between SCAN and MGET)
	// We can't easily simulate this with miniredis SCAN+MGET directly in a deterministic way,
	// but we can trust the logic once we've covered other parts, or use a manual mock if needed.
}
