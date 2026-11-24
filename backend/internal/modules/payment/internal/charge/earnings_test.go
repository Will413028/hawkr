package charge

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 每個來源狀態都收斂成賣家的三種狀態之一；來源新增狀態而這裡沒對應時回錯誤，不猜。
func TestEarningsStatusCoversEverySourceStatus(t *testing.T) {
	type c struct {
		kind model.EarningsType
		raw  string
		want model.EarningsStatus
	}
	var cases []c
	for _, s := range model.TransferStatusValues() {
		want := map[model.TransferStatus]model.EarningsStatus{
			model.TransferSucceeded: model.EarningsCompleted, model.TransferFailed: model.EarningsFailed,
			model.TransferPending: model.EarningsPending, model.TransferAnomaly: model.EarningsPending,
		}[s]
		cases = append(cases, c{model.EarningsTransfer, string(s), want})
	}
	cases = append(cases,
		c{model.EarningsReversal, "REVERSED", model.EarningsCompleted},
		c{model.EarningsReversal, "REVERSING", model.EarningsPending},
		c{model.EarningsReversal, "ANOMALY", model.EarningsPending},
		c{model.EarningsTopUp, "DONE", model.EarningsCompleted},
		c{model.EarningsTopUp, "WAITING", model.EarningsPending},
		c{model.EarningsTopUp, "SENDING", model.EarningsPending},
		c{model.EarningsTopUp, "ANOMALY", model.EarningsPending},
	)
	for _, tc := range cases {
		got, err := earningsStatus(tc.kind, tc.raw)
		if err != nil || got != tc.want {
			t.Errorf("%s/%s = %q, %v，預期 %q", tc.kind, tc.raw, got, err, tc.want)
		}
	}
	for _, bad := range []c{{model.EarningsReversal, "RELEASED", ""}, {model.EarningsReversal, "NONE", ""}, {model.EarningsReversal, "WAITING", ""}, {model.EarningsTopUp, "NONE", ""}, {model.EarningsTopUp, "RELEASED", ""}, {"OTHER", "DONE", ""}} {
		if got, err := earningsStatus(bad.kind, bad.raw); err == nil {
			t.Errorf("%s/%s 應該回錯誤，得到 %q", bad.kind, bad.raw, got)
		}
	}
}

// 篩選不合在碰資料庫之前就擋下。
func TestListEarningsRejectsBadInputBeforeReadingAnything(t *testing.T) {
	s := &Service{}
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	earlier := now.Add(-time.Hour)
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	for name, c := range map[string]struct {
		storeID int64
		f       model.EarningsFilter
		size    int32
	}{
		"沒有店家":    {0, model.EarningsFilter{}, 10},
		"起點等於終點":  {1, model.EarningsFilter{From: &now, To: &now}, 10},
		"起點晚於終點":  {1, model.EarningsFilter{From: &now, To: &earlier}, 10},
		"訂單編號太長":  {1, model.EarningsFilter{OrderNo: string(long)}, 10},
		"負數的每頁筆數": {1, model.EarningsFilter{}, -1},
	} {
		if _, err := s.ListEarnings(context.Background(), c.storeID, c.f, nil, c.size); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	if _, err := s.OrderEarnings(context.Background(), 1, "  "); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("空的訂單編號：err=%v", err)
	}
}
