// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...

package mysql

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLRegistry_Heartbeat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open mock database: %v", err)
	}
	defer db.Close()

	reg, _ := NewRegistry(db, "test_table")
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO test_table")).
		WithArgs("node-1", 3).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := reg.Heartbeat(ctx, "node-1", 3); err != nil {
		t.Errorf("Heartbeat failed: %v", err)
	}
}

func TestMySQLRegistry_GetActiveMembers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open mock database: %v", err)
	}
	defer db.Close()

	reg, _ := NewRegistry(db, "") // should use default table
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"member_id", "perceived_version"}).
		AddRow("node-1", 2).
		AddRow("node-2", 2)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT member_id, perceived_version FROM autoshard_registry")).
		WithArgs(30).
		WillReturnRows(rows)

	members, err := reg.GetActiveMembers(ctx, 30*time.Second)
	if err != nil {
		t.Errorf("GetActiveMembers failed: %v", err)
	}

	if len(members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(members))
	}
}

func TestMySQLRegistry_Deregister(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open mock database: %v", err)
	}
	defer db.Close()

	reg, _ := NewRegistry(db, "test_table")
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM test_table WHERE member_id = ?")).
		WithArgs("node-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := reg.Deregister(ctx, "node-1"); err != nil {
		t.Errorf("Deregister failed: %v", err)
	}
}

func TestMySQLRegistry_StartGarbageCollector(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open mock database: %v", err)
	}
	defer db.Close()

	reg, _ := NewRegistry(db, "test_table")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	myID := "node-alpha"

	// 1. We are the leader
	mock.ExpectQuery(regexp.QuoteMeta("SELECT member_id FROM test_table")).
		WithArgs(60).
		WillReturnRows(sqlmock.NewRows([]string{"member_id"}).AddRow(myID))

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM test_table WHERE last_heartbeat")).
		WithArgs(10).
		WillReturnResult(sqlmock.NewResult(0, 5))

	// Execute one cycle manually (Synchronous)
	_ = reg.performGarbageCollection(ctx, myID, 50*time.Millisecond, 10*time.Second)

	// 2. We are NOT the leader
	mock.ExpectQuery(regexp.QuoteMeta("SELECT member_id FROM test_table")).
		WithArgs(60).
		WillReturnRows(sqlmock.NewRows([]string{"member_id"}).AddRow("other-leader"))

	_ = reg.performGarbageCollection(ctx, myID, 50*time.Millisecond, 10*time.Second)

	// 3. Error in query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT member_id FROM test_table")).
		WithArgs(60).
		WillReturnError(fmt.Errorf("db error"))

	_ = reg.performGarbageCollection(ctx, myID, 50*time.Millisecond, 10*time.Second)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Expectations not met: %v", err)
	}
}

func TestMySQLRegistry_Errors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open mock database: %v", err)
	}
	defer db.Close()

	reg, _ := NewRegistry(db, "test_table")
	ctx := context.Background()

	// Heartbeat error
	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("exec error"))
	if err := reg.Heartbeat(ctx, "id", 1); err == nil {
		t.Error("Expected error from Heartbeat")
	}

	// GetActiveMembers query error
	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("query error"))
	if _, err := reg.GetActiveMembers(ctx, 10*time.Second); err == nil {
		t.Error("Expected error from GetActiveMembers")
	}

	// Scan error
	rows := sqlmock.NewRows([]string{"id", "ver"}).AddRow("id", "not-an-int")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	if _, err := reg.GetActiveMembers(ctx, 10*time.Second); err == nil {
		t.Error("Expected scan error from GetActiveMembers")
	}

	// Deregister error
	mock.ExpectExec("DELETE FROM").WillReturnError(fmt.Errorf("del error"))
	if err := reg.Deregister(ctx, "id"); err == nil {
		t.Error("Expected error from Deregister")
	}
}

func TestMySQLRegistry_NewRegistryError(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	if _, err := NewRegistry(db, "drop table students;--"); err == nil {
		t.Error("Expected error for malicious table name")
	}
}
