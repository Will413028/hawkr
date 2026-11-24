package rfq

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeStores struct {
	blocked, closed map[int64]struct{}
	blockErr        error
	openErr         error
}

func (f fakeStores) StoreIDsBlockingBuyer(context.Context, int64, []int64) (map[int64]struct{}, error) {
	return f.blocked, f.blockErr
}

func (f fakeStores) OpenStoreIDs(_ context.Context, ids []int64) (map[int64]struct{}, error) {
	out := map[int64]struct{}{}
	for _, id := range ids {
		if _, shut := f.closed[id]; !shut {
			out[id] = struct{}{}
		}
	}
	return out, f.openErr
}

func serviceWith(f fakeStores) Service { return Service{Blocks: f, Open: f} }

func TestCheckStoresPassesWhenEveryStoreIsOpenAndNotBlocking(t *testing.T) {
	if err := serviceWith(fakeStores{}).checkStores(context.Background(), 1, []int64{10, 20}, "msg"); err != nil {
		t.Fatal(err)
	}
}

func TestCheckStoresRejectsABlockingOrClosedStoreWithTheSameConflict(t *testing.T) {
	for name, f := range map[string]fakeStores{
		"封鎖": {blocked: map[int64]struct{}{20: {}}},
		"打烊": {closed: map[int64]struct{}{20: {}}},
	} {
		err := serviceWith(f).checkStores(context.Background(), 1, []int64{10, 20}, "msg")
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindVersionConflict || ae.Metadata["conflictTarget"] != TargetCandidateSet {
			t.Fatalf("%s：要 VERSION_CONFLICT candidate_set，得到 %v", name, err)
		}
		if strings.Contains(err.Error(), "20") {
			t.Fatalf("%s：訊息不得帶店 ID：%v", name, err)
		}
	}
}

func TestCheckStoresFailsClosedOnAnyPortError(t *testing.T) {
	boom := errors.New("boom")
	for name, f := range map[string]fakeStores{"封鎖查詢": {blockErr: boom}, "營業查詢": {openErr: boom}} {
		err := serviceWith(f).checkStores(context.Background(), 1, []int64{10}, "msg")
		if !errors.Is(err, boom) || apperr.KindOf(err) == apperr.KindVersionConflict {
			t.Fatalf("%s：要原樣回傳一般錯誤，得到 %v", name, err)
		}
	}
}

func TestCheckStoresWithoutPortsIsAnAssemblyError(t *testing.T) {
	if err := (Service{}).checkStores(context.Background(), 1, []int64{10}, "msg"); err == nil || apperr.KindOf(err) == apperr.KindVersionConflict {
		t.Fatalf("沒給 port 要回組裝錯誤，得到 %v", err)
	}
	f := fakeStores{}
	if err := (Service{Blocks: f}).checkStores(context.Background(), 1, []int64{10}, "msg"); err == nil {
		t.Fatal("缺 Open 也要失敗")
	}
}
