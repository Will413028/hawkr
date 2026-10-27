package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
)

type fakeShipmentResolver struct {
	statuses map[string]string
	err      error
	gotOrder []string
	calls    int
}

func (f *fakeShipmentResolver) BatchGetShipmentStatus(_ context.Context, orderNos []string) (map[string]string, error) {
	f.calls++
	f.gotOrder = orderNos
	if f.err != nil {
		return nil, f.err
	}
	return f.statuses, nil
}

func docRow(orderNo string) usecase.DocumentListItem {
	ref := orderNo
	return usecase.DocumentListItem{Document: domain.Document{
		SourceType: sourceTypeOrder, SourceRef: &ref,
	}}
}

// 🔴 本檔的核心一格：**「整批查不到」與「都還沒出貨」必須分得開。**
//
// 兩種情況下每一列的 shipment_status 都是空字串——差別只在 degraded。
// 少了這個布林，ADM 的 A12 頁面（它逐字寫著「空白會被讀成『還沒出貨』」）
// 在 shipping-service 抖動時會集體說謊，而 #3019 的症狀就換一個形式重演。
func TestResolveShipmentStatuses_DegradedIsDistinguishableFromNoShipment(t *testing.T) {
	log := zap.NewNop()

	t.Run("查得到、但這些訂單都沒有物流單", func(t *testing.T) {
		r := &fakeShipmentResolver{statuses: map[string]string{}}
		s := &InvoiceServer{shipmentStatuses: r, log: log}

		got, degraded := s.resolveShipmentStatuses(context.Background(), []string{"O1", "O2"})
		if degraded {
			t.Fatal("查得到就不是降級：degraded 應為 false")
		}
		if len(got) != 0 {
			t.Fatalf("沒有物流單就該是空 map，got=%#v", got)
		}
	})

	t.Run("整批查不到", func(t *testing.T) {
		r := &fakeShipmentResolver{err: errors.New("shipping down")}
		s := &InvoiceServer{shipmentStatuses: r, log: log}

		got, degraded := s.resolveShipmentStatuses(context.Background(), []string{"O1", "O2"})
		if !degraded {
			t.Fatal("🔴 查不到卻回 degraded=false —— 前端會把空字串讀成「還沒出貨」")
		}
		if len(got) != 0 {
			t.Fatalf("降級時不得回半套資料，got=%#v", got)
		}
	})
}

// resolver 沒接上（nil）不是降級：那是「這一欄從來就沒有來源」，
// 畫面退回修這件事之前的樣子，不該讓前端顯示「暫時取不到」。
func TestResolveShipmentStatuses_NilResolverIsNotDegraded(t *testing.T) {
	s := &InvoiceServer{shipmentStatuses: nil, log: zap.NewNop()}
	got, degraded := s.resolveShipmentStatuses(context.Background(), []string{"O1"})
	if degraded || len(got) != 0 {
		t.Fatalf("nil resolver 應回 (空 map, false)，got=%#v degraded=%v", got, degraded)
	}
}

// 一頁裡同一張訂單出現多列是常態（作廢重開＝SUPERSEDED ＋ 新票兩列，同一個 order_no）。
func TestResolveShipmentStatuses_DedupesAndDropsEmpty(t *testing.T) {
	r := &fakeShipmentResolver{statuses: map[string]string{"O1": "31"}}
	s := &InvoiceServer{shipmentStatuses: r, log: zap.NewNop()}

	if _, _ = s.resolveShipmentStatuses(context.Background(), []string{"O1", "O1", "", "O2"}); r.calls != 1 {
		t.Fatalf("應只呼叫一次，calls=%d", r.calls)
	}
	if len(r.gotOrder) != 2 || r.gotOrder[0] != "O1" || r.gotOrder[1] != "O2" {
		t.Fatalf("去重／濾空沒生效：%#v", r.gotOrder)
	}
}

// orderNosOf 只收 source_type=ORDER 的列：平台代製收據與 ADM 手動開票沒有訂單。
func TestOrderNosOf_OnlyOrderSourced(t *testing.T) {
	manual := usecase.DocumentListItem{Document: domain.Document{SourceType: "MANUAL"}}
	got := orderNosOf([]usecase.DocumentListItem{docRow("O1"), manual, docRow("O2")})
	if len(got) != 2 || got[0] != "O1" || got[1] != "O2" {
		t.Fatalf("非訂單來源的列不該進批次查詢：%#v", got)
	}
}

// toSellerDocumentItems：map 內的填值、不在 map 內的回空字串。
func TestToSellerDocumentItems_FillsShipmentStatus(t *testing.T) {
	// 用完整 fixture 的 server：toSummary 會走 query.FileAvailability，
	// 空的 &InvoiceServer{} 在那裡 nil-deref。
	s := newReadServer(t, nil).server
	s.shipmentStatuses = nil
	items := s.toSellerDocumentItems(
		[]usecase.DocumentListItem{docRow("O1"), docRow("O2")},
		map[string]string{"O1": "210"},
	)
	if len(items) != 2 {
		t.Fatalf("列數不對：%d", len(items))
	}
	if items[0].GetShipmentStatus() != "210" {
		t.Fatalf("🔴 有物流單卻沒填：%q —— 這正是 #3019 的原始症狀", items[0].GetShipmentStatus())
	}
	if items[1].GetShipmentStatus() != "" {
		t.Fatalf("沒有物流單應回空字串（誠實的『還沒出貨』），got=%q", items[1].GetShipmentStatus())
	}
}
