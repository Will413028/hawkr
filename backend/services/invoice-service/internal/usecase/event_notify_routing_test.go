package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/notifyrouting"
)

const (
	fxIssuerPK      int64 = 7304857280000000001
	fxStoreID       int64 = 2685914682
	fxDocID         int64 = 7304857280000000002
	fxBuyerID       int64 = 2623900520
	fxOtherStoreID  int64 = 2632049831
	fxComplainantID int64 = 2688637900
	fxSellerAccount int64 = 4242424242
)

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

type producer struct {
	Name    string
	Type    string
	Payload []byte
}

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
		{"P4 allowanceIssuedEvent", "allowanceIssuedEvent", must((&Allowance{}).allowanceIssuedPayload(
			issuer, doc,
			domain.Adjustment{
				ID: 6001, DocumentID: doc.ID,
				AllowanceNumber: ptrString("AL00000001"),
				GrossAmount:     ptrInt64(315),
				NetAmountExTax:  ptrInt64(300),
				TaxAmount:       ptrInt64(15),
			}, now))},
		{"P10 complaintOverdueEvent", "complaintOverdueEvent", must(complaintOverduePayload(issuer, c, now))},
		{"P3 voidedEvent", "voidedEvent", fromOutbox("P3", voidOutbox,
			(&Voiding{outbox: voidOutbox, issuers: &fakeIssuerRepo{issuer: issuer}}).
				emitVoided(ctx, doc, now))},
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

			if got["store_id"] != "2685914682" {
				t.Fatalf("store_id = %#v，want \"2685914682\"（invoice_issuers.issuer_ref_id）。\n"+
					"若它等於 %d，就是又填回了 invoice_issuers 的 PK。",
					got["store_id"], fxIssuerPK)
			}
			if len(r.asked) != 1 || r.asked[0] != fxStoreID {
				t.Fatalf("dispatcher 拿去解析的值 = %v，want [%d]。\n"+
					"空陣列代表 payload 根本沒有 store_id 鍵 —— 賣家向通知 100%% 消失。",
					r.asked, fxStoreID)
			}
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
		wantDocBuyer  string
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
					continue
				}
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

func assertProducersCoverEveryPayloadType(t *testing.T, producers []producer) {
	t.Helper()
	fset, files := parsePackageFiles(t, ".")
	rep := checkEventPayloads(fset, files)

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
