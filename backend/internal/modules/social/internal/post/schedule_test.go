package post

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type failingStores struct{ err error }

func (s failingStores) Store(context.Context, int64) (seller.Store, error) {
	return seller.Store{}, s.err
}

func TestOwnerAuthorizerDeniesOnlyPermanentLossOfTheRight(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		err    error
		denied bool
	}{
		{"no right", apperr.PermissionDenied("停權"), true},
		{"store gone", apperr.NotFound("店家已關閉"), true},
		{"precondition missing", apperr.FailedPrecondition("等待刪除"), false},
		{"transient", errors.New("連線中斷"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := ownerAuthorizer{stores: failingStores{err: c.err}}.AuthorizeScheduledPublish(context.Background(), model.ScheduledPublish{StoreID: 1})
			if errors.Is(err, model.ErrScheduleDenied) != c.denied {
				t.Fatalf("denied = %v，要 %v：%v", errors.Is(err, model.ErrScheduleDenied), c.denied, err)
			}
			if err == nil {
				t.Fatal("錯誤不能被吞掉")
			}
		})
	}
}
