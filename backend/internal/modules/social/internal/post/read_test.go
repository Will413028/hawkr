package post

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 窗口驗證在碰資料庫之前：超過上限的窗口與空窗口都是 InvalidInput（零值 Service 沒有 DB，驗證沒擋住就會 panic）。
func TestTrendingHashtagsRejectsWindowsAboveTheMaximum(t *testing.T) {
	t.Parallel()
	to := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	for name, from := range map[string]time.Time{
		"30 天多 1 微秒": to.Add(-model.MaxTrendingWindow - time.Microsecond),
		"90 天":       to.AddDate(0, 0, -90),
		"365 天":      to.AddDate(-1, 0, 0),
		"空窗口":        to,
	} {
		_, err := (&Service{}).TrendingHashtags(context.Background(), from, to, 10)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
}
