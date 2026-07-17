package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestKeyConstants(t *testing.T) {
	if metricsKeyPrefix != "metrics:" {
		t.Fatalf("expected metricsKeyPrefix = 'metrics:', got %q", metricsKeyPrefix)
	}
	if dirtySetKey != "metrics:dirty" {
		t.Fatalf("expected dirtySetKey = 'metrics:dirty', got %q", dirtySetKey)
	}
}

func TestNewClient_NotNil(t *testing.T) {
	c := NewClient(nil)
	if c == nil {
		t.Fatal("expected non-nil Client")
	}
}

func setupMiniredis(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	return NewClient(rdb), mr
}

func TestTrackView_WritesCorrectKeys(t *testing.T) {
	client, mr := setupMiniredis(t)

	err := client.TrackView(context.Background(), "skill", "sk-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify counter key
	counterKey := "metrics:skill:sk-1:view"
	val, err := mr.Get(counterKey)
	if err != nil {
		t.Fatalf("counter key not set: %v", err)
	}
	if val != "1" {
		t.Fatalf("expected counter=1, got %q", val)
	}

	// Verify dirty set
	members, err := mr.Members(dirtySetKey)
	if err != nil {
		t.Fatalf("dirty set error: %v", err)
	}
	if len(members) != 1 || members[0] != "skill:sk-1" {
		t.Fatalf("expected dirty set = [skill:sk-1], got %v", members)
	}

	// Second call should increment
	_ = client.TrackView(context.Background(), "skill", "sk-1")
	val, _ = mr.Get(counterKey)
	if val != "2" {
		t.Fatalf("expected counter=2 after second call, got %q", val)
	}
}

func TestTrackDownload_WritesCorrectKeys(t *testing.T) {
	client, mr := setupMiniredis(t)

	err := client.TrackDownload(context.Background(), "skill", "sk-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	counterKey := "metrics:skill:sk-2:download"
	val, err := mr.Get(counterKey)
	if err != nil {
		t.Fatalf("counter key not set: %v", err)
	}
	if val != "1" {
		t.Fatalf("expected counter=1, got %q", val)
	}

	members, err := mr.Members(dirtySetKey)
	if err != nil {
		t.Fatalf("dirty set error: %v", err)
	}
	found := false
	for _, m := range members {
		if m == "skill:sk-2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'skill:sk-2' in dirty set, got %v", members)
	}
}

func TestTrackInstall_WritesCorrectKeys(t *testing.T) {
	client, mr := setupMiniredis(t)

	err := client.TrackInstall(context.Background(), "mcp", "mcp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	counterKey := "metrics:mcp:mcp-1:install"
	val, err := mr.Get(counterKey)
	if err != nil {
		t.Fatalf("counter key not set: %v", err)
	}
	if val != "1" {
		t.Fatalf("expected counter=1, got %q", val)
	}

	members, err := mr.Members(dirtySetKey)
	if err != nil {
		t.Fatalf("dirty set error: %v", err)
	}
	found := false
	for _, m := range members {
		if m == "mcp:mcp-1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'mcp:mcp-1' in dirty set, got %v", members)
	}
}

func TestTrackView_RedisError(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	client := NewClient(rdb)

	// Close miniredis to simulate connection failure
	mr.Close()

	err := client.TrackView(context.Background(), "skill", "sk-1")
	if err == nil {
		t.Fatal("expected error when Redis is down")
	}
}
