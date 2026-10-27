package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParsePagePageSize_NormalizesBounds(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantPage int32
		wantSize int32
	}{
		{
			name:     "uses defaults",
			wantPage: 1,
			wantSize: 20,
		},
		{
			name:     "clamps page size",
			query:    "?page=3&page_size=999",
			wantPage: 3,
			wantSize: maxPostPageSize,
		},
		{
			name:     "invalid values use defaults",
			query:    "?page=-1&page_size=not-a-number",
			wantPage: 1,
			wantSize: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/posts"+tt.query, nil)

			page, pageSize := parsePagePageSize(c, 20, maxPostPageSize)
			if page != tt.wantPage || pageSize != tt.wantSize {
				t.Errorf("parsePagePageSize() = (%d, %d), want (%d, %d)", page, pageSize, tt.wantPage, tt.wantSize)
			}
		})
	}
}
