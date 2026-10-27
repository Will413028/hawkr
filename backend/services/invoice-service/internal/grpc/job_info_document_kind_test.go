package grpc

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
)

// toJobInfo 必須把 document_kind 帶下去（#3066 裁決 B 的 ADM 面收尾）。
//
// 🔴 **這一欄漏掉是完全無聲的**：不會編譯錯、不會讓任何既有測試轉紅。症狀是
// 端點 51（A06 詳情）與端點 48 的 relatedJobs 兩個入口的 documentKind 恆為空字串，
// 而列表面（端點 50／33）走 toJobInfoView 的 SQL 版本**有值** ⇒ 同一份契約在兩個
// 入口說不同的話。dev 實測（2026-09-05）三種 operation type 皆為 ""。
//
// 連帶壞掉的是 shipmentUnaffectedNotice：noticeForKind 對未知 kind 退回發票措辭，
// 於是免用統一發票的賣家在 A06 讀到「發票開立與出貨進度互不影響」——一件他這輩子
// 不會遇到的事。
func TestToJobInfo_CarriesDocumentKind(t *testing.T) {
	s := &InvoiceServer{query: &usecase.Query{}, log: zap.NewNop()}
	op := domain.Operation{
		ID: 7001, DocumentID: 8001,
		OperationType: codemeta.InvoiceOperationTypeIssue.Value,
		Status:        codemeta.InvoiceOperationStatusSucceeded.Value,
	}
	issuer := domain.Issuer{ID: 900, IssuerRefID: 2642010415}

	for _, kind := range []string{
		codemeta.InvoiceDocumentKindReceipt.Value,
		codemeta.InvoiceDocumentKindB2c.Value,
		codemeta.InvoiceDocumentKindB2b.Value,
	} {
		t.Run(kind, func(t *testing.T) {
			doc := domain.Document{
				ID: 8001, DocumentKind: kind,
				Status: codemeta.InvoiceDocumentStatusIssued.Value,
			}
			got := s.toJobInfo(op, doc, issuer)
			if got.GetDocumentKind() != kind {
				t.Fatalf("document_kind 應為 %q，實得 %q —— 前端靠它分辨並列的發票／收據兩個區塊，"+
					"且不得從 operation_type 推導（那個 enum 沒有 RECEIPT）", kind, got.GetDocumentKind())
			}
			// 文案跟著 kind 走：收據賣家不該讀到「發票開立…」。
			wantNotice := shipmentUnaffectedNotice
			if kind == codemeta.InvoiceDocumentKindReceipt.Value {
				wantNotice = shipmentUnaffectedNoticeReceipt
			}
			if got.GetShipmentUnaffectedNotice() != wantNotice {
				t.Fatalf("shipment_unaffected_notice 應為 %q，實得 %q",
					wantNotice, got.GetShipmentUnaffectedNotice())
			}
		})
	}
}
