package handler

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListParamsNormalizesRepeatedAndCommaValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, raw := range []string{"/mcps?category=dev&category=search&transport=stdio", "/mcps?category=dev,search&transport=stdio"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", raw, nil)
		p, _, _ := listParams(c)
		if !reflect.DeepEqual(p.Categories, []string{"dev", "search"}) {
			t.Fatalf("%s categories=%v", raw, p.Categories)
		}
		if !reflect.DeepEqual(p.Transports, []string{"stdio"}) {
			t.Fatalf("%s transports=%v", raw, p.Transports)
		}
	}
}
