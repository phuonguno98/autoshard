// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package autoshard

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"
)

// GenerateMemberID automatically allocates a unique identifier (ID) for a Worker / Member.
// Having sufficient hostname, timestamp, and 8 bytes of random entropy eliminates the risk of ID collision.
// Output format: "{prefix}-{hostname}-{timestamp}-{hex}"
// Example: myworker-pod-asdr-16982233-a1b2c3d4
func GenerateMemberID(prefix string) string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// 8 bytes ~ 16 hex chars padding (safe for massive burst scaling and high-concurrency clusters)
	b := make([]byte, 8)
	_, _ = rand.Read(b)

	return fmt.Sprintf("%s-%s-%d-%x", prefix, hostname, time.Now().Unix(), b)
}
