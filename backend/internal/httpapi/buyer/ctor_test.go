package buyer

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
)

type noFollowerCounts struct{}

func (noFollowerCounts) SubscriberCounts(context.Context, []int64) (map[int64]int64, error) {
	return nil, nil
}

type noRatings struct{}

func (noRatings) StoreRatingStats(context.Context, []int64) (map[int64]httpseller.RatingStats, error) {
	return nil, nil
}

// mustApp 是測試的建構：StatsSource 是必要的（零值被建構子擋下）；沒有指定的測試給一份沒有任何資料的來源。
func mustApp(t testing.TB, m Module, images rpc.Images, ports Ports, stats ...*httpseller.StatsSource) *App {
	t.Helper()
	var s *httpseller.StatsSource
	if len(stats) > 0 {
		s = stats[0]
	} else {
		var err error
		if s, err = httpseller.NewStatsSource(noFollowerCounts{}, noRatings{}, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
			t.Fatal(err)
		}
	}
	h, err := NewApp(m, images, ports, s)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestNewAppRejectsANilStatsSource(t *testing.T) {
	if _, err := NewApp(fakeModule{}, rpc.Images{}, Ports{}, nil); err == nil {
		t.Fatal("NewApp 收下了 nil 的 StatsSource")
	}
}
