package usecase

import (
	"context"
	"errors"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"testing"
)

type watchRepoFake struct {
	calls                       int
	accountID, newID, deletedID int64
	crop, market                string
	result                      domain.MarketWatchTargetResult
	err                         error
}

func (f *watchRepoFake) Create(_ context.Context, a, id int64, crop, market string) (domain.MarketWatchTargetResult, error) {
	f.calls++
	f.accountID = a
	f.newID = id
	f.crop = crop
	f.market = market
	return f.result, f.err
}
func (f *watchRepoFake) Delete(_ context.Context, a, id int64) error {
	f.calls++
	f.accountID = a
	f.deletedID = id
	return f.err
}
func TestMarketWatchIdentityValidationAndRepositoryResult(t *testing.T) {
	repo := &watchRepoFake{result: domain.MarketWatchTargetResult{WatchTargetID: 44, Created: false}}
	u := NewMarketWatch(repo, func() int64 { return 99 })
	ctx := context.Background()
	empty := ""
	for _, market := range []*string{nil, &empty} {
		crop := ""
		if market != nil {
			crop = "A1"
		}
		if _, err := u.Create(ctx, 123, crop, market); !errors.Is(err, domain.ErrInvalidWatchTarget) {
			t.Fatal("invalid create accepted")
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid write touched repo")
	}
	got, err := u.Create(ctx, 123, "A1", nil)
	if err != nil || got.WatchTargetID != 44 || got.Created || repo.newID != 99 || repo.accountID != 123 || repo.market != "" {
		t.Fatalf("create %+v %v", got, err)
	}
	for _, id := range []string{"", "-1", "0", "+1", "1.2", "9223372036854775808"} {
		if err := u.Delete(ctx, 123, id); !errors.Is(err, domain.ErrInvalidWatchTarget) {
			t.Fatalf("accepted id %q", id)
		}
	}
	if err := u.Delete(ctx, 123, "44"); err != nil || repo.deletedID != 44 || repo.accountID != 123 {
		t.Fatal("delete actor/id lost")
	}
	repo.err = domain.ErrWatchLimit
	if _, err := u.Create(ctx, 123, "B1", nil); !errors.Is(err, domain.ErrWatchLimit) {
		t.Fatal("watch limit lost")
	}
}
