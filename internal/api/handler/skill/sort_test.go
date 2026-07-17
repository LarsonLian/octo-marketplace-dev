package skill

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mininglamp-OSS/octo-marketplace/internal/middleware"
	"github.com/Mininglamp-OSS/octo-marketplace/internal/model"
	skillrepo "github.com/Mininglamp-OSS/octo-marketplace/internal/repository/skill"
	"github.com/gin-gonic/gin"
)

func TestListInvalidSortReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	auth := middleware.NewAuthenticator(false, nil, model.Identity{
		UID:  "user-1",
		Name: "Alice",
	}, "space-1")
	rg := r.Group("/api/v1", auth.Handler())
	h := New(nil) // no actual service needed for validation
	h.Register(rg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/skills?sort=invalid_sort", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestListValidSortModes(t *testing.T) {
	for _, mode := range []string{
		skillrepo.SortComprehensive,
		skillrepo.SortLatest,
		skillrepo.SortDownloads,
		skillrepo.SortViews,
	} {
		if !validSortModes[mode] {
			t.Errorf("sort mode %q should be valid", mode)
		}
	}
}

func TestParseOffset(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"0", 0},
		{"-1", 0},
		{"abc", 0},
		{"10", 10},
		{"100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseOffset(tt.input)
			if got != tt.expected {
				t.Errorf("parseOffset(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
