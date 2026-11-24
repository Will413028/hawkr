package intake

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

type blockResolver struct {
	registry.Resolver
	buyerPairs     map[[2]int64]bool
	buyerBlocksStr map[[2]int64]bool
	strBlocksBuyer map[[2]int64]bool
	err            error
}

func (b blockResolver) BuyersBlockedBetween(_ context.Context, x, y int64) (bool, error) {
	return b.buyerPairs[[2]int64{x, y}] || b.buyerPairs[[2]int64{y, x}], b.err
}

func (b blockResolver) BuyerBlocksStore(_ context.Context, buyer, store int64) (bool, error) {
	return b.buyerBlocksStr[[2]int64{buyer, store}], b.err
}

func (b blockResolver) StoreBlocksBuyer(_ context.Context, store, buyer int64) (bool, error) {
	return b.strBlocksBuyer[[2]int64{store, buyer}], b.err
}

func buyerP(id int64) registry.Principal {
	return registry.Principal{Side: model.AudienceBuyer, ID: id}
}
func storeP(id int64) registry.Principal {
	return registry.Principal{Side: model.AudienceStore, ID: id}
}

func actors(ps ...registry.Principal) []registry.Actor {
	out := make([]registry.Actor, len(ps))
	for i, p := range ps {
		out[i] = registry.Actor{Principal: p, AccountID: 1000 + p.ID}
	}
	return out
}

func TestBlockedCoversEveryPairAndDirection(t *testing.T) {
	t.Parallel()
	r := blockResolver{
		buyerPairs:     map[[2]int64]bool{{1, 2}: true},
		buyerBlocksStr: map[[2]int64]bool{{3, 30}: true},
		strBlocksBuyer: map[[2]int64]bool{{40, 4}: true},
	}
	cases := []struct {
		name   string
		self   registry.Principal
		actors []registry.Actor
		want   bool
	}{
		{"買家對買家：收件人封鎖 actor", buyerP(1), actors(buyerP(2)), true},
		{"買家對買家：actor 封鎖收件人", buyerP(2), actors(buyerP(1)), true},
		{"買家對買家：沒有封鎖", buyerP(1), actors(buyerP(5)), false},
		{"收件買家封鎖 actor 店", buyerP(3), actors(storeP(30)), true},
		{"actor 店封鎖收件買家", buyerP(4), actors(storeP(40)), true},
		{"收件店被 actor 買家封鎖", storeP(30), actors(buyerP(3)), true},
		{"收件店封鎖 actor 買家", storeP(40), actors(buyerP(4)), true},
		{"買家與店沒有封鎖", buyerP(3), actors(storeP(40)), false},
		{"買家號碼與店號碼是兩個空間：買家 30 不是店 30", buyerP(30), actors(storeP(3)), false},
		{"店與店沒有封鎖", storeP(30), actors(storeP(40)), false},
		{"任一位 actor 有封鎖就略過", buyerP(1), actors(buyerP(5), buyerP(2)), true},
		{"沒有 actor", buyerP(1), nil, false},
		{"沒有 Self（參數裡已有帳號的型別）", registry.Principal{}, actors(buyerP(2)), false},
	}
	for _, c := range cases {
		got, err := Blocked(context.Background(), r, c.self, c.actors)
		if err != nil || got != c.want {
			t.Errorf("%s：Blocked = %v（%v），預期 %v", c.name, got, err, c.want)
		}
	}
}

func TestBlockedReturnsReadErrorsInsteadOfDelivering(t *testing.T) {
	t.Parallel()
	boom := errors.New("db down")
	got, err := Blocked(context.Background(), blockResolver{err: boom}, buyerP(1), actors(buyerP(2)))
	if !errors.Is(err, boom) || got {
		t.Fatalf("Blocked = %v（%v），讀不到封鎖時要回錯誤讓 job 重試，不能當作沒封鎖", got, err)
	}
}
