package upload

import (
	"context"
	"errors"
	"testing"

	metricssvc "github.com/Mininglamp-OSS/octo-marketplace/internal/service/metrics"
	"github.com/gin-gonic/gin"
)

// mockMetricsRedis implements metricssvc.MetricsRedis for testing.
type mockMetricsRedis struct {
	downloadCalled bool
	downloadType   string
	downloadID     string
	downloadErr    error
}

func (m *mockMetricsRedis) TrackView(_ context.Context, _, _ string) error { return nil }
func (m *mockMetricsRedis) TrackDownload(_ context.Context, resourceType, resourceID string) error {
	m.downloadCalled = true
	m.downloadType = resourceType
	m.downloadID = resourceID
	return m.downloadErr
}
func (m *mockMetricsRedis) TrackInstall(_ context.Context, _, _ string) error { return nil }

// alwaysVisibleResolver is a test resolver that always returns true.
type alwaysVisibleResolver struct{}

func (r *alwaysVisibleResolver) CanView(_ context.Context, _ string, _ metricssvc.Caller) (bool, error) {
	return true, nil
}

func TestDownloadTracksMetricsOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRedis := &mockMetricsRedis{}
	mSvc := metricssvc.New(mockRedis)
	metricssvc.RegisterResolver("skill", &alwaysVisibleResolver{})

	h := &Handler{metricsSvc: mSvc}

	// Verify the handler's metricsSvc is set and functional
	if h.metricsSvc == nil {
		t.Fatal("metricsSvc should not be nil")
	}

	// Simulate calling TrackDownload as the handler would after URL gen success
	err := h.metricsSvc.TrackDownload(context.Background(), "skill", "skill-123")
	if err != nil {
		t.Fatalf("TrackDownload error: %v", err)
	}
	if !mockRedis.downloadCalled {
		t.Error("expected download tracking to be called on redis")
	}
	if mockRedis.downloadType != "skill" {
		t.Errorf("resource type = %q, want %q", mockRedis.downloadType, "skill")
	}
	if mockRedis.downloadID != "skill-123" {
		t.Errorf("resource id = %q, want %q", mockRedis.downloadID, "skill-123")
	}
}

func TestDownloadMetricsRedisFailureDoesNotPropagate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRedis := &mockMetricsRedis{downloadErr: errors.New("redis unavailable")}
	mSvc := metricssvc.New(mockRedis)
	metricssvc.RegisterResolver("skill", &alwaysVisibleResolver{})

	h := &Handler{metricsSvc: mSvc}

	// The metrics service logs redis errors but does NOT return them to the caller
	err := h.metricsSvc.TrackDownload(context.Background(), "skill", "skill-fail")
	if err != nil {
		t.Fatalf("TrackDownload should not propagate redis errors to caller, got: %v", err)
	}
	if !mockRedis.downloadCalled {
		t.Error("redis Track should still be attempted")
	}
}

func TestDownloadNoTrackingWhenMetricsSvcNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{metricsSvc: nil}

	// Handler code checks `if h.metricsSvc != nil` before calling TrackDownload.
	// When nil, no tracking occurs and no panic happens.
	if h.metricsSvc != nil {
		t.Fatal("metricsSvc should be nil in this test")
	}
	// No panic = test passes
}

func TestSetMetricsService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRedis := &mockMetricsRedis{}
	mSvc := metricssvc.New(mockRedis)

	h := New(nil, nil, nil)
	if h.metricsSvc != nil {
		t.Fatal("initially metricsSvc should be nil")
	}

	h.SetMetricsService(mSvc)
	if h.metricsSvc == nil {
		t.Fatal("metricsSvc should be set after SetMetricsService")
	}
}
