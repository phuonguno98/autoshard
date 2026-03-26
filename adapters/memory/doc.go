// MIT License
//
// Copyright (c) 2026 phuonguno98
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

/*
Package memory implements an in-memory storage adapter for the autoshard registry.

This adapter utilizes a thread-safe Go map (sync.RWMutex) to simulate distributed member
discovery and heartbeat registration. It is highly optimized and exclusively intended
for Local Development environments and Unit Testing of the core partitioner logic.

Due to its volatile and non-distributed nature, it MUST NOT be used in a
Production environment encompassing multiple physical server instances.
*/
package memory
