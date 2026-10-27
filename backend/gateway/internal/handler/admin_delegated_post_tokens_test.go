package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 貼文是一張圖加文字的卡片：代行建立與修改最多 1 個 stagedToken（B1）。
// 多於 1 個在這裡就拒絕，不再放行第 2–6 張給 social 靜默丟掉。
func TestValidatePostStagedTokens_AtMostOne(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name   string
		tokens []string
		want   bool
	}{
		{"沒有", nil, true},
		{"一個", []string{"a"}, true},
		{"兩個", []string{"a", "b"}, false},
		{"六個", []string{"a", "b", "c", "d", "e", "f"}, false},
		{"空字串", []string{""}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/", nil)
			if got := validatePostStagedTokens(ctx, c.tokens); got != c.want {
				t.Fatalf("validatePostStagedTokens(%v) = %v, want %v", c.tokens, got, c.want)
			}
		})
	}
}
