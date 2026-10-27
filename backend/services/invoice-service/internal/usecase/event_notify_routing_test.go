package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/notifyrouting"
)

// ---------------------------------------------------------------------------
// 行為層第二道線：**每一個**出向事件的 store_id 都解得回賣家的通知目標
//
// 🔴 為什麼需要這一層，而型別 ＋ AST 不夠：前兩道守的是「值從哪裡來」，守不了
// 「那個值到了下游解不解得開」。P0-3 的原始形狀就是**來源看起來完全合理**
// （invoice_issuers.id，一個真實存在的欄位），只是下游解不出賣家。
//
// 🔴 本檔刻意**不寫任何手工 payload fixture**：payload 一律由**真正的生產者函式**
// 產出，解析一律走 **internal/notifyrouting 的真正那一份**（不是抄一份）。
// 兩端都由同一份想像產生、自己跟自己對必然全綠——那正是 2026-08-30 四個 P0 的
// 共同結構性成因（見 .claude/skills/mq-wire-contract）。
//
// 🔴 fixture 的鑑別力來自「每一個候選值都不同」：
//
//	issuer.ID          7304857280000000001  ← 填這個就是 P0-3（TSID，解不出賣家）
//	issuer.IssuerRefID 2685914682           ← 唯一正確的 store_id
//	doc.ID             7304857280000000002
//	doc.IssuerID       7304857280000000001
//	buyerRefID         2623900520           ← recipient_type=BUYER 時的 buyer_id
//	sellerRefID        2632049831           ← recipient_type=SELLER 時 recipient_ref_id
//	                                          裝的是**另一個賣家的 store_id**
//	complainantBuyerID 2688637900
//
// 任何兩個值相同的 fixture 都會讓這些測試證明一件現實中不存在的事。
// ---------------------------------------------------------------------------

const (
	fxIssuerPK      int64 = 7304857280000000001
	fxStoreID       int64 = 2685914682
	fxDocID         int64 = 7304857280000000002
	fxBuyerID       int64 = 2623900520
	fxOtherStoreID  int64 = 2632049831
	fxComplainantID int64 = 2688637900
	fxSellerAccount int64 = 4242424242
)

// resolveOnlyStoreID 只認得**真正的 store_id**，其餘一律回錯。
//
// 🔴 鑑別力全在「其餘一律回錯」：一個接受任何 int64 的 resolver 會讓 publisher 填
// invoice_issuers 的 PK 也照樣通過 —— 而那正是原缺陷的形狀。
type resolveOnlyStoreID struct {
	asked []int64
}

func (r *resolveOnlyStoreID) ResolveSellerAccountID(_ context.Context, storeID int64) (int64, error) {
	r.asked = append(r.asked, storeID)
	if storeID != fxStoreID {
		return 0, errors.New("no seller for that store_id")
	}
	return fxSellerAccount, nil
}

func fxIssuer() domain.Issuer {
	return domain.Issuer{
		ID: fxIssuerPK, IssuerType: "SELLER", IssuerRefID: fxStoreID,
		DisplayName: "端到端測試賣場", Status: "active",
	}
}

// fxDoc 造一份 document。recipientType 與 recipientRef 由呼叫端指定，因為
// buyer_id 的正確性**只在非 BUYER 方向上才觀測得到**。
func fxDoc(recipientType string, recipientRef *int64) domain.Document {
	order := "O000000001"
	return domain.Document{
		ID: fxDocID, IssuerID: fxIssuerPK, SourceRef: &order,
		RecipientType: recipientType, RecipientRefID: recipientRef,
		DocumentKind:   codemeta.InvoiceDocumentKindB2c.Value,
		Status:         codemeta.InvoiceDocumentStatusIssued.Value,
		RecipientTaxID: domain.B2CBuyerIdentifier, TotalAmount: 100,
	}
}

func fxComplaint() domain.Complaint {
	order := "O000000001"
	return domain.Complaint{
		ID: 9001, DocumentID: fxDocID, IssuerID: fxIssuerPK, OrderNo: &order,
		ComplainantBuyerID: fxComplainantID, ReasonCode: "NOT_RECEIVED",
		Status: codemeta.InvoiceComplaintStatusOpen.Value, SlaDueAt: time.Now().UTC(),
	}
}

func ptrInt64(v int64) *int64    { return &v }
func ptrString(v string) *string { return &v }

// producer 是一個出向事件生產點。
//
// Type 是它產出的 payload 型別名，而且**不是標籤而是斷言**：
// TestEveryScannedPayloadTypeHasABehaviouralProducer 會拿 AST 掃到的型別清單與這裡的
// Type 雙向對拍，並用該型別的 json tag 集合驗證實際 marshal 出來的 key ——
// Type 填錯會當場紅，不會靜默變成一個好聽的字串。
type producer struct {
	Name    string
	Type    string
	Payload []byte
}

// eventProducers 是**全部**出向事件的生產點。
//
// 🔴 這份清單曾經是**硬寫的七項**，而它的下界檢查也是硬寫的 `len(producers) < 7`。
// 那組合有一個安靜的缺口：新增第 7 個事件型別的人**不會**想到要回來加一項，
// 於是那個事件完全沒有行為層覆蓋——而行為層正是「型別 ＋ AST 都放行的繞道」
// 唯一擋得住的那道線。現在下界由 checkEventPayloads 掃出來的型別清單推導
// （TestEveryScannedPayloadTypeHasABehaviouralProducer），新事件不可能沒有生產點。
//
// 這份清單與 event_store_id_contract_test.go 的 knownEventPayloadTypes 互相釘住：
// 那邊的 AST gate 保證「型別都存在且都追得回 outbox」，這邊保證「都真的解得回賣家」。
func eventProducers(t *testing.T, doc domain.Document) []producer {
	t.Helper()
	now := time.Now().UTC()
	issuer := fxIssuer()
	c := fxComplaint()

	iss := &Issuance{}
	must := func(b json.RawMessage, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatalf("生產者回錯：%v", err)
		}
		return b
	}
	fromOutbox := func(name string, ob *fakeOutbox, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatalf("%s 生產者回錯：%v", name, err)
		}
		if len(ob.inserted) != 1 {
			t.Fatalf("%s 落了 %d 列 outbox，want 1", name, len(ob.inserted))
		}
		return ob.inserted[0].Payload
	}

	ctx := context.Background()

	adminOutbox := &fakeOutbox{}
	adminJob := &AdminJob{issuers: &fakeIssuerRepo{issuer: issuer}, outbox: adminOutbox}

	taxOutbox := &fakeOutbox{}
	taxUC := &TaxProfile{outbox: taxOutbox}

	voidOutbox := &fakeOutbox{}
	reviewOutbox := &fakeOutbox{}
	resendOutbox := &fakeOutbox{}
	resendUC := &NotificationResend{issuers: &fakeIssuerRepo{issuer: issuer}, outbox: resendOutbox}

	return []producer{
		{"P1/P6 issuedEvent（開立）", "issuedEvent", must(iss.issuedEventPayload(
			issuer, doc, &amego.IssueResult{InvoiceNumber: "AB12345678"}, nil, now))},
		{"P2 issueFailedEvent", "issueFailedEvent", must(iss.failedEventPayload(
			issuer, doc, domain.Operation{ID: 5001, AttemptCount: 3},
			failureClassPermanent, codemeta.InvoiceManualReasonSellerTrackInsufficient.Value,
			"3040122", now))},
		{"P8 complaintCreatedEvent", "complaintCreatedEvent", must(complaintCreatedPayload(issuer, c, doc, now))},
		{"P9 complaintRespondedEvent", "complaintRespondedEvent", must(sellerComplaintRespondedPayload(
			issuer, c, codemeta.InvoiceComplaintActionResent.Value,
			codemeta.InvoiceComplaintStatusResolved.Value, now))},
		{"P12 resolvedExternallyEvent", "resolvedExternallyEvent", fromOutbox("P12", adminOutbox,
			adminJob.emitResolvedExternally(ctx, doc, externalIssuance{
				InvoiceNo: "XY98765432", InvoiceDate: now, TotalAmount: 100, Now: now,
			}))},
		{"P11 taxStatusChangedEvent", "taxStatusChangedEvent", fromOutbox("P11", taxOutbox,
			taxUC.emitStatusChanged(ctx, issuer,
				domain.SellerTaxProfile{ID: 6001, IssuerID: fxIssuerPK, ReviewStatus: "SUBMITTED"},
				codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, now))},
		{"P1/P6 issuedEvent（重寄）", "issuedEvent", fromOutbox("重寄", resendOutbox,
			resendUC.emitResend(ctx, doc, 1, now))},
		// P5：原票號取自**原 document**、新票號取自 provider 回應，兩者填反的症狀是
		// 「原發票 X 已作廢並重新開立，新發票號碼 X」——同一個號碼且不會有東西報錯，
		// 所以這裡刻意讓兩個號碼取不同的值（#3055）。
		// P4：折讓開立。冪等鍵用 **adjustment id** 而不是 document id ——
		// 同一張發票可以分多次折讓，共用 document id 會讓第二張折讓單的通知被去重掉。
		{"P4 allowanceIssuedEvent", "allowanceIssuedEvent", must((&Allowance{}).allowanceIssuedPayload(
			issuer, doc,
			domain.Adjustment{
				ID: 6001, DocumentID: doc.ID,
				AllowanceNumber: ptrString("AL00000001"),
				GrossAmount:     ptrInt64(315),
				NetAmountExTax:  ptrInt64(300),
				TaxAmount:       ptrInt64(15),
			}, now))},
		// P10：申訴逾期。它與 P8（created）、P9（responded）是同一件案子的三個 phase，
		// 冪等鍵靠 phase 分開 —— 三者共用一個鍵的話 notification 端會去重成一則。
		{"P10 complaintOverdueEvent", "complaintOverdueEvent", must(complaintOverduePayload(issuer, c, now))},
		// P3：作廢可以單獨發生（退款折讓路徑不重開）⇒ 它與 P5 是兩件事，各自要有生產點。
		{"P3 voidedEvent", "voidedEvent", fromOutbox("P3", voidOutbox,
			(&Voiding{outbox: voidOutbox, issuers: &fakeIssuerRepo{issuer: issuer}}).
				emitVoided(ctx, doc, now))},
		// P7：fail-closed 落點的賣家通知。兩條路（建單、收據）共用同一個 emitter。
		{"P7 reviewRequiredEvent", "reviewRequiredEvent", fromOutbox("P7", reviewOutbox,
			(&Issuance{outbox: reviewOutbox}).emitReviewRequired(
				ctx, issuer, doc, codemeta.InvoiceManualReasonReceiptAuthMissing.Value, now))},
		{"P5 reissuedEvent", "reissuedEvent", must(iss.reissuedEventPayload(
			issuer, doc,
			domain.Document{ID: fxDocID + 1, ProviderInvoiceNo: ptrString("AA10000437")},
			&amego.IssueResult{InvoiceNumber: "AA10001506"}, now))},
	}
}

// TestEveryOutgoingEventResolvesToTheSellerNotifyTarget 是六個事件型別各自的
// 行為層第二道線（原本只有 issuedEvent 有）。
func TestEveryOutgoingEventResolvesToTheSellerNotifyTarget(t *testing.T) {
	doc := fxDoc(codemeta.InvoiceRecipientTypeBuyer.Value, ptrInt64(fxBuyerID))
	producers := eventProducers(t, doc)
	// 🔴 下界由 AST 掃出來的型別清單推導，不再是硬寫的數字（見
	// TestEveryScannedPayloadTypeHasABehaviouralProducer）。硬寫 `< 7` 的問題是
	// 「第 7 個事件型別」不會讓它變紅——它本來就是 7。
	assertProducersCoverEveryPayloadType(t, producers)

	for _, p := range producers {
		t.Run(p.Name, func(t *testing.T) {
			r := &resolveOnlyStoreID{}
			out, err := notifyrouting.Apply(context.Background(), p.Payload, r, nil)
			if err != nil {
				t.Fatalf("notifyrouting.Apply: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			// ① store_id 必須是 issuer_ref_id，不是 invoice_issuers 的 PK
			if got["store_id"] != "2685914682" {
				t.Fatalf("store_id = %#v，want \"2685914682\"（invoice_issuers.issuer_ref_id）。\n"+
					"若它等於 %d，就是又填回了 invoice_issuers 的 PK。",
					got["store_id"], fxIssuerPK)
			}
			// ② dispatcher 真的拿它去問了（而不是根本沒有這個鍵）
			if len(r.asked) != 1 || r.asked[0] != fxStoreID {
				t.Fatalf("dispatcher 拿去解析的值 = %v，want [%d]。\n"+
					"空陣列代表 payload 根本沒有 store_id 鍵 —— 賣家向通知 100%% 消失。",
					r.asked, fxStoreID)
			}
			// ③ 解出來的 account_id 不是 "0"（"0" ＝ 下游會 skip 該則通知）
			if got["notify_seller_account_id"] != "4242424242" {
				t.Fatalf("notify_seller_account_id = %#v，want \"4242424242\"。"+
					"它是 \"0\" 就代表賣家向通知會被下游 skip。",
					got["notify_seller_account_id"])
			}
		})
	}
}

// TestBuyerIDNeverCarriesAStoreIDAcrossEveryProducer 是 P0-3 在 buyer_id 軸上的
// 孿生缺陷的行為層證據。
//
// 🔴 這一支的價值全在**陰性**：只驗 BUYER 方向不會抓到任何東西，因為原缺陷在
// BUYER 方向上的行為完全正確。要驗的是「recipient_type=SELLER 時，那個
// recipient_ref_id（＝另一個賣家的 store_id）**不會**出現在 buyer_id 上」。
func TestBuyerIDNeverCarriesAStoreIDAcrossEveryProducer(t *testing.T) {
	cases := []struct {
		name          string
		recipientType string
		recipientRef  *int64
		// wantDocBuyer 是**文件衍生**事件（P1/P2/P12/重寄）該有的 buyer_id。
		wantDocBuyer string
	}{
		{"方向一 BUYER", codemeta.InvoiceRecipientTypeBuyer.Value, ptrInt64(fxBuyerID), "2623900520"},
		{"方向二 SELLER（recipient_ref_id 是另一個賣家的 store_id）",
			codemeta.InvoiceRecipientTypeSeller.Value, ptrInt64(fxOtherStoreID), ""},
		{"方向三 EXTERNAL（ref 為 NULL）", codemeta.InvoiceRecipientTypeExternal.Value, nil, ""},
		{"方向四 EXTERNAL 但 ref 非空（髒資料）",
			codemeta.InvoiceRecipientTypeExternal.Value, ptrInt64(fxOtherStoreID), ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := fxDoc(c.recipientType, c.recipientRef)
			for _, p := range eventProducers(t, doc) {
				var got map[string]any
				if err := json.Unmarshal(p.Payload, &got); err != nil {
					t.Fatalf("%s: unmarshal: %v", p.Name, err)
				}
				raw, present := got["buyer_id"]
				if !present {
					continue // P11 是純賣家向事件，沒有 buyer_id 欄位
				}
				// 申訴類事件的 buyer_id 來自 complainant_buyer_id，與 recipient_type 無關。
				// P10 同屬這一類（#3055 的橫向稽查）：逾期是那件**申訴**的屬性，
				// 不是被申訴文件的收件人。
				want := c.wantDocBuyer
				switch p.Name {
				case "P8 complaintCreatedEvent", "P9 complaintRespondedEvent",
					"P10 complaintOverdueEvent":
					want = "2688637900"
				}
				if raw != want {
					t.Fatalf("%s 的 buyer_id = %#v，want %q。\n"+
						"若它等於 %d，那是**另一個賣家的 store_id** 被裝進了 buyer_id ——"+
						"§6.3.9 模板的 target_user_field 正是它 ⇒ 通知會送給一個不存在的買家，"+
						"或更糟，送給 buyer_id 恰好等於該 store_id 的另一個人。",
						p.Name, raw, want, fxOtherStoreID)
				}
			}
		})
	}
}

// goldenPath 是跨服務 payload 契約快照的位置（repo 根的 tests/contract/golden/）。
const goldenPath = "../../../../tests/contract/golden/event_payload_keys.json"

// TestProducersMatchTheGoldenEmittedKeys 把**真的發出去的 key 集合**與 golden 快照對拍。
//
// 🔴 這一支補的是一個已知盲區：既有的 golden gate 只驗
// 「notification template 讀的 key ⊆ golden」，**不驗** 「golden ⊆ publisher 實發」
// ——golden 手寫錯了不會被抓（#1529 賣家送達通知因此靜默 0 投遞）。
// 這裡是反方向：拿**生產者實際 marshal 出來的東西**去對 golden，兩個方向都有人守才算閉合。
//
// 為什麼順便值得做：本輪把六個事件的 store_id／buyer_id 換成了 eventwire 的不透明型別。
// 型別換了而 wire 形狀必須一字不變 —— 這一支就是那句話的證據，不是我的宣稱。
func TestProducersMatchTheGoldenEmittedKeys(t *testing.T) {
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		// 🔴 目標不存在一律 FAIL 不 SKIP：檔案被搬走時 SKIP 會讓這道檢查靜默消失。
		t.Fatalf("讀不到 golden 快照 %s：%v", goldenPath, err)
	}
	var golden map[string]struct {
		EmittedKeys []string `json:"emitted_keys"`
		RoutingKey  string   `json:"routing_key"`
		Publisher   string   `json:"publisher"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("golden 解不開：%v", err)
	}

	// 生產點 → golden 條目。同一個 payload 型別供多個條目時（issuedEvent 供 buyer／seller／
	// receipt 三條）逐條都要對得上 —— 三條的 emitted_keys 必須一致，否則就是快照自己漂了。
	want := map[string][]string{
		"P1/P6 issuedEvent（開立）":       {"invoice.issued.buyer", "invoice.issued.seller", "invoice.receipt.issued.buyer"},
		"P1/P6 issuedEvent（重寄）":       {"invoice.issued.buyer", "invoice.issued.seller", "invoice.receipt.issued.buyer"},
		"P2 issueFailedEvent":         {"invoice.issue_failed.seller"},
		"P8 complaintCreatedEvent":    {"invoice.complaint.created.seller"},
		"P9 complaintRespondedEvent":  {"invoice.complaint.responded.buyer"},
		"P12 resolvedExternallyEvent": {"invoice.resolved_externally.buyer"},
		"P11 taxStatusChangedEvent":   {"invoice.seller_tax_profile.status_changed.seller"},
		"P5 reissuedEvent":            {"invoice.reissued.buyer"},
		"P3 voidedEvent":              {"invoice.voided.buyer"},
		"P7 reviewRequiredEvent":      {"invoice.operation.review_required.seller"},
		"P10 complaintOverdueEvent":   {"invoice.complaint.overdue.seller"},
		"P4 allowanceIssuedEvent":     {"invoice.allowance.issued.buyer"},
	}

	doc := fxDoc(codemeta.InvoiceRecipientTypeBuyer.Value, ptrInt64(fxBuyerID))
	for _, p := range eventProducers(t, doc) {
		entries, mapped := want[p.Name]
		if !mapped {
			t.Fatalf("生產點 %q 沒有對應的 golden 條目 —— 新增事件時必須同時登記，"+
				"否則它的 wire 形狀沒有任何東西在守", p.Name)
		}
		var got map[string]any
		if err := json.Unmarshal(p.Payload, &got); err != nil {
			t.Fatalf("%s: unmarshal: %v", p.Name, err)
		}
		gotKeys := make([]string, 0, len(got)+1)
		for k := range got {
			gotKeys = append(gotKeys, k)
		}
		// notify_seller_account_id 由 outbox dispatcher 在派送時補（§6.3.3.2），
		// 生產者不填 —— 但它在 golden 裡，所以比對前補上。
		gotKeys = append(gotKeys, notifyrouting.KeyNotifySellerAccountID)
		sort.Strings(gotKeys)

		for _, name := range entries {
			g, ok := golden[name]
			if !ok {
				t.Fatalf("golden 沒有條目 %q（%s 的對拍對象）", name, p.Name)
			}
			if g.Publisher != "invoice-service" {
				t.Fatalf("golden 條目 %q 的 publisher 是 %q", name, g.Publisher)
			}
			wantKeys := append([]string(nil), g.EmittedKeys...)
			sort.Strings(wantKeys)
			if strings.Join(gotKeys, ",") != strings.Join(wantKeys, ",") {
				t.Errorf("%s 實發的 key 集合與 golden %q 不一致：\n  實發 %v\n  golden %v\n"+
					"多出來的 key ＝ golden 沒登記（下游模板讀不到）；少掉的 key ＝ golden 騙人"+
					"（模板照 golden 寫，執行期拿到 undefined）。",
					p.Name, name, gotKeys, wantKeys)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 行為層的覆蓋下界：從 AST 掃到的型別清單推導，不是硬寫的數字（丙）
// ---------------------------------------------------------------------------

// assertProducersCoverEveryPayloadType 是本檔所有「逐生產點」測試共用的**下界**。
//
// 🔴 它取代的是 `if len(producers) < 7`。那個寫法有一個安靜的缺口：
// 新增第 7 個事件型別的人不會回來改這裡的 7，而 7 本來就是 7 ⇒ 那個新事件
// **完全沒有行為層覆蓋**。而行為層正是「型別 ＋ AST 兩道都放行的繞道」唯一擋得住的線
// （本輪的第六種繞道就是那個形狀），所以它漏掉的正好是最需要它的那一格。
//
// 現在的下界是：**checkEventPayloads 掃到的每一個 payload 型別都必須有生產點**。
// 那份清單是從原始碼 AST 來的（不是任何人的自述），所以新增事件型別而沒有生產點時，
// 這裡會直接指名那個型別轉紅。
func assertProducersCoverEveryPayloadType(t *testing.T, producers []producer) {
	t.Helper()
	fset, files := parsePackageFiles(t, ".")
	rep := checkEventPayloads(fset, files)

	// 鑑別力自檢：辨識器認不出任何型別時，「每個型別都有生產點」會空洞地成立。
	if len(rep.PayloadTypes) == 0 {
		t.Fatal("AST 掃不到任何事件 payload 型別 —— 這不是「全部都有生產點」，是下界失效了")
	}
	if len(producers) == 0 {
		t.Fatal("一個生產點都沒有")
	}

	tags := payloadJSONTags(t, fset, files)

	covered := map[string]bool{}
	for _, p := range producers {
		if _, known := tags[p.Type]; !known {
			t.Fatalf("生產點 %q 宣稱產出 %q，而 AST 在本 package 裡找不到這個型別。\n"+
				"Type 不是標籤是斷言：填錯就是這裡紅，不會靜默變成一個好聽的字串。\n"+
				"掃到的型別：%v", p.Name, p.Type, rep.PayloadTypes)
		}
		covered[p.Type] = true

		// Type ↔ payload 的綁定：實際 marshal 出來的 key 集合必須等於該型別的 json tag
		// 集合。少了這一步，Type 只是一個沒人驗的字串，填成任何已存在的型別名都會過。
		var got map[string]any
		if err := json.Unmarshal(p.Payload, &got); err != nil {
			t.Fatalf("%s: unmarshal: %v", p.Name, err)
		}
		gotKeys := make([]string, 0, len(got))
		for k := range got {
			gotKeys = append(gotKeys, k)
		}
		sort.Strings(gotKeys)
		wantKeys := append([]string(nil), tags[p.Type]...)
		sort.Strings(wantKeys)
		if strings.Join(gotKeys, ",") != strings.Join(wantKeys, ",") {
			t.Errorf("生產點 %q 宣稱產出 %s，但實際 marshal 出來的 key 集合對不上該型別：\n"+
				"  實發 %v\n  %s 的 json tag %v", p.Name, p.Type, gotKeys, p.Type, wantKeys)
		}
	}

	for _, tn := range rep.PayloadTypes {
		if covered[tn] {
			continue
		}
		t.Errorf("事件 payload 型別 %s **沒有任何行為層生產點**。\n"+
			"型別（internal/eventwire）與 AST（event_store_id_contract_test.go）兩道守的是"+
			"「值從哪裡來」，守不了「那個值到了下游解不解得開」——後者只有把真正的生產者跑一遍"+
			"才驗得到，而 2026-08-31 的第六種繞道（就地捏一個 domain.Issuer）正是兩道都放行的形狀。\n"+
			"把它加進 eventProducers，或說明它為什麼不會被送出去（那樣的話它也不該存在）。",
			tn)
	}
}

// TestEveryScannedPayloadTypeHasABehaviouralProducer 讓上面那條下界自己成為一支測試，
// 這樣「新增事件型別忘了加生產點」不必等到別的測試碰巧跑到才浮現。
func TestEveryScannedPayloadTypeHasABehaviouralProducer(t *testing.T) {
	doc := fxDoc(codemeta.InvoiceRecipientTypeBuyer.Value, ptrInt64(fxBuyerID))
	assertProducersCoverEveryPayloadType(t, eventProducers(t, doc))
}

// payloadJSONTags 回「型別名 → 它的 json tag 清單」。
//
// 只收 tag 非空且不是 "-" 的欄位；這些 struct 全部沒有 omitempty（有的話 key 集合會
// 隨值變動，本比對就必須改成子集關係）。真的出現 omitempty 時這裡會誤報，那是刻意的：
// 一個會依值消失的 wire 欄位，下游拿到的是 undefined，本來就該當場被看見。
func payloadJSONTags(t *testing.T, fset *token.FileSet, files []*ast.File) map[string][]string {
	t.Helper()
	rep := checkEventPayloads(fset, files)
	byName := map[string]*ast.StructType{}
	for _, f := range files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					byName[ts.Name.Name] = st
				}
			}
		}
	}
	out := map[string][]string{}
	for _, tn := range rep.PayloadTypes {
		st, ok := byName[tn]
		if !ok {
			t.Fatalf("AST 認出 payload 型別 %s，卻找不到它的 struct 宣告 —— 辨識器壞了", tn)
		}
		var tagList []string
		for _, fld := range st.Fields.List {
			tag := jsonTagOf(fld)
			if tag == "" || tag == "-" {
				continue
			}
			if strings.Contains(tag, "omitempty") {
				t.Fatalf("%s 的欄位 tag %q 帶 omitempty —— wire 欄位依值消失，"+
					"下游拿到的是 undefined", tn, tag)
			}
			tagList = append(tagList, tag)
		}
		if len(tagList) == 0 {
			t.Fatalf("payload 型別 %s 一個 json tag 都沒有", tn)
		}
		out[tn] = tagList
	}
	return out
}
