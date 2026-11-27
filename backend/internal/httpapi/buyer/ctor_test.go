package buyer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

type noFollowerCounts struct{}

func (noFollowerCounts) SubscriberCounts(context.Context, []int64) (map[int64]int64, error) {
	return nil, nil
}

type noRatings struct{}

func (noRatings) StoreRatingStats(context.Context, []int64) (map[int64]httpseller.RatingStats, error) {
	return nil, nil
}

type unusedBlocking struct{}

func (unusedBlocking) ContentAuthorFor(context.Context, social.Caller, social.Target) (social.ContentAuthor, error) {
	panic("unused")
}
func (unusedBlocking) GetReview(context.Context, uuid.UUID, int64) (review.Review, error) {
	panic("unused")
}
func (unusedBlocking) MessageParties(context.Context, uuid.UUID) (im.ConversationParties, error) {
	panic("unused")
}
func (unusedBlocking) AccountIDByBuyerID(context.Context, int64) (int64, error) { panic("unused") }
func (unusedBlocking) Store(context.Context, int64) (seller.Store, error)       { panic("unused") }

func unusedSources() blocks.Sources {
	return blocks.Sources{Authors: unusedBlocking{}, Reviews: unusedBlocking{}, Messages: unusedBlocking{}, Identities: blocks.Identities{Buyers: unusedBlocking{}, Stores: unusedBlocking{}}}
}

func mustApp(t testing.TB, m Module, images rpc.Images, ports Ports, stats ...*httpseller.StatsSource) *App {
	t.Helper()
	return mustAppWith(t, m, images, ports, unusedSources(), stats...)
}

func mustAppWith(t testing.TB, m Module, images rpc.Images, ports Ports, blocking blocks.Sources, stats ...*httpseller.StatsSource) *App {
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
	h, err := NewApp(m, images, ports, s, blocking)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestNewAppRejectsANilStatsSource(t *testing.T) {
	if _, err := NewApp(fakeModule{}, rpc.Images{}, Ports{}, nil, unusedSources()); err == nil {
		t.Fatal("NewApp 收下了 nil 的 StatsSource")
	}
}

func TestNewAppRejectsIncompleteBlockingSources(t *testing.T) {
	s, err := httpseller.NewStatsSource(noFollowerCounts{}, noRatings{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp(fakeModule{}, rpc.Images{}, Ports{}, s, blocks.Sources{}); !errors.Is(err, ErrBlockingRequired) {
		t.Fatalf("缺封鎖來源要回 ErrBlockingRequired：%v", err)
	}
}
