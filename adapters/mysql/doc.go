// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

/*
Package mysql provides a MySQL/MariaDB backed registry adapter for autoshard.

This adapter leverages intrinsic database time functions (e.g., NOW()) to strictly
evaluate member pulse and heartbeat TTLs. This architectural decision completely
eliminates Split-Brain scenarios triggered by clock skew anomalies across application servers.

Furthermore, it integrates a deterministic Dynamic Leader Election mechanism to
distribute Garbage Collection responsibilities autonomously, preventing database lock contentions
while ensuring "Zombie" members are physically exterminated.
*/
package mysql
