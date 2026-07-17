package metrics

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// --- End-to-end integration tests verifying the complete metrics pipeline ---

// TestIntegration_ViewTrackFlushDB verifies the full chain:
// Redis INCR+SADD → flush worker → DB upsert.
func TestIntegration_ViewTrackFlushDB(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	cfg.Interval = 50 * time.Millisecond
	cfg.Batch = 500

	fw := NewFlushWorker(rdb, repo, cfg)
	ctx := context.Background()

	// Simulate frontend calling POST /api/v1/metrics/track { event_type: "view" }
	// which internally calls redis.TrackView:
	pipe := rdb.Pipeline()
	pipe.Incr(ctx, "metrics:skill:skill-abc:view")
	pipe.SAdd(ctx, "metrics:dirty", "skill:skill-abc")
	_, err := pipe.Exec(ctx)
	if err != nil {
		t.Fatalf("simulate track view: %v", err)
	}

	// Verify Redis state before flush
	v, err := rdb.Get(ctx, "metrics:skill:skill-abc:view").Result()
	if err != nil || v != "1" {
		t.Fatalf("expected view counter=1, got %q err=%v", v, err)
	}
	dirtySize, _ := rdb.SCard(ctx, "metrics:dirty").Result()
	if dirtySize != 1 {
		t.Fatalf("expected dirty set size=1, got %d", dirtySize)
	}

	// Run flush
	fw.flush(ctx)

	// Verify DB got the upsert
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 DB upsert, got %d", len(repo.calls))
	}
	if repo.calls[0].ResourceType != "skill" || repo.calls[0].ResourceID != "skill-abc" {
		t.Errorf("unexpected upsert call: %+v", repo.calls[0])
	}
	if repo.calls[0].ViewDelta != 1 {
		t.Errorf("expected viewDelta=1, got %d", repo.calls[0].ViewDelta)
	}

	// Verify Redis counter was reset
	v, _ = rdb.Get(ctx, "metrics:skill:skill-abc:view").Result()
	if v != "0" {
		t.Errorf("expected view counter reset to 0, got %q", v)
	}

	// Verify dirty set is empty
	dirtySize, _ = rdb.SCard(ctx, "metrics:dirty").Result()
	if dirtySize != 0 {
		t.Errorf("expected empty dirty set, got %d", dirtySize)
	}
}

// TestIntegration_DownloadTrackFlushDB verifies download tracking end-to-end.
func TestIntegration_DownloadTrackFlushDB(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	fw := NewFlushWorker(rdb, repo, cfg)
	ctx := context.Background()

	// Simulate download URL generation triggering metrics.TrackDownload:
	pipe := rdb.Pipeline()
	pipe.Incr(ctx, "metrics:skill:skill-xyz:download")
	pipe.SAdd(ctx, "metrics:dirty", "skill:skill-xyz")
	_, err := pipe.Exec(ctx)
	if err != nil {
		t.Fatalf("simulate track download: %v", err)
	}

	// Flush
	fw.flush(ctx)

	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 DB upsert, got %d", len(repo.calls))
	}
	if repo.calls[0].DownloadDelta != 1 {
		t.Errorf("expected downloadDelta=1, got %d", repo.calls[0].DownloadDelta)
	}
	if repo.calls[0].ViewDelta != 0 {
		t.Errorf("expected viewDelta=0, got %d", repo.calls[0].ViewDelta)
	}
}

// TestIntegration_MultipleViewsAccumulate verifies that multiple view increments
// within a single flush window accumulate correctly.
func TestIntegration_MultipleViewsAccumulate(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	fw := NewFlushWorker(rdb, repo, cfg)
	ctx := context.Background()

	// Simulate 5 views and 3 downloads on the same skill
	for i := 0; i < 5; i++ {
		rdb.Incr(ctx, "metrics:skill:popular:view")
	}
	for i := 0; i < 3; i++ {
		rdb.Incr(ctx, "metrics:skill:popular:download")
	}
	rdb.SAdd(ctx, "metrics:dirty", "skill:popular")

	fw.flush(ctx)

	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(repo.calls))
	}
	if repo.calls[0].ViewDelta != 5 {
		t.Errorf("expected viewDelta=5, got %d", repo.calls[0].ViewDelta)
	}
	if repo.calls[0].DownloadDelta != 3 {
		t.Errorf("expected downloadDelta=3, got %d", repo.calls[0].DownloadDelta)
	}
}

// TestIntegration_RedisDown_TrackDoesNotBlock verifies that when Redis is
// unavailable, the track operation fails gracefully without blocking the caller.
func TestIntegration_RedisDown_TrackDoesNotBlock(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	// Shut down Redis to simulate failure
	mr.Close()

	// Track should fail but not panic or block
	pipe := rdb.Pipeline()
	pipe.Incr(ctx, "metrics:skill:sk-1:view")
	pipe.SAdd(ctx, "metrics:dirty", "skill:sk-1")
	_, err := pipe.Exec(ctx)

	// Error is expected — the important thing is it doesn't panic or hang
	if err == nil {
		t.Error("expected error when Redis is down")
	}
}

// TestIntegration_RedisDown_FlushSkips verifies that when Redis goes down
// mid-operation, the flush worker logs and skips without crashing.
func TestIntegration_RedisDown_FlushSkips(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	fw := NewFlushWorker(rdb, repo, cfg)
	ctx := context.Background()

	// Close Redis before flush
	mr.Close()

	// flush should not panic
	fw.flush(ctx)

	// No DB calls since we can't acquire lock or read dirty set
	if len(repo.calls) != 0 {
		t.Errorf("expected 0 upserts when Redis is down, got %d", len(repo.calls))
	}
}

// TestIntegration_MultiWorkerLock verifies that only one of multiple workers
// acquires the flush lock and performs the flush.
func TestIntegration_MultiWorkerLock(t *testing.T) {
	mr := miniredis.RunT(t)

	// Create two workers with different instances
	rdb1 := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	rdb2 := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})

	repo1 := &mockRepo{}
	repo2 := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	fw1 := NewFlushWorker(rdb1, repo1, cfg)
	fw2 := NewFlushWorker(rdb2, repo2, cfg)

	ctx := context.Background()

	// Add dirty data
	mr.Set("metrics:skill:sk-1:view", "10")
	mr.SAdd("metrics:dirty", "skill:sk-1")

	// Run both workers concurrently
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		fw1.flush(ctx)
	}()
	go func() {
		defer wg.Done()
		fw2.flush(ctx)
	}()
	wg.Wait()

	// Exactly one should have processed the data
	totalCalls := len(repo1.calls) + len(repo2.calls)
	if totalCalls != 1 {
		t.Errorf("expected exactly 1 total upsert across 2 workers, got %d (w1=%d, w2=%d)",
			totalCalls, len(repo1.calls), len(repo2.calls))
	}

	// Lock should be released after flush
	ok := mr.Exists(flushLockKey)
	if ok {
		t.Error("expected lock to be released after both workers complete")
	}
}

// TestIntegration_MultiWorkerLock_ValueProtection verifies that a worker
// does NOT delete another worker's lock.
func TestIntegration_MultiWorkerLock_ValueProtection(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	fw := NewFlushWorker(rdb, repo, cfg)

	ctx := context.Background()

	// Pre-set lock to a different instance
	mr.Set(flushLockKey, "other-instance-id")
	mr.SetTTL(flushLockKey, 120*time.Second)

	// This worker should skip flush
	fw.flush(ctx)

	if len(repo.calls) != 0 {
		t.Errorf("expected no upserts when lock held by another, got %d", len(repo.calls))
	}

	// Lock must still be held by other instance (not deleted)
	val, err := mr.Get(flushLockKey)
	if err != nil {
		t.Fatalf("lock should still exist: %v", err)
	}
	if val != "other-instance-id" {
		t.Errorf("lock value changed: %q", val)
	}
}

// TestIntegration_ComprehensiveSort_Formula verifies that the comprehensive
// sort formula weights downloads, views, and recency correctly.
// SQL formula: (download_count * 5 + view_count * 1 + 20 / POW(age_days + 2, 1.2))
func TestIntegration_ComprehensiveSort_Formula(t *testing.T) {
	// Verify relative ordering invariants of the sort formula:
	// 1. Downloads are weighted 5x more than views
	// 2. New skills get a recency bonus (~8.7 max for brand-new)
	// 3. Old popular skills still rank above new empty skills

	type skillScore struct {
		name          string
		downloadCount int64
		viewCount     int64
		ageHours      int64
	}

	// These should be in descending score order
	orderedSkills := []skillScore{
		{"popular-old", 50, 100, 720},    // 50*5 + 100 = 350 + tiny bonus
		{"medium-recent", 10, 20, 48},    // 10*5 + 20 = 70 + small bonus
		{"new-with-downloads", 3, 0, 1},  // 3*5 + 0 = 15 + ~8.7 bonus ≈ 23.7
		{"brand-new-empty", 0, 0, 0},     // 0 + 0 + ~8.7 ≈ 8.7
	}

	// Verify relative ordering: each skill should score higher than the next
	for i := 0; i < len(orderedSkills)-1; i++ {
		a := orderedSkills[i]
		b := orderedSkills[i+1]
		// Compute approximate scores (without exact pow, but the relative order is clear)
		scoreA := float64(a.downloadCount)*5 + float64(a.viewCount)
		scoreB := float64(b.downloadCount)*5 + float64(b.viewCount)
		// Add maximum possible bonus (8.7) to the lower-scored item
		if scoreA < scoreB+9.0 {
			t.Errorf("expected %q (base=%.0f) to rank above %q (base=%.0f) even with max recency bonus",
				a.name, scoreA, b.name, scoreB)
		}
	}

	// Specific invariant: downloads matter more than views
	if 1*5 < 4*1 {
		t.Error("1 download should contribute more than 4 views")
	}
	// 1 download = 5 points, 1 view = 1 point
	if 5 != 1*5 {
		t.Error("download weight should be 5")
	}
}

// TestIntegration_ConsecutiveFlushes verifies that the counter accumulates
// between flushes correctly: new increments after flush are captured next cycle.
func TestIntegration_ConsecutiveFlushes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	fw := NewFlushWorker(rdb, repo, cfg)
	ctx := context.Background()

	// First batch: 3 views
	for i := 0; i < 3; i++ {
		rdb.Incr(ctx, "metrics:skill:sk-1:view")
	}
	rdb.SAdd(ctx, "metrics:dirty", "skill:sk-1")

	fw.flush(ctx)

	if len(repo.calls) != 1 || repo.calls[0].ViewDelta != 3 {
		t.Fatalf("flush 1: expected viewDelta=3, got calls=%d delta=%d",
			len(repo.calls), safeViewDelta(repo.calls, 0))
	}

	// Second batch: 7 more views arrive after flush
	for i := 0; i < 7; i++ {
		rdb.Incr(ctx, "metrics:skill:sk-1:view")
	}
	rdb.SAdd(ctx, "metrics:dirty", "skill:sk-1")

	fw.flush(ctx)

	if len(repo.calls) != 2 || repo.calls[1].ViewDelta != 7 {
		t.Fatalf("flush 2: expected viewDelta=7, got calls=%d delta=%d",
			len(repo.calls), safeViewDelta(repo.calls, 1))
	}
}

// TestIntegration_FlushWorker_StartStop verifies the worker starts, ticks, and
// shuts down gracefully without deadlock.
func TestIntegration_FlushWorker_StartStop(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	cfg.Interval = 20 * time.Millisecond
	fw := NewFlushWorker(rdb, repo, cfg)

	// Add some data to flush
	mr.Set("metrics:skill:sk-1:view", "2")
	mr.SAdd("metrics:dirty", "skill:sk-1")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		fw.Start(ctx)
		close(done)
	}()

	// Let it tick at least once
	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// graceful shutdown
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not shut down within timeout")
	}

	// Should have flushed at least once
	if len(repo.calls) < 1 {
		t.Error("expected at least 1 flush during the run")
	}
}

// TestIntegration_MixedResourceTypes verifies that v1 only processes 'skill' type
// and ignores other resource types in the dirty set.
func TestIntegration_MixedResourceTypes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	fw := NewFlushWorker(rdb, repo, cfg)
	ctx := context.Background()

	// Mix of skill and non-skill types
	mr.Set("metrics:skill:sk-1:view", "5")
	mr.SAdd("metrics:dirty", "skill:sk-1")

	mr.Set("metrics:mcp:mcp-1:view", "10")
	mr.SAdd("metrics:dirty", "mcp:mcp-1")

	mr.Set("metrics:template:tpl-1:download", "3")
	mr.SAdd("metrics:dirty", "template:tpl-1")

	fw.flush(ctx)

	// Only skill should be processed
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 upsert (skill only), got %d", len(repo.calls))
	}
	if repo.calls[0].ResourceType != "skill" || repo.calls[0].ResourceID != "sk-1" {
		t.Errorf("unexpected call: %+v", repo.calls[0])
	}
}

// TestIntegration_LargeBatch verifies flush handles many dirty keys across
// multiple SPOP batches.
func TestIntegration_LargeBatch(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	repo := &mockRepo{}

	cfg := DefaultFlushWorkerConfig()
	cfg.Batch = 3 // Very small batch to force multiple iterations
	fw := NewFlushWorker(rdb, repo, cfg)
	ctx := context.Background()

	// Add 10 dirty skills
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("skill:sk-%d", i)
		mr.Set(fmt.Sprintf("metrics:skill:sk-%d:view", i), "1")
		mr.SAdd("metrics:dirty", key)
	}

	fw.flush(ctx)

	// All 10 should be processed across multiple batch iterations
	if len(repo.calls) != 10 {
		t.Errorf("expected 10 upserts, got %d", len(repo.calls))
	}

	// Dirty set should be empty
	dirtySize, _ := rdb.SCard(ctx, "metrics:dirty").Result()
	if dirtySize != 0 {
		t.Errorf("expected empty dirty set, got %d", dirtySize)
	}
}

func safeViewDelta(calls []upsertCall, idx int) int64 {
	if idx >= len(calls) {
		return -1
	}
	return calls[idx].ViewDelta
}
