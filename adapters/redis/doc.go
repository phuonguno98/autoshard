// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

/*
Package redis provides a Redis backed registry adapter for autoshard.

By utilizing high-performance in-memory key-value data structures and
native Time-To-Live (TTL) expirations, this adapter guarantees rapid
convergence rates and implicit Garbage Collection for the clustering algorithm,
maintaining simplicity and exceptional throughput.

WARNING: The current implementation uses Redis SCAN to discover member keys.
If you are deploying Autoshard on a massive, shared Redis instance containing
millions of unrelated keys, the SCAN operation may suffer from increased latency.
It is highly recommended to use a dedicated Redis Logical Database (e.g., SELECT 1)
to ensure the Keyspace remains small and Sync operations remain lightning fast.
*/
package redis
