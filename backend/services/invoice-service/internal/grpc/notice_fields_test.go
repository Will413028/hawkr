package grpc

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

// #3031：兩個 notice 欄位在此之前恆為空字串，而兩個理由都寫在程式碼註解裡、
// 兩個都不成立：
//
//	shipping_fee_notice  「設計書全文沒有指定這句文案」→ 定版字串在 58A §B04
//	status_notice        「這一欄沒有被規定的內容」    → 58A §B06 說它是 toast 的 body
//
// 空字串的後果分別是：B04 底部那條「恆顯示」的 info banner 沒有內容、
// B06 送出成功的 toast 沒有內文。
func TestCorrectionStatusNotices_CoversEveryStatus(t *testing.T) {
	for _, s := range codemeta.AllInvoiceCorrectionStatuss {
		if usecase.CorrectionStatusNotices[s.Value] == "" {
			t.Errorf("狀態 %s（%s）沒有文案", s.Value, s.Label)
		}
	}
	if len(usecase.CorrectionStatusNotices) != len(codemeta.AllInvoiceCorrectionStatuss) {
		t.Errorf("文案表 %d 筆、狀態 %d 個 —— 多出來的是已刪除的狀態值",
			len(usecase.CorrectionStatusNotices), len(codemeta.AllInvoiceCorrectionStatuss))
	}

	got := usecase.CorrectionStatusNotices[codemeta.InvoiceCorrectionStatusReissueFailed.Value]
	if !contains(got, "已作廢") {
		t.Errorf("REISSUE_FAILED 的文案沒有說出原發票已作廢：%q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestToCorrectionData_FillsStatusNotice(t *testing.T) {
	for _, s := range codemeta.AllInvoiceCorrectionStatuss {
		got := toCorrectionData(domain.CorrectionRequest{Status: s.Value})
		if got.GetStatusNotice() == "" {
			t.Errorf("status=%s 的 statusNotice 是空的 —— B06 的 toast 會沒有內文", s.Value)
		}
		if got.GetStatusNotice() != usecase.CorrectionStatusNotices[s.Value] {
			t.Errorf("status=%s 沒有走共用文案表", s.Value)
		}
	}
}

// 🔴 逐字比對：58A 兩處都寫「逐字顯示後端 shippingFeeNotice」，而 B05 那頁註明
// 「本頁沒有該欄位時用同一句常數」⇒ 前端有一份硬編碼的同句話
// （website features/invoices/i18n.ts）。兩邊漂移的話，同一個買家在兩個頁面
// 會看到兩種說法，而畫面上不會有任何錯誤。
func TestShippingFeeNotice_IsTheVerbatimSpecString(t *testing.T) {
	const spec = "物流費用已包含在本張發票的銷售額中，另列的物流明細不是另一張發票。"
	if usecase.ShippingFeeNotice != spec {
		t.Fatalf("與 58A §B04 的定版字串不符：\n got=%q\nwant=%q", usecase.ShippingFeeNotice, spec)
	}
}

// #3031：兩個「請求編號」欄位。
//
// 🔴 兩者的來源都是 correlation_id，而表上沒有名為 request_id 的欄位——
// 恆為空字串的後果是 A06 第 4 層每一列都追不回我方的 log，S05 的賣家也給不出
// 可供客服追查的編號。
func TestProviderExchange_FillsRequestIdFromCorrelationId(t *testing.T) {
	cid := "corr-abc-123"
	got := toProviderExchange(domain.ProviderAuditLogEntry{
		ID: 1, Endpoint: "/json/f0401", CorrelationID: &cid,
	})
	if got.GetRequestId() != cid {
		t.Fatalf("request_id 沒填或沒取 correlation_id：%q", got.GetRequestId())
	}
	none := toProviderExchange(domain.ProviderAuditLogEntry{ID: 2, Endpoint: "/json/f0401"})
	if none.GetRequestId() != "" {
		t.Fatalf("沒有 correlation_id 時應為空字串，實得 %q", none.GetRequestId())
	}
}

// #3031：申訴卡的「買家要的是：發票／收據」（58A §S09 那一行）。
func TestToComplaintInfo_FillsExpectedDocKind(t *testing.T) {
	got := toComplaintInfo(usecase.ComplaintView{
		Complaint:       domain.Complaint{ID: 1},
		ExpectedDocKind: "RECEIPT",
	})
	if got.GetExpectedDocKind() != "RECEIPT" {
		t.Fatalf("expected_doc_kind 沒填：%q —— S09 卡上那一行會是空的", got.GetExpectedDocKind())
	}
	none := toComplaintInfo(usecase.ComplaintView{Complaint: domain.Complaint{ID: 2}})
	if none.GetExpectedDocKind() != "" {
		t.Fatalf("沒有來源時應為空字串，實得 %q", none.GetExpectedDocKind())
	}
}

// #3054：案件編號。恆為空字串時 B07 的整個「案件編號 ＋ 複製鍵」區塊不渲染
// （`complaint.caseNo ? ... : null`，沒有 fallback）⇒ 買家向客服描述自己的案件時
// 講不出任何識別碼。
//
// 🔴 值的來源不是新欄位而是本列的 TSID：58 §5.1.12 的 DDL 逐字寫著
// 「"id" bigint NOT NULL, -- TSID（對外＝案件編號）」。
func TestToComplaintInfo_FillsCaseNoFromTSID(t *testing.T) {
	const id int64 = 7500965941417607168
	got := toComplaintInfo(usecase.ComplaintView{Complaint: domain.Complaint{ID: id}})
	if got.GetCaseNo() != "7500965941417607168" {
		t.Fatalf("case_no = %q，want 該列的 TSID —— 空字串會讓 B07 整個編號區塊消失",
			got.GetCaseNo())
	}
	if got.GetCaseNo() != got.GetComplaintId() {
		t.Fatalf("case_no(%q) ≠ complaint_id(%q)：58 §5.1.12 定義兩者同源",
			got.GetCaseNo(), got.GetComplaintId())
	}
	other := toComplaintInfo(usecase.ComplaintView{Complaint: domain.Complaint{ID: id + 1}})
	if other.GetCaseNo() == got.GetCaseNo() {
		t.Fatal("兩件不同的申訴拿到同一個案件編號 ⇒ 它是常數，對客服指稱案件沒有用")
	}
}

// #3054 的稽查：A08 詳情的「賣家：{storeId}」。
//
// 🔴 這一欄原本在 toComplaintInfo 裡是常數空字串，而 ADM **列表**（端點 58）有補、
// **詳情**（端點 59）沒補 ⇒ 管理員從列表點進去，賣家那一段整個消失
// （前端是 `complaint?.storeId ? <span>賣家：…</span> : null`，沒有 fallback）。
//
// 判準與 caseNo 那一支相同：mapper 給的是空字串**是對的**（買賣家面不顯示它，
// 不該為了填滿契約多打一次 issuer 查詢），錯的是 ADM 詳情沒有覆寫。
func TestToComplaintInfo_StoreIdIsCallerSupplied(t *testing.T) {
	got := toComplaintInfo(usecase.ComplaintView{Complaint: domain.Complaint{ID: 7, IssuerID: 900}})
	if got.GetStoreId() != "" {
		t.Fatalf("mapper 不該自己填 store_id（它拿不到 issuer_ref_id）：%q", got.GetStoreId())
	}
	got.StoreId = fmtID(2636840881)
	if got.GetStoreId() != "2636840881" {
		t.Fatalf("覆寫後 store_id = %q", got.GetStoreId())
	}
	if got.GetStoreId() == fmtID(900) {
		t.Fatal("store_id 取到 IssuerID（PK）了 —— 那是 §6 明令不可互換的兩個鍵")
	}
}
