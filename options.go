// MIT License
//
// Copyright (c) 2026 phuonguno98
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package autoshard

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"time"
)

// HasherFunc defines a custom hash function mapping a byte array to uint64.
// Allows the Consumer to use MurmurHash, FNV-1a, etc., instead of the default SHA-256.
type HasherFunc func([]byte) uint64

// DefaultHasher is the default hashing function using the SHA-256 algorithm due to its
// high collision resistance and deterministic nature, highly suitable for modulo partitioning.
// It returns the first 8 bytes of the SHA-256 sum as a uint64.
func DefaultHasher(data []byte) uint64 {
	sum := sha256.Sum256(data)
	return binary.BigEndian.Uint64(sum[:8])
}

// Options contains configuration for the Partitioner.
type Options struct {
	// SyncInterval is the cycle for fetching State from the Datastore continuously.
	SyncInterval time.Duration

	// ActiveWindow is the safe lifetime of a Member.
	// Any Member without a Heartbeat within this window will be flagged as dead.
	ActiveWindow time.Duration

	// Hasher is a custom hash function (if replacing SHA-256 is desired).
	Hasher HasherFunc

	// EnableJitter relaxes the Heartbeat and Sync cycles by adding
	// random delay variations (to prevent Thundering Herd on the database).
	EnableJitter bool

	// JitterPercent is the percentage (0-1) of random amplitude variation.
	JitterPercent float64

	// OnSyncError is an optional callback invoked when a Sync cycle fails.
	// This enables the consumer to integrate with their observability stack.
	OnSyncError func(err error)

	// OnStateChange is invoked when the Partitioner's convergence state transitions.
	// Useful for emitting metrics (e.g., gauge for cluster size, convergence status).
	OnStateChange func(converged bool, totalMembers int, myIndex int)
}

// Option represents a function applying the Functional Options Pattern.
type Option func(*Options)

// defaultOptions returns safe, optimized out-of-the-box configuration.
func defaultOptions() *Options {
	return &Options{
		SyncInterval:  10 * time.Second,
		ActiveWindow:  30 * time.Second,
		Hasher:        DefaultHasher,
		EnableJitter:  true,
		JitterPercent: 0.20, // default varies by an additional max of 20%
	}
}

// WithSyncInterval adjusts the synchronization interval.
func WithSyncInterval(d time.Duration) Option {
	return func(o *Options) {
		if d > 0 {
			o.SyncInterval = d
		}
	}
}

// WithActiveWindow adjusts the dead threshold timing.
func WithActiveWindow(d time.Duration) Option {
	return func(o *Options) {
		if d > 0 {
			o.ActiveWindow = d
		}
	}
}

// WithHasher configures a custom hash function.
func WithHasher(h HasherFunc) Option {
	return func(o *Options) {
		if h != nil {
			o.Hasher = h
		}
	}
}

// DisableJitter allows turning off the Jitter compensation (commonly used in Unit Tests).
func DisableJitter() Option {
	return func(o *Options) {
		o.EnableJitter = false
	}
}

// WithJitterPercent adjusts the Jitter variation ratio (default 0.2 is 20%).
func WithJitterPercent(p float64) Option {
	return func(o *Options) {
		if p > 0 && p < 1 {
			o.JitterPercent = p
		}
	}
}

// WithOnSyncError configures a callback for synchronization errors.
func WithOnSyncError(fn func(error)) Option {
	return func(o *Options) {
		o.OnSyncError = fn
	}
}

// WithOnStateChange configures a callback for partitioner state transitions.
func WithOnStateChange(fn func(bool, int, int)) Option {
	return func(o *Options) {
		o.OnStateChange = fn
	}
}

// GetJitteredInterval takes a base duration and adds a random jitter value.
// Breaking the perfect synchronization of dozens of cron jobs from different Pods/Processes.
func (o *Options) GetJitteredInterval(base time.Duration) time.Duration {
	if !o.EnableJitter || o.JitterPercent <= 0 {
		return base
	}

	maxJitter := float64(base) * o.JitterPercent
	if maxJitter <= 0 {
		return base
	}

	// Time will vary positively: base + random(0 to maxJitter)
	jitter := time.Duration(rand.Float64() * maxJitter)
	return base + jitter
}
