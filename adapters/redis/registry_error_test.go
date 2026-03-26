package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// MockRedis errors for specific coverage
type mockRedisErr struct {
	redis.Cmdable
	err error
}

func (m *mockRedisErr) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetErr(m.err)
	return cmd
}

func (m *mockRedisErr) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetErr(m.err)
	return cmd
}

func (m *mockRedisErr) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	cmd := redis.NewScanCmd(ctx, nil, 0)
	cmd.SetErr(m.err)
	return cmd
}

func TestRedisRegistry_Errors(t *testing.T) {
	mock := &mockRedisErr{err: errors.New("network error")}
	reg, _ := NewRegistry(mock, "test:", 1*time.Minute)
	ctx := context.Background()

	if err := reg.Heartbeat(ctx, "id", 1); err == nil {
		t.Error("Expected error from Heartbeat")
	}

	if _, err := reg.GetActiveMembers(ctx, 1*time.Second); err == nil {
		t.Error("Expected error from GetActiveMembers (Scan)")
	}

	if err := reg.Deregister(ctx, "id"); err == nil {
		t.Error("Expected error from Deregister")
	}
}
