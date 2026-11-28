package grpc

import (
	"context"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
)

const (
	ocOrderNo   = "O320270909"
	ocBuyerID   = int64(2623900520)
	ocOtherByer = int64(2688637900)
)

type ocNoDocuments struct{ domain.DocumentRepository }

func (ocNoDocuments) ListOrderDocumentsForBuyer(context.Context, string, int64) ([]domain.Document, error) {
	return nil, nil
}

func ocServer(t *testing.T, cancelled qCancelled) *InvoiceServer {
	t.Helper()
	q := usecase.NewQuery(
		rIssuers{}, qProfiles{}, qAccounts{}, qPrefs{},
		ocNoDocuments{}, qOperations{}, qComplaints{}, &qAudit{},
		qAdjustments{}, qCorrections{}, qOverview{},
		qBizDays{synced: syncedNow(time.Now().UTC())},
		qCutovers{}, qUnlocks{},
		cancelled,
		usecase.IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute},
		zap.NewNop(),
	)
	return NewInvoiceServer(nil, q, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
}

func ocGet(t *testing.T, s *InvoiceServer, buyerID int64) *pb.OrderInvoiceData {
	t.Helper()
	resp, err := s.GetOrderInvoice(context.Background(), &pb.GetOrderInvoiceRequest{
		BuyerId: buyerID, OrderNo: ocOrderNo,
	})
	if err != nil {
		t.Fatalf("不應回 gRPC error：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d，端點 8 對「沒有文件」的正確答案不是業務錯誤", resp.GetCode())
	}
	return resp.GetData()
}

// TestGetOrderInvoice_CancelledOrderDoesNotPromiseAnInvoice 是 #2904 的迴歸測試。
func TestGetOrderInvoice_CancelledOrderDoesNotPromiseAnInvoice(t *testing.T) {
	s := ocServer(t, qCancelled{rows: map[qCancelledKey]bool{
		{orderNo: ocOrderNo, buyerID: ocBuyerID}: true,
	}})
	data := ocGet(t, s, ocBuyerID)

	if data.GetState() != "ORDER_CANCELLED" {
		t.Errorf("state = %q, want ORDER_CANCELLED", data.GetState())
	}
	if got := data.GetStateNotice(); got != "訂單已取消，不會開立發票" {
		t.Errorf("stateNotice = %q, want 「訂單已取消，不會開立發票」", got)
	}
	if data.GetStateNotice() == "付款完成後會立即開立發票" {
		t.Error("🔴 已取消的訂單仍在承諾開立發票 —— 那句話對它永遠不會成真")
	}
	if len(data.GetDocuments()) != 0 {
		t.Errorf("documents 應為空，得 %d 筆", len(data.GetDocuments()))
	}
}

// TestGetOrderInvoice_UncancelledOrderStillAwaitsPayment 是控制組。
//
// 少了它，「把 len(docs)==0 一律改成 ORDER_CANCELLED」也會讓上面那支變綠——
// 而那會讓每一張還沒付款的訂單都被告知不會開發票。
func TestGetOrderInvoice_UncancelledOrderStillAwaitsPayment(t *testing.T) {
	s := ocServer(t, qCancelled{})
	data := ocGet(t, s, ocBuyerID)

	if data.GetState() != "AWAITING_PAYMENT" {
		t.Errorf("state = %q, want AWAITING_PAYMENT（還沒付款的訂單不該被告知不會開發票）", data.GetState())
	}
	if got := data.GetStateNotice(); got != "付款完成後會立即開立發票" {
		t.Errorf("stateNotice = %q, want 「付款完成後會立即開立發票」", got)
	}
}

// TestGetOrderInvoice_CancellationIsNotAnExistenceOracle 是 Tier A 的陰性斷言（§8.1.4／E21）。
//
// 🔴 新增的查詢若只用 order_no 當鍵，任何登入買家拿別人的 order_no 來問都會得到
// 「這張訂單存在且已取消」。訂單號是可枚舉的（O ＋ 9 位數字）⇒ 憑空多出一個訂單
// 存在性 oracle，而畫面上完全正常、log 乾淨、沒有任何錯誤。
//
// 同一支端點的另一半（ListOrderDocumentsForBuyer）本來就雙欄過濾；這一半不跟上
// 就是把它繞開。
func TestGetOrderInvoice_CancellationIsNotAnExistenceOracle(t *testing.T) {
	s := ocServer(t, qCancelled{rows: map[qCancelledKey]bool{
		{orderNo: ocOrderNo, buyerID: ocBuyerID}: true,
	}})

	other := ocGet(t, s, ocOtherByer)
	if other.GetState() != "AWAITING_PAYMENT" {
		t.Errorf("越權查詢得到 state=%q —— 它洩漏了「這張訂單存在且已取消」", other.GetState())
	}

	fresh := ocServer(t, qCancelled{})
	baseline := ocGet(t, fresh, ocOtherByer)
	if other.GetState() != baseline.GetState() || other.GetStateNotice() != baseline.GetStateNotice() {
		t.Errorf("越權回應與「不存在」不一致：got (%q,%q) want (%q,%q)",
			other.GetState(), other.GetStateNotice(), baseline.GetState(), baseline.GetStateNotice())
	}
}
