package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// #3054 的稽查：ADM 申訴詳情（端點 59）必須帶得出 store_id。
//
// 🔴 症狀：ADM **列表**（端點 58）從 SQL join 帶回 issuer_ref_id，**詳情**沒有 ⇒
// 管理員從列表點進去，A08 的「賣家：{storeId}」整段消失
// （前端是 `complaint?.storeId ? <span>賣家：…</span> : null`，沒有 fallback）。
// dev 實測：列表 storeId='2636840881'、同一件的詳情 storeId=”。
//
// 🔴 值必須是 invoice_issuers.issuer_ref_id，**不是** complaint.IssuerID（那是 PK）。
// §6 明訂兩者不可互換，而它們在這裡都是 bigint、填錯不會有任何東西報錯。
func TestGetComplaintForAdmin_CarriesStoreID(t *testing.T) {
	const (
		issuerPK int64 = 700001
		storeID  int64 = 2636840881
		cID      int64 = 7500965941417607168
	)
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	q := (&Query{
		issuers:     &fakeIssuerRepo{issuer: domain.Issuer{ID: issuerPK, IssuerRefID: storeID}},
		complaints:  &fakeComplaintRepo{stored: domain.Complaint{ID: cID, IssuerID: issuerPK, SlaDueAt: now}},
		audit:       &fakeAudit{},
		businessDay: &fakeBizDays{},
		documents:   &fakeDocRepo{},
	}).WithClock(func() time.Time { return now })

	v, err := q.GetComplaintForAdmin(context.Background(), 2613330734, cID)
	if err != nil {
		t.Fatalf("GetComplaintForAdmin: %v", err)
	}
	if v.StoreID != storeID {
		t.Fatalf("StoreID = %d，want %d（issuer_ref_id）—— A08 的「賣家：」讀這一欄", v.StoreID, storeID)
	}
	// 🔴 鑑別力：兩個 id 都是 bigint，取錯不會有任何東西報錯。
	if v.StoreID == issuerPK {
		t.Fatal("取到 IssuerID（PK）了 —— §6 明訂 store_id 與 issuer PK 不可互換")
	}
}

// 陰性對照：買家面與賣家面刻意不帶 store_id（畫面不顯示它，不為了填滿契約多打一次
// issuer 查詢）。這一支同時是上一支的鑑別力證明 —— 沒有它，一個「無條件填 storeID」
// 的實作也會讓上面通過。
func TestComplaintView_BuyerAndSellerPathsLeaveStoreIDZero(t *testing.T) {
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	q := (&Query{
		issuers:     &fakeIssuerRepo{issuer: domain.Issuer{ID: 700001, IssuerRefID: 2636840881}},
		complaints:  &fakeComplaintRepo{stored: domain.Complaint{ID: 1, IssuerID: 700001, SlaDueAt: now}},
		audit:       &fakeAudit{},
		businessDay: &fakeBizDays{},
		documents:   &fakeDocRepo{},
	}).WithClock(func() time.Time { return now })

	v, err := q.complaintView(context.Background(), domain.Complaint{ID: 1, IssuerID: 700001, SlaDueAt: now})
	if err != nil {
		t.Fatalf("complaintView: %v", err)
	}
	if v.StoreID != 0 {
		t.Fatalf("共用的 complaintView 不該自己填 StoreID（實得 %d）：那會讓買家面與賣家面"+
			"每次都多打一次 issuer 查詢，而它們的畫面根本不顯示它", v.StoreID)
	}
}
