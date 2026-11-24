package admin

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/campaign"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/voucher"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var errStop = errors.New("stop before the database")

// fakeBuyers 記下每次查詢；stop 為 true 時查完回 errStop，讓測試在碰到資料庫之前結束。
type fakeBuyers struct {
	missing map[int64]bool
	stop    bool
	calls   [][]int64
}

func (f *fakeBuyers) Existing(_ context.Context, ids []int64) ([]int64, error) {
	f.calls = append(f.calls, slices.Clone(ids))
	var out []int64
	for _, id := range ids {
		if !f.missing[id] {
			out = append(out, id)
		}
	}
	if f.stop {
		return nil, errStop
	}
	return out, nil
}

type fakeDirectory struct {
	state map[int64]StoreState
	asked []int64
}

func (f *fakeDirectory) StoreState(_ context.Context, id int64) (StoreState, error) {
	f.asked = append(f.asked, id)
	return f.state[id], nil
}

func newService(b *fakeBuyers, d *fakeDirectory) *Service {
	return New(Deps{Campaign: campaign.New(campaign.Deps{}), Voucher: voucher.New(voucher.Deps{}), Buyers: b, Directory: d})
}

func fields(t *testing.T, err error) []string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("不是 apperr：%v", err)
	}
	var out []string
	for _, v := range ae.FieldViolations() {
		out = append(out, v.Field)
	}
	return out
}

func ids(n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(i + 1)
	}
	return out
}

var actor = adminaudit.Actor{AdminID: 7, Action: "test"}

// 原始名單長度超過上限立即拒絕，不查買家；上限內先去重再算 1 到 MaxIssueBatch：500 個不重複加重複的通過、501 個不重複被拒。
func TestIssueBoundsTheRawListAndDeduplicatesBeforeTheBatchLimit(t *testing.T) {
	b := &fakeBuyers{stop: true}
	s := newService(b, &fakeDirectory{})

	_, err := s.Issue(context.Background(), actor, voucher.IssueInput{BuyerIDs: ids(model.MaxIssueRequestBuyers + 1)})
	if apperr.KindOf(err) != apperr.KindInvalidInput || len(b.calls) != 0 {
		t.Fatalf("原始長度超過上限：err=%v 查詢=%d 次", err, len(b.calls))
	}

	_, err = s.Issue(context.Background(), actor, voucher.IssueInput{BuyerIDs: ids(model.MaxIssueBatch + 1)})
	if apperr.KindOf(err) != apperr.KindInvalidInput || len(b.calls) != 0 {
		t.Fatalf("501 個不重複：err=%v 查詢=%d 次", err, len(b.calls))
	}

	raw := append(ids(model.MaxIssueBatch), 1, 2, 3)
	if _, err = s.Issue(context.Background(), actor, voucher.IssueInput{BuyerIDs: raw}); !errors.Is(err, errStop) {
		t.Fatalf("500 個不重複加重複：err=%v，預期通過上限檢查走到買家查詢", err)
	}
	if len(b.calls) != 1 || len(b.calls[0]) != model.MaxIssueBatch {
		t.Fatalf("買家查詢收到的人數不是去重後的 %d 位：%v", model.MaxIssueBatch, len(b.calls))
	}
}

// 買家不存在：指出第一個在請求陣列的位置（重複的不影響位置），不碰資料庫。
func TestIssueRejectsAnUnknownBuyerAtTheFirstRequestIndex(t *testing.T) {
	b := &fakeBuyers{missing: map[int64]bool{12: true, 13: true}}
	s := newService(b, &fakeDirectory{})
	_, err := s.Issue(context.Background(), actor, voucher.IssueInput{BuyerIDs: []int64{9, 9, 13, 12}})
	if apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Equal(fields(t, err), []string{"BuyerIDs[2]"}) {
		t.Fatalf("err=%v 欄位=%v，預期 invalid_input 指出 BuyerIDs[2]", err, fields(t, err))
	}
}

// DESIGNATED 的名單與限店的店家在寫入前檢查：名單指出第一個不存在的位置；店家不存在或已關閉指出 StoreID，停權不擋。
func TestCreateCampaignChecksTheTargetListAndTheStoreBeforeWriting(t *testing.T) {
	b := &fakeBuyers{missing: map[int64]bool{12: true}}
	d := &fakeDirectory{state: map[int64]StoreState{11: {Exists: true}, 12: {Exists: true, Closed: true}}}
	s := newService(b, d)
	spec := model.CampaignSpec{Title: "t"}

	_, err := s.CreateCampaign(context.Background(), actor, campaign.CreateInput{CampaignSpec: spec, TargetType: model.TargetDesignated, TargetBuyers: []int64{9, 10, 12}})
	if !slices.Equal(fields(t, err), []string{"TargetBuyers[2]"}) {
		t.Fatalf("名單：err=%v 欄位=%v", err, fields(t, err))
	}
	for name, store := range map[string]int64{"不存在": 99, "已關閉": 12} {
		_, err = s.CreateCampaign(context.Background(), actor, campaign.CreateInput{CampaignSpec: spec, TargetType: model.TargetAll, StoreID: store})
		if apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Equal(fields(t, err), []string{"StoreID"}) {
			t.Fatalf("店家%s：err=%v 欄位=%v，預期 invalid_input 指出 StoreID", name, err, fields(t, err))
		}
	}
	if !slices.Equal(d.asked, []int64{99, 12}) {
		t.Fatalf("店家查詢：%v", d.asked)
	}
}
