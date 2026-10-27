//go:build integration

// 本檔跑真的 SQL。它與 query_integration_test.go 同一條理由，但守的是**本波新增**的
// 那批唯讀端點：
//
//	① Tier A 的擁有者述詞住在 SQL 的 WHERE —— 把 `AND i.issuer_ref_id = $2` 拿掉，
//	   單元測試的 fake 照樣正確、照樣全綠，只有真的 PostgreSQL 會把別人的那一列交出來
//	② 遮罩的三面對照 —— 「DB 裡是密文」這件事只有真 DB 答得出來，
//	   而它正是「賣家看到的遮罩值真的來自解密」的前提
//	③ total 與 records 是否同一個母體 —— fake 的 Count 是我寫的，
//	   真正會漂移的是 db/queries 裡那兩句述詞
//
// 跑法：`make test-integration`（需要本機 Docker daemon）。
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
)

// ---------------------------------------------------------------------------
// §8.1.3(b)：同一份文件、三個視角、三種形態
// ---------------------------------------------------------------------------

// TestIntegration_QueryRead_ThreeFacesOfTheSameCarrier 是本波最重要的一支。
//
// 它把 §8.1.3 的三格一次驗完，而且**必須是同一支測試**：
//
//	前提｜DB 裡是密文     —— 否則後面看到的「明碼」可能根本沒經過解密
//	買家｜完整            —— 資料主體（裁定 1：遮掉後四碼他永遠發現不了自己打錯一碼）
//	賣家｜遮罩            —— 非資料主體。**這是 MaskCarrierID 的第一條生產路徑**
//	ADM ｜明碼 ＋ 留稽核  —— 裁定 2 的兩半，缺一都不成立
//
// 拆成三支的話，每一支都可以靠「把另外兩格改成跟自己一樣」而通過。
func TestIntegration_QueryRead_ThreeFacesOfTheSameCarrier(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)
	doc := qitSeedDocument(t, env, env.issuerID+1100, qitBuyerA, "O000001101")

	// 前提：落庫的是密文。這一句同時證明後面看到的明碼真的來自解密。
	var stored string
	if err := env.pool.QueryRow(ctx,
		`SELECT carrier_id1_ciphertext FROM invoice_documents WHERE id=$1`, doc.ID).Scan(&stored); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if stored == qitCarrierPlain {
		t.Fatal("🔴 載具明碼直接落庫了（§5.1.5 要求密文儲存）")
	}

	// ── 買家面：完整 ──
	buyerView, err := env.query.GetBuyerDocument(ctx, qitBuyerA, doc.ID)
	if err != nil {
		t.Fatalf("買家應讀得到自己的文件：%v", err)
	}
	if buyerView.CarrierSummary != qitCarrierPlain {
		t.Fatalf("🔴 買家是資料主體，載具應為完整值（§8.1.3(b)），實得 %q", buyerView.CarrierSummary)
	}

	// ── 賣家面：遮罩 ──
	sellerView, err := env.query.GetSellerDocument(ctx, env.storeID, doc.ID)
	if err != nil {
		t.Fatalf("賣家應讀得到自己開出的文件：%v", err)
	}
	const wantMasked = "/T••••+P"
	if sellerView.Detail.CarrierSummary != wantMasked {
		t.Fatalf("🔴 賣家不是資料主體，載具必須遮罩成 %q，實得 %q", wantMasked, sellerView.Detail.CarrierSummary)
	}
	// 判準的第二層：**整個投影裡找不到明碼**。只驗欄位值擋不住
	// 「有人多帶一個欄位把明碼撈出去」，而那正是 write-only 語意最常見的破法。
	if blob := fmt.Sprintf("%+v", sellerView); strings.Contains(blob, qitCarrierPlain) {
		t.Fatalf("🔴 買家載具明碼出現在賣家的投影裡：%s", blob)
	}
	if strings.Contains(fmt.Sprintf("%+v", sellerView), qitEmailPlain) {
		t.Fatal("🔴 買家 email 明碼出現在賣家的投影裡")
	}
	// 賣家讀取**不得**留 PII 揭露稽核：他看到的是遮罩值，沒有揭露任何 P 類明文。
	// 留了的話 InvoicePiiRevealSpike 告警會被賣家的日常操作淹沒。
	if n := countPiiReveals(t, env, doc.ID); n != 0 {
		t.Fatalf("🔴 賣家面（遮罩）不該留 PII 揭露稽核，實得 %d 列", n)
	}

	// ── ADM 面：明碼 ＋ 稽核 ──
	const actorID = int64(90101)
	adminView, err := env.query.GetDocumentForAdmin(ctx, actorID, doc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adminView.RevealedCarrierID != qitCarrierPlain {
		t.Fatalf("🔴 ADM 端不遮罩（§8.1.3(c) 裁定 2），實得 %q", adminView.RevealedCarrierID)
	}
	if adminView.RevealedEmail != qitEmailPlain {
		t.Fatalf("🔴 ADM 端 email 應為明碼，實得 %q", adminView.RevealedEmail)
	}
	if n := countPiiReveals(t, env, doc.ID); n != 1 {
		t.Fatalf("🔴 裁定 2 的另一半：每次 ADM 揭露必須留一列 INVOICE_PII_REVEAL，實得 %d 列", n)
	}
}

// countPiiReveals 回這份文件目前有幾列 INVOICE_PII_REVEAL 稽核。
func countPiiReveals(t *testing.T, env *qitEnv, documentID int64) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM invoice_admin_audit_logs
		  WHERE action='INVOICE_PII_REVEAL' AND resource_type='INVOICE_DOCUMENT' AND resource_id=$1`,
		documentID).Scan(&n); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	return n
}

// failingAudit 讓稽核寫入失敗，其餘讀取照常。
type failingAudit struct {
	domain.AuditRepository
}

var errAuditDown = errors.New("audit table down")

func (failingAudit) AppendAdminAuditLog(context.Context, domain.NewAdminAuditLogEntry) (int64, error) {
	return 0, errAuditDown
}

// TestIntegration_QueryRead_AdminRevealFailsClosedWhenAuditFails 釘住 fail-closed。
//
// 裁定 2 的代價由稽核承擔 ⇒ 稽核寫不進去就不揭露。少了這一條，稽核在資料庫出問題時
// 靜默失效而畫面完全正常 —— 那正是「機制只讓問題被發現、沒讓問題不發生」的反面。
func TestIntegration_QueryRead_AdminRevealFailsClosedWhenAuditFails(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)
	doc := qitSeedDocument(t, env, env.issuerID+1200, qitBuyerA, "O000001201")

	ring := itKeyRing(t)
	broken := NewQuery(
		postgres.NewIssuerRepository(env.pool),
		postgres.NewSellerTaxProfileRepository(env.pool),
		postgres.NewProviderAccountRepository(env.pool, ring),
		postgres.NewBuyerPreferenceRepository(env.pool, ring),
		env.docs,
		postgres.NewOperationRepository(env.pool),
		postgres.NewComplaintRepository(env.pool),
		failingAudit{AuditRepository: postgres.NewAuditRepository(env.pool)},
		postgres.NewAdjustmentRepository(env.pool),
		postgres.NewCorrectionRequestRepository(env.pool),
		postgres.NewOverviewRepository(env.pool),
		postgres.NewBusinessDayRepository(env.pool),
		postgres.NewProviderCutoverRepository(env.pool),
		postgres.NewManualUnlockRepository(env.pool),
		postgres.NewCancelledOrderRepository(env.pool),
		IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute},
		nil,
	)

	view, err := broken.GetDocumentForAdmin(ctx, 90201, doc.ID)
	if err == nil {
		t.Fatal("🔴 稽核寫入失敗時不得回資料")
	}
	if !errors.Is(err, errAuditDown) {
		t.Fatalf("錯誤應由稽核寫入失敗造成，實得 %v", err)
	}
	// 回傳值必須是零值：帶著明碼的 view ＋ 一個 error，呼叫端只要漏判一次 err
	// 就等於繞過了整條 fail-closed。
	if view.RevealedCarrierID != "" || view.RevealedEmail != "" {
		t.Fatalf("🔴 fail-closed 時仍帶出了明碼：carrier=%q email=%q", view.RevealedCarrierID, view.RevealedEmail)
	}
}

// ---------------------------------------------------------------------------
// Tier A：擁有者述詞必須在 SQL 裡
// ---------------------------------------------------------------------------

// TestIntegration_QueryRead_TierA_SellerDocumentOwnerIsInSQLPredicate 是
// 「把 `AND i.issuer_ref_id = $2` 從 GetSellerInvoiceDocumentByID 拿掉」這個變異的
// 唯一偵測點。fake repository 會忠實照述詞回答，所以只有真 DB 抓得到。
func TestIntegration_QueryRead_TierA_SellerDocumentOwnerIsInSQLPredicate(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)
	doc := qitSeedDocument(t, env, env.issuerID+1300, qitBuyerA, "O000001301")

	if _, err := env.query.GetSellerDocument(ctx, env.storeID, doc.ID); err != nil {
		t.Fatalf("擁有者應讀得到自己的文件：%v", err)
	}

	// (a) 別的 store 讀同一份文件 —— 這一列**存在**，只是不屬於他。
	_, foreignErr := env.query.GetSellerDocument(ctx, env.storeID+1, doc.ID)
	if foreignErr == nil {
		t.Fatal("🔴 越權讀取成功了 —— issuer_ref_id 沒有進 SQL 的 WHERE（§8.1.4(b)）")
	}
	// (b) 同一個 store 讀一個不存在的 document_id。
	_, missingErr := env.query.GetSellerDocument(ctx, env.storeID+1, doc.ID+999_999)
	if missingErr == nil {
		t.Fatal("不存在的 document_id 應該回錯誤")
	}
	assertSameDomainError(t, foreignErr, missingErr,
		"越權與不存在必須回同一個碼與同一段訊息（§8.1.4(c)：由同一行程式碼產生）")
	if !isBizCode(foreignErr, bizcode.InvoiceDocumentNotFound) {
		t.Fatalf("預期業務碼 %d，實得 %v", bizcode.InvoiceDocumentNotFound, foreignErr)
	}
}

// TestIntegration_QueryRead_TierA_BuyerDocumentListIsOwnerScoped 是列表版。
//
// 別人的 buyer_id 得到的不是錯誤，而是與「這位買家還沒有任何文件」完全相同的空列表
// ——連 total 都要是 0。只驗 records 為空而不驗 total 的話，
// 「records 有述詞、count 沒有」這個漂移不會被抓到（症狀：說有 30 筆、翻頁全是空的）。
func TestIntegration_QueryRead_TierA_BuyerDocumentListIsOwnerScoped(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)
	qitSeedDocument(t, env, env.issuerID+1400, qitBuyerA, "O000001401")

	mine, err := env.query.ListBuyerDocuments(ctx, BuyerDocumentsInput{BuyerID: qitBuyerA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mine.Total != 1 || len(mine.Records) != 1 {
		t.Fatalf("擁有者應看到 1 筆，實得 total=%d len=%d", mine.Total, len(mine.Records))
	}

	other, err := env.query.ListBuyerDocuments(ctx, BuyerDocumentsInput{BuyerID: qitBuyerB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(other.Records) != 0 {
		t.Fatalf("🔴 另一位買家看到了 %d 筆 —— recipient_ref_id 沒有進 List 的 WHERE", len(other.Records))
	}
	if other.Total != 0 {
		t.Fatalf("🔴 另一位買家的 total 是 %d —— Count 的述詞與 List 不一致", other.Total)
	}
}

// ---------------------------------------------------------------------------
// total 與 records 必須是同一個母體
// ---------------------------------------------------------------------------

// TestIntegration_QueryRead_TotalEqualsSumOfPages 是分頁的鑑別力測試。
//
// 判準有三層，缺一都會讓一種真實的失敗溜過去：
//
//	① total 必須等於各頁列數相加      —— 擋 total 用 len(records) 充數
//	② total 在翻頁之間不得改變        —— 擋 List 與 Count 述詞漂移（兩個母體）
//	③ 同一份文件不得跨頁出現兩次      —— 擋排序鍵不穩定造成的 OFFSET 漏列／重複
func TestIntegration_QueryRead_TotalEqualsSumOfPages(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)

	const want = 5
	for i := 0; i < want; i++ {
		qitSeedDocument(t, env, env.issuerID+1500+int64(i), qitBuyerA, fmt.Sprintf("O00000150%d", i))
	}

	seen := map[int64]int{}
	summed := 0
	total := int32(-1)
	// page_size=2 ⇒ 5 筆會落在 3 頁（2/2/1），最後一頁不滿也要正確。
	for page := int32(1); page <= 4; page++ {
		got, err := env.query.ListBuyerDocuments(ctx, BuyerDocumentsInput{
			BuyerID: qitBuyerA, Page: page, PageSize: 2,
		})
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if total < 0 {
			total = got.Total
		}
		if got.Total != total {
			t.Fatalf("🔴 total 在翻頁之間從 %d 變成 %d —— records 與 total 分屬兩個母體", total, got.Total)
		}
		for _, rec := range got.Records {
			seen[rec.Document.ID]++
			summed++
		}
	}

	if total != want {
		t.Fatalf("total 應為 %d，實得 %d", want, total)
	}
	if summed != want {
		t.Fatalf("🔴 各頁列數相加 %d ≠ total %d —— 分頁漏列或重複", summed, total)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("🔴 文件 %d 在跨頁時出現 %d 次（排序鍵不穩定）", id, n)
		}
	}
}

// ---------------------------------------------------------------------------
// 端點 8：作廢重開鏈上的每一份都要回
// ---------------------------------------------------------------------------

// TestIntegration_QueryRead_OrderInvoiceReturnsWholeReissueChain 是
// 「ListOrderInvoiceDocumentsForBuyer 必須是 :many」那條註解的偵測點。
//
// 舊實作（GetInvoiceDocumentForBuyer，`:one`）在同一組述詞命中兩列時 **pgx 取任意
// 一列而不報錯**（連 LIMIT 都沒有，順序未定義），症狀是買家看到作廢前的舊發票
// 而畫面完全正常。
func TestIntegration_QueryRead_OrderInvoiceReturnsWholeReissueChain(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)
	const orderNo = "O000001601"

	first := qitSeedDocument(t, env, env.issuerID+1600, qitBuyerA, orderNo)
	second := qitSeedDocumentWithVersion(t, env, env.issuerID+1601, qitBuyerA, orderNo, 2)

	view, err := env.query.GetOrderInvoice(ctx, qitBuyerA, orderNo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(view.Documents) != 2 {
		t.Fatalf("🔴 這張訂單有 2 份文件，回應只有 %d 份 —— 只取第一筆的實作會讓買家看不到另一張", len(view.Documents))
	}
	got := map[int64]bool{}
	for _, d := range view.Documents {
		got[d.Document.ID] = true
	}
	if !got[first.ID] || !got[second.ID] {
		t.Fatalf("兩份文件都要在，實得 %v", got)
	}
	// 排序是 snapshot_version DESC ⇒ 第一列是最新那一份，state／snapshot_version 取它。
	if view.Documents[0].Document.ID != second.ID {
		t.Fatalf("最新一份（snapshot_version=2）應排最前，實得 id=%d", view.Documents[0].Document.ID)
	}
	if view.SnapshotVersion != 2 {
		t.Fatalf("snapshot_version 應取最新一份（2），實得 %d", view.SnapshotVersion)
	}

	// 買家是資料主體 ⇒ 兩份都是完整載具（有載具的那一份）。
	if view.NotifiedEmailMasked != MaskEmail(qitEmailPlain) {
		t.Fatalf("notified_email_masked 應為遮罩值 %q，實得 %q", MaskEmail(qitEmailPlain), view.NotifiedEmailMasked)
	}

	// 別的買家拿到的是「還沒付款」，不是這兩份文件。
	foreign, err := env.query.GetOrderInvoice(ctx, qitBuyerB, orderNo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(foreign.Documents) != 0 || foreign.State != orderInvoiceStateAwaitingPayment {
		t.Fatalf("🔴 越權讀到了文件：%d 份、state=%q", len(foreign.Documents), foreign.State)
	}
}

// qitSeedDocumentWithVersion 與 qitSeedDocument 相同，但可指定 snapshot_version
// （作廢重開鏈的第二份）。
func qitSeedDocumentWithVersion(t *testing.T, env *qitEnv, docID, buyerID int64, orderNo string, version int32) domain.Document {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(48 * time.Hour)
	taipei := time.Now().UTC()
	doc, inserted, err := env.docs.Create(ctx, domain.NewDocument{
		ID: docID, IssuerID: env.issuerID, IssuerNameSnapshot: "查詢整合測試賣場",
		SourceType: "ORDER", SourceRef: orderNo, TriggerType: "AUTO",
		RecipientType: "BUYER", RecipientRefID: &buyerID,
		RecipientName:  "測試買家",
		RecipientTaxID: "0000000000",
		RecipientEmail: qitEmailPlain,
		CarrierID1:     qitCarrierPlain,
		CarrierType:    "3J0002",
		DocumentKind:   "B2C", FulfillmentMode: "AMEGO_AUTO",
		Status:          codemeta.InvoiceDocumentStatusPending.Value,
		SnapshotVersion: version, IssueAttemptSeq: 1,
		SalesAmount: 100, TaxAmount: 0, TotalAmount: 100, TaxType: 1, TaxRateBps: 500,
		Provider: "AMEGO", ProviderRequestNo: fmt.Sprintf("R%d", docID),
		InvoiceDateTaipei: &taipei, TaxPeriodCode: "11507",
		IssueNotBefore:      time.Now().UTC(),
		StatutoryDeadlineAt: &deadline,
		SourceSnapshot:      []byte(`{"orderNo":"` + orderNo + `"}`),
		Now:                 time.Now().UTC(),
	})
	if err != nil || !inserted {
		t.Fatalf("seed document v%d: err=%v inserted=%v", version, err, inserted)
	}
	return doc
}

// ---------------------------------------------------------------------------
// S12：用途說明由明細行推導
// ---------------------------------------------------------------------------

// TestIntegration_QueryRead_ReceivedNoteIsDerivedFromLines 釘住 proto 對
// recipient_note 的定案（不加 DB 欄、由 invoice_document_lines 推導）。
//
// 兩個分支都要驗：一行 ⇒ 就是那一行的品名；多行 ⇒ 後綴「等 N 項」。
// 只驗其一的話「行數永遠取 1」或「永遠加後綴」都不會紅。
func TestIntegration_QueryRead_ReceivedNoteIsDerivedFromLines(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)

	single := qitSeedReceivedDocument(t, env, env.issuerID+1700, []string{"平台服務費"})
	multi := qitSeedReceivedDocument(t, env, env.issuerID+1701, []string{"平台服務費", "金流手續費", "廣告費"})

	got, err := env.query.ListSellerReceivedDocuments(ctx, ReceivedDocumentsInput{StoreID: env.storeID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 2 || len(got.Records) != 2 {
		t.Fatalf("應有 2 筆進項，實得 total=%d len=%d", got.Total, len(got.Records))
	}
	notes := map[int64]string{}
	for _, r := range got.Records {
		notes[r.Document.ID] = r.RecipientNote
	}
	if notes[single.ID] != "平台服務費" {
		t.Fatalf("單行文件的用途說明應是那一行的品名，實得 %q", notes[single.ID])
	}
	if notes[multi.ID] != "平台服務費等 3 項" {
		t.Fatalf("多行文件應後綴「等 N 項」，實得 %q", notes[multi.ID])
	}

	// 進項的授權述詞是 recipient_type='SELLER' 對 recipient_ref_id ——
	// 與銷項（issuer 側）是**兩套述詞**，不得互相回退。
	foreign, err := env.query.ListSellerReceivedDocuments(ctx, ReceivedDocumentsInput{StoreID: env.storeID + 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if foreign.Total != 0 || len(foreign.Records) != 0 {
		t.Fatalf("🔴 別的 store 讀到了進項：total=%d len=%d", foreign.Total, len(foreign.Records))
	}
}

// qitSeedReceivedDocument 建一份「開給這位賣家」的進項文件（recipient_type=SELLER）。
//
// source_ref 必須留空：chk_invdoc_source_ref 是雙向等式
// （source_type='ORDER' ⇔ source_ref IS NOT NULL），MANUAL 帶了 ref 就是 23514。
func qitSeedReceivedDocument(t *testing.T, env *qitEnv, docID int64, lineNames []string) domain.Document {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	storeID := env.storeID
	// chk_invdoc_deadline_presence 是**雙向等式**：document_kind ∈ {B2C,B2B} ⇔
	// statutory_deadline_at IS NOT NULL。B2B 少給它就是 23514，而不是「少一欄」。
	deadline := now.Add(7 * 24 * time.Hour)
	doc, inserted, err := env.docs.Create(ctx, domain.NewDocument{
		ID: docID, IssuerID: env.issuerID, IssuerNameSnapshot: "範例生鮮",
		// 方向三／四由端點 57 產生，走 MANUAL_LINES。
		SourceType: "MANUAL", TriggerType: "ADM_MANUAL",
		RecipientType: "SELLER", RecipientRefID: &storeID,
		RecipientName: "查詢整合測試賣場", RecipientTaxID: "12345678",
		DocumentKind: "B2B", FulfillmentMode: "AMEGO_AUTO",
		// 建成 PENDING 而不是 ISSUED：chk_invdoc_issued_has_no 要求 ISSUED 必須帶
		// 發票號碼，而號碼只有 MarkIssued 這條路寫得進去。本支驗的是 recipient_note
		// 的推導與擁有者述詞，兩者都與 status 無關（列表沒有帶 status 篩選）。
		Status:          codemeta.InvoiceDocumentStatusPending.Value,
		SnapshotVersion: 1, IssueAttemptSeq: 1,
		SalesAmount: 100, TaxAmount: 5, TotalAmount: 105, TaxType: 1, TaxRateBps: 500,
		Provider: "AMEGO", ProviderRequestNo: fmt.Sprintf("R%d", docID),
		InvoiceDateTaipei: &now, TaxPeriodCode: "11507",
		IssueNotBefore:      now,
		StatutoryDeadlineAt: &deadline,
		SourceSnapshot:      []byte(`{}`),
		Now:                 now,
	})
	if err != nil || !inserted {
		t.Fatalf("seed received document: err=%v inserted=%v", err, inserted)
	}

	lines := make([]domain.NewDocumentLine, 0, len(lineNames))
	for i, name := range lineNames {
		lines = append(lines, domain.NewDocumentLine{
			LineSeq: int32(i + 1), LineType: "PRODUCT", Description: name,
			Quantity: 1, UnitPrice: 100, NetAmount: 100, TaxCategory: 1, TaxRateBps: 500,
			AllowanceableBase: 100,
		})
	}
	if err := env.docs.InsertLines(ctx, doc.ID, lines, now); err != nil {
		t.Fatalf("seed lines: %v", err)
	}
	return doc
}
