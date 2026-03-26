// MIT License
//
// Copyright (c) 2026 phuonguno98
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	"github.com/phuonguno98/autoshard"
)

var validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Registry implements autoshard.Registry for MySQL/MariaDB databases.
// EXTREMELY IMPORTANT: It strictly evaluates time using the Database's intrinsic NOW() function,
// instead of Go's time.Now(), to completely eliminate Split-Brain issues introduced by clock skew.
type Registry struct {
	db        *sql.DB
	tableName string
}

// NewRegistry initializes the MySQL Registry.
// It validates the tableName to prevent SQL Injection (Defensive Programming).
func NewRegistry(db *sql.DB, tableName string) (*Registry, error) {
	if tableName == "" {
		tableName = "autoshard_registry"
	}

	if !validTableName.MatchString(tableName) {
		return nil, fmt.Errorf("autoshard/mysql: invalid table name %q", tableName)
	}

	return &Registry{
		db:        db,
		tableName: tableName,
	}, nil
}

// Heartbeat employs UPSERT logic (INSERT ... ON DUPLICATE KEY UPDATE)
// to accurately record the member's pulse and current perceived cluster scale.
func (r *Registry) Heartbeat(ctx context.Context, memberID string, perceivedVersion int) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (member_id, perceived_version, last_heartbeat) 
		VALUES (?, ?, NOW()) 
		ON DUPLICATE KEY UPDATE 
			perceived_version = VALUES(perceived_version), 
			last_heartbeat = NOW()
	`, r.tableName)

	_, err := r.db.ExecContext(ctx, query, memberID, perceivedVersion)
	if err != nil {
		return fmt.Errorf("execute heartbeat query: %w", err)
	}
	return nil
}

// GetActiveMembers fetches nodes that have reported a heartbeat within the allowable activeWindow.
// Leveraging MySQL's NOW() ensures deterministic logic irrespective of Application Nodes' clocks.
func (r *Registry) GetActiveMembers(ctx context.Context, activeWindow time.Duration) ([]autoshard.MemberInfo, error) {
	// Convert duration to seconds for precision interval matching via SQL
	seconds := int(activeWindow.Seconds())
	if seconds <= 0 {
		seconds = 30 // Safe fallback
	}

	query := fmt.Sprintf(`
		SELECT member_id, perceived_version 
		FROM %s 
		WHERE last_heartbeat >= NOW() - INTERVAL ? SECOND
	`, r.tableName)

	rows, err := r.db.QueryContext(ctx, query, seconds)
	if err != nil {
		return nil, fmt.Errorf("query active members: %w", err)
	}
	defer rows.Close()

	var members []autoshard.MemberInfo
	for rows.Next() {
		var m autoshard.MemberInfo
		if err := rows.Scan(&m.ID, &m.PerceivedVersion); err != nil {
			return nil, fmt.Errorf("scan active member row: %w", err)
		}
		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active member rows: %w", err)
	}

	return members, nil
}

// Deregister natively wipes the member row upon Graceful Shutdown requests.
func (r *Registry) Deregister(ctx context.Context, memberID string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE member_id = ?`, r.tableName)
	if _, err := r.db.ExecContext(ctx, query, memberID); err != nil {
		return fmt.Errorf("execute deregister query: %w", err)
	}
	return nil
}

// StartGarbageCollector triggers physical extermination of strictly dead members.
// Showcases Dynamic Leader Election: Exclusively the node holding the newest heartbeat
// executes the DROP query (ties broken by member_id precedence), averting deadlocks and duplicated labor.
func (r *Registry) StartGarbageCollector(ctx context.Context, myMemberID string, checkInterval, deadThreshold time.Duration) error {
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		deadSeconds := int(deadThreshold.Seconds())

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Dynamic Leader Election: Determining the alpha node of this tick
				// We use a window of checkInterval * 2 to ensure we find active nodes
				leaderWindow := int(checkInterval.Seconds() * 2)
				if leaderWindow < 60 {
					leaderWindow = 60 // Minimum safety window
				}

				queryLeader := fmt.Sprintf(`
					SELECT member_id 
					FROM %s 
					WHERE last_heartbeat >= NOW() - INTERVAL ? SECOND
					ORDER BY last_heartbeat DESC, member_id DESC 
					LIMIT 1
				`, r.tableName)

				var leaderID string
				err := r.db.QueryRowContext(ctx, queryLeader, leaderWindow).Scan(&leaderID)

				if err == sql.ErrNoRows || err != nil {
					continue // Failed to nominate a leader or all nodes are virtually dead
				}

				if leaderID == myMemberID {
					// We act as the GC Leader for this turn. Launch Extermination command.
					queryDelete := fmt.Sprintf(`
						DELETE FROM %s 
						WHERE last_heartbeat < NOW() - INTERVAL ? SECOND
					`, r.tableName)

					_, _ = r.db.ExecContext(ctx, queryDelete, deadSeconds)
				}
			}
		}
	}()
	return nil
}
