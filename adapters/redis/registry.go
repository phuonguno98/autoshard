// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/phuonguno98/autoshard"
	"github.com/redis/go-redis/v9"
)

// Registry implements autoshard.Registry for Redis.
// It leverages Redis Keys with Time-To-Live (TTL) expirations to physically
// and naturally drop dead members without requiring active Garbage Collection routines.
type Registry struct {
	client redis.Cmdable
	prefix string
	ttl    time.Duration
}

// NewRegistry initializes a new Redis Registry adapter.
func NewRegistry(client redis.Cmdable, prefix string, ttl time.Duration) (*Registry, error) {
	if prefix == "" {
		prefix = "autoshard:"
	} else if prefix[len(prefix)-1] != ':' {
		prefix += ":"
	}

	if ttl <= 0 {
		ttl = 60 * time.Second
	}

	return &Registry{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}, nil
}

// Heartbeat refreshes the TTL of the member's key and records the perceived Version.
func (r *Registry) Heartbeat(ctx context.Context, memberID string, perceivedVersion int) error {
	key := r.prefix + memberID
	if err := r.client.Set(ctx, key, perceivedVersion, r.ttl).Err(); err != nil {
		return fmt.Errorf("set heartbeat key: %w", err)
	}
	return nil
}

// GetActiveMembers fetches all active members currently available in Redis.
func (r *Registry) GetActiveMembers(ctx context.Context, activeWindow time.Duration) ([]autoshard.MemberInfo, error) {
	// Scan for keys with the designated prefix
	match := r.prefix + "*"

	// SCAN is non-blocking and safer than KEYS in production systems
	var keys []string
	iter := r.client.Scan(ctx, 0, match, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("scan members from redis: %w", err)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	// Fetch values for all found keys in a single Network Roundtrip via MGet
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget members from redis: %w", err)
	}

	var members []autoshard.MemberInfo
	for i, key := range keys {
		// vals[i] can be nil if the key expired between SCAN and MGET
		if vals[i] == nil {
			continue
		}

		// Parse version from interface{} (usually string for redis)
		var version int
		if _, err := fmt.Sscanf(fmt.Sprint(vals[i]), "%d", &version); err != nil {
			continue // Skip corrupted or invalid value
		}

		// Extract memberID by slicing off the prefix
		memberID := key[len(r.prefix):]
		members = append(members, autoshard.MemberInfo{
			ID:               memberID,
			PerceivedVersion: version,
		})
	}

	return members, nil
}

// Deregister purposefully deletes the member's tracking key to enable immediate balancing.
func (r *Registry) Deregister(ctx context.Context, memberID string) error {
	key := r.prefix + memberID
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete heartbeat key: %w", err)
	}
	return nil
}

// StartGarbageCollector serves as a No-op. Redis intrinsically handles GC through Keyspace Expiration (TTL).
func (r *Registry) StartGarbageCollector(ctx context.Context, myMemberID string, checkInterval, deadThreshold time.Duration) error {
	// No operation required in Redis due to TTL
	return nil
}
