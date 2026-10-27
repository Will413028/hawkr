package eventwire

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

func ptr(v int64) *int64 { return &v }

// TestStoreIDOfReadsIssuerRefIDNotPK 是 P0-3 的單元層證據。
//
// 鑑別力來自 fixture：ID 與 IssuerRefID 是**不同的兩個值**，所以「填錯哪一個」
// 在這裡可觀測。兩者同值的 fixture 會讓這支測試證明一件現實中不存在的事。
func TestStoreIDOfReadsIssuerRefIDNotPK(t *testing.T) {
	issuer := domain.Issuer{ID: 7304857280000000001, IssuerType: "SELLER", IssuerRefID: 2685914682}
	got := mustMarshal(t, StoreIDOf(issuer))
	if got != `"2685914682"` {
		t.Fatalf("store_id = %s，want \"2685914682\"（issuer_ref_id）。"+
			"若它等於 %d 就是又填回了 invoice_issuers 的 PK。", got, issuer.ID)
	}
}

// TestStoreIDIsAlwaysAString：TSID 以 JSON number 進 generic-map consumer 會被解成
// float64 而截斷（memory reference_tsid_json_event_precision）。
func TestStoreIDIsAlwaysAString(t *testing.T) {
	got := mustMarshal(t, StoreIDOf(domain.Issuer{ID: 1, IssuerRefID: 9007199254740993}))
	if got != `"9007199254740993"` {
		t.Fatalf("store_id = %s，必須是帶引號的字串", got)
	}
}

// TestPlatformIssuerStoreIDIsZero：PLATFORM 開票人沒有賣家，"0" 是正確結果不是缺漏。
func TestPlatformIssuerStoreIDIsZero(t *testing.T) {
	if got := mustMarshal(t, StoreIDOf(domain.Issuer{ID: 5, IssuerType: "PLATFORM", IssuerRefID: 0})); got != `"0"` {
		t.Fatalf("PLATFORM store_id = %s，want \"0\"", got)
	}
}

// TestUnsetWireValuesRefuseToMarshal 是「漏填」的第二道線（第一道是 AST 的 R4）。
//
// 🔴 這裡驗的是**陰性**保證：漏填不得靜默變成空字串。空字串會讓 dispatcher 解不出
// 賣家 ⇒ notify_seller_account_id 恆 "0" ⇒ 通知被 skip，而那正是要防的靜默失效。
func TestUnsetWireValuesRefuseToMarshal(t *testing.T) {
	if _, err := json.Marshal(struct {
		S StoreID `json:"store_id"`
	}{}); !errors.Is(err, ErrUnsetStoreID) {
		t.Fatalf("零值 StoreID 的 marshal error = %v，want ErrUnsetStoreID", err)
	}
	if _, err := json.Marshal(struct {
		B BuyerID `json:"buyer_id"`
	}{}); !errors.Is(err, ErrUnsetBuyerID) {
		t.Fatalf("零值 BuyerID 的 marshal error = %v，want ErrUnsetBuyerID", err)
	}
	// 沒載入的 issuer／document／complaint 同樣不得靜默產出值。
	if _, err := json.Marshal(StoreIDOf(domain.Issuer{})); !errors.Is(err, ErrUnsetStoreID) {
		t.Fatalf("零值 issuer 應產出不可 marshal 的 StoreID，實得 err=%v", err)
	}
	if _, err := json.Marshal(BuyerIDOfDocument(domain.Document{})); !errors.Is(err, ErrUnsetBuyerID) {
		t.Fatalf("零值 document 應產出不可 marshal 的 BuyerID，實得 err=%v", err)
	}
	if _, err := json.Marshal(BuyerIDOfComplaint(domain.Complaint{})); !errors.Is(err, ErrUnsetBuyerID) {
		t.Fatalf("零值 complaint 應產出不可 marshal 的 BuyerID，實得 err=%v", err)
	}
}

// TestBuyerIDOfDocumentJudgesRecipientTypeFirst 是 P0-3 在 buyer_id 軸上的孿生缺陷。
//
// 🔴 四個方向逐一驗，**含三個陰性**：只驗「BUYER 會填」不會抓到原缺陷，因為原缺陷在
// BUYER 方向上的行為完全正確。保證的實質是「非 BUYER 不會填」。
func TestBuyerIDOfDocumentJudgesRecipientTypeFirst(t *testing.T) {
	const buyerKey = int64(2623900520)
	const storeKey = int64(2685914682) // recipient_type=SELLER 時 recipient_ref_id 是這個

	cases := []struct {
		name string
		doc  domain.Document
		want string
	}{
		{
			name: "方向一 BUYER：填買家業務鍵",
			doc:  domain.Document{ID: 1, RecipientType: codemeta.InvoiceRecipientTypeBuyer.Value, RecipientRefID: ptr(buyerKey)},
			want: `"2623900520"`,
		},
		{
			name: "方向二 SELLER：recipient_ref_id 是 store_id，不得裝進 buyer_id",
			doc:  domain.Document{ID: 2, RecipientType: codemeta.InvoiceRecipientTypeSeller.Value, RecipientRefID: ptr(storeKey)},
			want: `""`,
		},
		{
			name: "方向三 EXTERNAL（ref 為 NULL）",
			doc:  domain.Document{ID: 3, RecipientType: codemeta.InvoiceRecipientTypeExternal.Value},
			want: `""`,
		},
		{
			name: "方向四 EXTERNAL 但 ref 非空（髒資料）：型別仍不得放行",
			doc:  domain.Document{ID: 4, RecipientType: codemeta.InvoiceRecipientTypeExternal.Value, RecipientRefID: ptr(storeKey)},
			want: `""`,
		},
		{
			name: "BUYER 但 ref 為 NULL：沒有可填的值",
			doc:  domain.Document{ID: 5, RecipientType: codemeta.InvoiceRecipientTypeBuyer.Value},
			want: `""`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mustMarshal(t, BuyerIDOfDocument(c.doc)); got != c.want {
				t.Fatalf("buyer_id = %s，want %s。\n"+
					"填成 %d 的話，§6.3.9 模板的 target_user_field 會把通知送給一個不存在的買家"+
					"（或更糟，送給 buyer_id 恰好等於該 store_id 的另一個人）。",
					got, c.want, storeKey)
			}
		})
	}
}

// TestBuyerIDOfComplaintNeedsNoRecipientTypeCheck 說明為什麼申訴軸不需要判別。
func TestBuyerIDOfComplaintNeedsNoRecipientTypeCheck(t *testing.T) {
	got := mustMarshal(t, BuyerIDOfComplaint(domain.Complaint{ID: 9, ComplainantBuyerID: 2688637900}))
	if got != `"2688637900"` {
		t.Fatalf("buyer_id = %s，want \"2688637900\"", got)
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
