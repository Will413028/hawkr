package characterize

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
)

type noFollowers struct{}

func (noFollowers) SubscriberCounts(context.Context, []int64) (map[int64]int64, error) {
	return nil, nil
}

type noRatings struct{}

func (noRatings) StoreRatingStats(context.Context, []int64) (map[int64]httpseller.RatingStats, error) {
	return nil, nil
}

// Stats 是 httpapi.Deps.Stats 的測試替身：粉絲與評價都沒有資料（卡片與統計的欄位不設）。只給測試 import。
func Stats(t testing.TB) *httpseller.StatsSource {
	t.Helper()
	s, err := httpseller.NewStatsSource(noFollowers{}, noRatings{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// WebOrigin 是 httpapi.Deps.WebOrigin 的測試用網域。只給測試 import。
func WebOrigin(t testing.TB) rpc.WebOrigin {
	t.Helper()
	w, err := rpc.NewWebOrigin("https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return w
}
