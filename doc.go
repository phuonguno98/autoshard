// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

/*
Package autoshard provides a masterless, active-active workload partitioning library.

It utilizes modulo hashing combined with a zero-communication consensus barrier
to deterministically distribute workload jobs across a dynamic cluster of application
members (nodes/workers). Instead of relying on gossip protocols or direct network RPC,
autoshard leverages your existing database (MySQL, PostgreSQL, Redis) as the single source
of truth for member discovery and cluster convergence.

Key Features:
- Masterless Active-Active: Every member is equal, completely removing single points of failure.
- Zero-Communication Consensus: No peer-to-peer network mesh required, circumventing network partitions.
- Asymmetric Safe Mode: Prioritizes high availability during rebalancing to prevent systemic stagnation.
- Generics Support: Seamlessly handles strings, integers, and unsigned integers as Job IDs.
- Deterministic Hashing: Built-in SHA-256 hashing ensuring cross-platform stability.
*/
package autoshard
