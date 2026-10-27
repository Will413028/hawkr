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

// ---------------------------------------------------------------------------
// 端點 8：已取消的訂單不得再說「付款完成後會立即開立發票」（#2904）
//
// 🔴 這個 bug 的形狀是本檔存在的理由：**沒有任何既有測試會紅**。前端忠實渲染後端
// 給的 state／stateNotice，後端也「成功」回了 200，DB 沒有任何異常列。它是靠人把
// 訂單詳情頁的截圖看完才發現的——上方是紅色的「已取消」時間軸，下方的發票區在說
// 「付款完成後會立即開立發票」。
//
// 所以這裡斷言的是**買家真的會讀到的那兩個字串**，不是「有沒有呼叫到 repository」。
// ---------------------------------------------------------------------------

const (
	ocOrderNo   = "O320270909"
	ocBuyerID   = int64(2623900520)
	ocOtherByer = int64(2688637900)
)

// ocNoDocuments 是「這張訂單沒有任何 invoice_documents 列」——付款前與取消後**完全
// 一樣**的那個資料狀態。#2904 的本體就是這兩種情況被回同一個答案。
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
	// 🔴 陰性斷言：那句永遠不會成真的話**不得**出現在回應的任何一格。
	// 只驗 state 等於新值不夠——把 notice map 的那一格填錯（或忘了填，於是查表落空
	// 回空字串再被別人補上舊句）都還是會讓買家讀到它。
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

	// 別人的訂單：回應必須與「這張訂單根本不存在」完全一致。
	other := ocGet(t, s, ocOtherByer)
	if other.GetState() != "AWAITING_PAYMENT" {
		t.Errorf("越權查詢得到 state=%q —— 它洩漏了「這張訂單存在且已取消」", other.GetState())
	}

	// 位元組層級比對：與一張完全不存在的訂單的回應逐欄相同。
	fresh := ocServer(t, qCancelled{})
	baseline := ocGet(t, fresh, ocOtherByer)
	if other.GetState() != baseline.GetState() || other.GetStateNotice() != baseline.GetStateNotice() {
		t.Errorf("越權回應與「不存在」不一致：got (%q,%q) want (%q,%q)",
			other.GetState(), other.GetStateNotice(), baseline.GetState(), baseline.GetStateNotice())
	}
}
