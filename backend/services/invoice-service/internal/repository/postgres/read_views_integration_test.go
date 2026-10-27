//go:build integration

// 讀取視圖（分頁列表 ＋ owner-scoped 明細）的真 SQL 測試。
//
// 這一批的兩個核心失效形狀**都不是編譯錯，也都不會讓 happy path 轉紅**：
//
//	① 擁有者述詞漏掉／寫錯 ⇒ A 讀得到 B 的發票。只驗「A 讀得到自己的」永遠是綠的
//	   ——每一支都必須同時斷言**陰性**（A 讀不到 B 的），那才是 Tier A 的保證本身。
//	② List 與 Count 的述詞漂移 ⇒ 總數說有 N 筆、翻到某一頁是空的。只驗第一頁抓不到，
//	   必須**跨頁**把每一頁的筆數加起來與 total 對帳。
//
// 跑法：`make test-integration`（需要本機 Docker daemon）。共用 integration_test.go
// 的容器與 dbtest 設定。
package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// seed helpers
//
// 每個 helper 只填「讓 CHECK 全部成立」的最小欄位集合。本表的 CHECK 有四條是
// **雙向等式**（voided／na_reason／external_no／carrier_pair），少寫一半當場 23514
// ——所以這些 helper 刻意不接受「隨便組一組欄位」的參數。
// ---------------------------------------------------------------------------

// seedWorld 是一組完整的、互不相干的測試資料：一個賣家 issuer ＋ 一個買家，
// 底下有文件／operation／申訴／更正申請／稅務身分／provider 帳號各至少一筆。
type seedWorld struct {
	StoreID      int64
	BuyerID      int64
	IssuerID     int64
	OrderNo      string
	DocumentID   int64 // 銷項（賣家開給買家）
	ReceivedID   int64 // 進項（平台開給賣家）
	JobID        int64
	ComplaintID  int64
	CorrectionID int64
	ProfileID    int64
	AccountID    int64
}

var seedSeq int64

// nextID 產生單調遞增且不與其他測試碰撞的主鍵。
func nextID() int64 {
	seedSeq++
	return time.Now().UnixNano() + seedSeq*1_000
}

func mustExec(ctx context.Context, t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("seed exec failed: %v\nSQL: %s", err, sql)
	}
}

// seedIssuerFor 建一列 SELLER issuer，並回它的 id。
func seedIssuerFor(ctx context.Context, t *testing.T, pool *pgxpool.Pool, storeID int64) int64 {
	t.Helper()
	id := nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_issuers (id, issuer_type, issuer_ref_id, display_name, status)
		 VALUES ($1, 'SELLER', $2, $3, 'active')`,
		id, storeID, fmt.Sprintf("store-%d", storeID))
	return id
}

// seedOrderDocument 建一份「賣家 → 買家」的 B2C 訂單發票。
//
// attemptSeq 進 uq_invdoc_order_identity 的最後一段：同一張訂單要建第二份文件
// （作廢重開）時必須遞增它，否則 ON CONFLICT 之外的路徑會撞 23505。
func seedOrderDocument(ctx context.Context, t *testing.T, pool *pgxpool.Pool,
	issuerID, buyerID int64, orderNo, status string, attemptSeq int32, issuedAt time.Time) int64 {
	t.Helper()
	id := nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_documents (
		   id, issuer_id, issuer_name_snapshot, source_type, source_ref, trigger_type,
		   recipient_type, recipient_ref_id, recipient_name, recipient_tax_id,
		   document_kind, fulfillment_mode, status,
		   snapshot_version, issue_attempt_seq,
		   sales_amount, tax_amount, total_amount, tax_type, tax_rate_bps,
		   provider, provider_invoice_no, provider_issued_at,
		   issue_not_before, statutory_deadline_at)
		 VALUES ($1, $2, 'seed-issuer', 'ORDER', $3, 'AUTO',
		         'BUYER', $4, '消費者', '0000000000',
		         'B2C', 'AMEGO_AUTO', $5,
		         1, $6,
		         100, 0, 100, 1, 500,
		         'AMEGO', $7, $8,
		         now(), now() + interval '2 days')`,
		id, issuerID, orderNo, buyerID, status, attemptSeq,
		// uq_invdoc_provider_invoice_no 是 (provider, provider_invoice_no) 的
		// partial UNIQUE ⇒ 號碼必須逐份唯一，不能拿 attempt_seq 當種子。
		fmt.Sprintf("A%09d", id%1_000_000_000), issuedAt)
	return id
}

// seedReceivedDocument 建一份「平台 → 賣家」的 B2B 進項發票（端點 72 的資料形狀）。
// source_type='MANUAL' ⇒ source_ref 必須是 NULL（chk_invdoc_source_ref 是雙向等式）。
func seedReceivedDocument(ctx context.Context, t *testing.T, pool *pgxpool.Pool,
	issuerID, storeID int64, description string, lineCount int) int64 {
	t.Helper()
	id := nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_documents (
		   id, issuer_id, issuer_name_snapshot, source_type, trigger_type,
		   recipient_type, recipient_ref_id, recipient_name, recipient_tax_id,
		   document_kind, fulfillment_mode, status,
		   snapshot_version, issue_attempt_seq,
		   sales_amount, tax_amount, total_amount, tax_type, tax_rate_bps,
		   provider, provider_invoice_no, provider_issued_at,
		   issue_not_before, statutory_deadline_at)
		 VALUES ($1, $2, '範例生鮮', 'MANUAL', 'ADM_MANUAL',
		         'SELLER', $3, '測試商行', '12345678',
		         'B2B', 'AMEGO_AUTO', 'ISSUED',
		         1, 1,
		         100, 5, 105, 1, 500,
		         'AMEGO', $4, now(),
		         now(), now() + interval '7 days')`,
		id, issuerID, storeID, fmt.Sprintf("BB%08d", id%100000000))
	for seq := 1; seq <= lineCount; seq++ {
		desc := description
		if seq > 1 {
			desc = fmt.Sprintf("%s-%d", description, seq)
		}
		mustExec(ctx, t, pool,
			`INSERT INTO invoice_document_lines (
			   document_id, line_seq, line_type, description, quantity, unit_price, net_amount,
			   tax_category, tax_rate_bps, allowanceable_base)
			 VALUES ($1, $2, 'PRODUCT', $3, 1, 100, 100, 1, 500, 100)`,
			id, seq, desc)
	}
	return id
}

// seedOperation 建一筆 ISSUE operation。
// chk_invop_adjustment_pair：只有 VOID／ALLOWANCE／VOID_ALLOWANCE 需要 adjustment_id。
func seedOperation(ctx context.Context, t *testing.T, pool *pgxpool.Pool, documentID int64, status string) int64 {
	t.Helper()
	id := nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_operations (
		   id, document_id, operation_type, operation_key, provider_request_no,
		   status, request_snapshot_hash, statutory_deadline_at)
		 VALUES ($1, $2, 'ISSUE', $3, $4, $5, 'seedhash', now() + interval '2 days')`,
		id, documentID, fmt.Sprintf("op-%d", id), fmt.Sprintf("req-%d", id), status)
	return id
}

// seedComplaint 建一件申訴。uq_invcomp_open_per_document ⇒ 一份文件只能有一件未結案。
func seedComplaint(ctx context.Context, t *testing.T, pool *pgxpool.Pool,
	documentID, issuerID, buyerID int64, orderNo, status string, slaDueAt time.Time) int64 {
	t.Helper()
	id := nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_complaints (
		   id, document_id, issuer_id, order_no, complainant_buyer_id,
		   reason_code, status, sla_due_at)
		 VALUES ($1, $2, $3, $4, $5, 'NOT_RECEIVED', $6, $7)`,
		id, documentID, issuerID, orderNo, buyerID, status, slaDueAt)
	return id
}

func seedCorrection(ctx context.Context, t *testing.T, pool *pgxpool.Pool, documentID, buyerID int64) int64 {
	t.Helper()
	id := nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_correction_requests (
		   id, document_id, requested_by_buyer_id, expected_snapshot_version,
		   before_payload, after_payload, status)
		 VALUES ($1, $2, $3, 1, '{}'::jsonb, '{}'::jsonb, 'REQUESTED')`,
		id, documentID, buyerID)
	return id
}

func seedTaxProfile(ctx context.Context, t *testing.T, pool *pgxpool.Pool, issuerID int64, reviewStatus string) int64 {
	t.Helper()
	id := nextID()
	// attestation_* 兩欄一律帶上：chk_stp_attested_when_committed 對 SUBMITTED／
	// SELF_ISSUE_PENDING 要求非空（裁決 C 的不變式），而本 helper 的呼叫端就在用那兩個值。
	// 對其他狀態多寫這兩欄無害（chk_stp_attestation_pair 只要求同進同出）。
	mustExec(ctx, t, pool,
		`INSERT INTO seller_tax_profiles (
		   id, issuer_id, review_status, claimed_obligation, legal_name, tax_id,
		   submitted_at, pending_valid_until, attestation_version, attested_at)
		 VALUES ($1, $2, $3, 'REQUIRED', $4, '87654321', now(), now() + interval '30 days', 'v-test', now())`,
		id, issuerID, reviewStatus, fmt.Sprintf("legal-%d", id))
	return id
}

func seedProviderAccountRow(ctx context.Context, t *testing.T, pool *pgxpool.Pool, issuerID int64) int64 {
	t.Helper()
	id := nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_provider_accounts (
		   id, issuer_id, provider, provider_tax_id,
		   credential_ciphertext, credential_fingerprint, credential_last4, status)
		 VALUES ($1, $2, 'AMEGO', '87654321', 'ct', $3, 'abcd', 'CREDENTIAL_SUBMITTED')`,
		id, issuerID, fmt.Sprintf("fp-%d", id))
	return id
}

// buildWorld 建一整組互不相干的資料。兩個 world 之間**沒有任何共用主鍵**，
// 所以任何一支查詢只要漏了擁有者述詞，就會在跨 world 的斷言上轉紅。
func buildWorld(ctx context.Context, t *testing.T, pool *pgxpool.Pool, tag int64) seedWorld {
	t.Helper()
	w := seedWorld{
		StoreID: 700000 + tag,
		BuyerID: 800000 + tag,
		OrderNo: fmt.Sprintf("O-SEED-%d", tag),
	}
	w.IssuerID = seedIssuerFor(ctx, t, pool, w.StoreID)
	w.DocumentID = seedOrderDocument(ctx, t, pool, w.IssuerID, w.BuyerID, w.OrderNo, "ISSUED", 1, time.Now().UTC())
	w.ReceivedID = seedReceivedDocument(ctx, t, pool, w.IssuerID, w.StoreID, "平台服務費", 1)
	w.JobID = seedOperation(ctx, t, pool, w.DocumentID, "MANUAL_PROCESS_ALLOWED")
	w.ComplaintID = seedComplaint(ctx, t, pool, w.DocumentID, w.IssuerID, w.BuyerID, w.OrderNo, "OPEN",
		time.Now().UTC().Add(72*time.Hour))
	w.CorrectionID = seedCorrection(ctx, t, pool, w.DocumentID, w.BuyerID)
	w.ProfileID = seedTaxProfile(ctx, t, pool, w.IssuerID, "SUBMITTED")
	w.AccountID = seedProviderAccountRow(ctx, t, pool, w.IssuerID)
	return w
}

func fullPage() domain.Page { return domain.Page{Limit: 50, Offset: 0} }

// ---------------------------------------------------------------------------
// ① Tier A：每一支 owner-scoped 查詢都必須「A 查不到 B 的」
//
// 🔴 這支測試的價值全在**陰性斷言**。陽性（自己讀得到自己的）人人記得寫，
//    而 approved-only 型保證的實質內容是「非 X 不會 Y」——後者必須有獨立證據，
//    不能由前者推論。
// ---------------------------------------------------------------------------

func TestIntegration_ReadViews_OwnerScopingIsInThePredicate(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	a := buildWorld(ctx, t, pool, 1)
	b := buildWorld(ctx, t, pool, 2)

	docRepo := NewDocumentRepository(pool, testKeyRing(t))
	opRepo := NewOperationRepository(pool)
	complaintRepo := NewComplaintRepository(pool)
	correctionRepo := NewCorrectionRequestRepository(pool)
	taxRepo := NewSellerTaxProfileRepository(pool)
	providerRepo := NewProviderAccountRepository(pool, testKeyRing(t))

	externalStatuses := []string{"MANUAL_PROCESS_ALLOWED", "RESOLVED_EXTERNALLY"}

	// listCases：回 slice 的查詢——越權必須回**空集合**（不是回了再過濾）。
	listCases := []struct {
		name string
		// own 用 A 自己的身分查 A 的資料，cross 用 B 的身分查 A 的資料。
		own   func() (int, error)
		cross func() (int, error)
	}{
		{
			name: "ListForBuyer（端點 68）",
			own: func() (int, error) {
				got, err := docRepo.ListForBuyer(ctx, domain.BuyerDocumentQuery{BuyerID: a.BuyerID, Page: fullPage()})
				return len(got), err
			},
			cross: func() (int, error) {
				got, err := docRepo.ListForBuyer(ctx, domain.BuyerDocumentQuery{BuyerID: b.BuyerID, Page: fullPage()})
				// B 只看得到自己的那一份；下面用「不含 A 的 id」再驗一次。
				for _, d := range got {
					if d.ID == a.DocumentID {
						return 1, err
					}
				}
				return 0, err
			},
		},
		{
			name: "ListOrderDocumentsForBuyer（端點 8）",
			own: func() (int, error) {
				got, err := docRepo.ListOrderDocumentsForBuyer(ctx, a.OrderNo, a.BuyerID)
				return len(got), err
			},
			cross: func() (int, error) {
				got, err := docRepo.ListOrderDocumentsForBuyer(ctx, a.OrderNo, b.BuyerID)
				return len(got), err
			},
		},
		{
			name: "ListForSeller（端點 30）",
			own: func() (int, error) {
				got, err := docRepo.ListForSeller(ctx, domain.SellerDocumentQuery{StoreID: a.StoreID, Page: fullPage()})
				return len(got), err
			},
			cross: func() (int, error) {
				got, err := docRepo.ListForSeller(ctx, domain.SellerDocumentQuery{StoreID: b.StoreID, Page: fullPage()})
				for _, d := range got {
					if d.Document.ID == a.DocumentID {
						return 1, err
					}
				}
				return 0, err
			},
		},
		{
			name: "ListReceivedForSeller（端點 72）",
			own: func() (int, error) {
				got, err := docRepo.ListReceivedForSeller(ctx, domain.ReceivedDocumentQuery{StoreID: a.StoreID, Page: fullPage()})
				return len(got), err
			},
			cross: func() (int, error) {
				got, err := docRepo.ListReceivedForSeller(ctx, domain.ReceivedDocumentQuery{StoreID: b.StoreID, Page: fullPage()})
				for _, d := range got {
					if d.Document.ID == a.ReceivedID {
						return 1, err
					}
				}
				return 0, err
			},
		},
		{
			name: "ListOrderExternalIssuances（62R）",
			own: func() (int, error) {
				got, err := opRepo.ListOrderExternalIssuances(ctx, a.StoreID, a.OrderNo, externalStatuses, fullPage())
				return len(got), err
			},
			cross: func() (int, error) {
				got, err := opRepo.ListOrderExternalIssuances(ctx, b.StoreID, a.OrderNo, externalStatuses, fullPage())
				return len(got), err
			},
		},
		{
			name: "ListForBuyerOrder（端點 13）",
			own: func() (int, error) {
				got, err := complaintRepo.ListForBuyerOrder(ctx, domain.BuyerComplaintQuery{BuyerID: a.BuyerID, OrderNo: a.OrderNo, Page: fullPage()})
				return len(got), err
			},
			cross: func() (int, error) {
				got, err := complaintRepo.ListForBuyerOrder(ctx, domain.BuyerComplaintQuery{BuyerID: b.BuyerID, OrderNo: a.OrderNo, Page: fullPage()})
				return len(got), err
			},
		},
		{
			name: "ComplaintRepository.ListForSeller（端點 34）",
			own: func() (int, error) {
				got, err := complaintRepo.ListForSeller(ctx, domain.SellerComplaintQuery{StoreID: a.StoreID, Page: fullPage()})
				return len(got), err
			},
			cross: func() (int, error) {
				got, err := complaintRepo.ListForSeller(ctx, domain.SellerComplaintQuery{StoreID: b.StoreID, Page: fullPage()})
				for _, c := range got {
					if c.ID == a.ComplaintID {
						return 1, err
					}
				}
				return 0, err
			},
		},
	}

	for _, tc := range listCases {
		t.Run(tc.name+"/自己讀得到", func(t *testing.T) {
			n, err := tc.own()
			if err != nil {
				t.Fatalf("own: %v", err)
			}
			if n == 0 {
				t.Fatal("自己的資料讀不到 ⇒ 述詞過嚴（陽性斷言失敗，測試本身失去鑑別力）")
			}
		})
		t.Run(tc.name+"/讀不到別人的", func(t *testing.T) {
			n, err := tc.cross()
			if err != nil {
				t.Fatalf("cross: %v", err)
			}
			if n != 0 {
				t.Fatalf("越權：另一個擁有者讀到了 %d 筆不屬於他的資料（擁有者述詞沒進 WHERE）", n)
			}
		})
	}

	// singleCases：回單筆的查詢——越權必須與「不存在」不可區分，兩者都是
	// pgx.ErrNoRows 被 notFound() 翻成同一個業務碼。
	singleCases := []struct {
		name  string
		own   func() error
		cross func() error
	}{
		{
			name:  "GetForBuyerByID（端點 69）",
			own:   func() error { _, err := docRepo.GetForBuyerByID(ctx, a.DocumentID, a.BuyerID); return err },
			cross: func() error { _, err := docRepo.GetForBuyerByID(ctx, a.DocumentID, b.BuyerID); return err },
		},
		{
			name:  "GetForSeller（端點 31）",
			own:   func() error { _, err := docRepo.GetForSeller(ctx, a.DocumentID, a.StoreID); return err },
			cross: func() error { _, err := docRepo.GetForSeller(ctx, a.DocumentID, b.StoreID); return err },
		},
		{
			name:  "GetViewForSeller（端點 33）",
			own:   func() error { _, err := opRepo.GetViewForSeller(ctx, a.JobID, a.StoreID); return err },
			cross: func() error { _, err := opRepo.GetViewForSeller(ctx, a.JobID, b.StoreID); return err },
		},
		{
			name:  "ComplaintRepository.GetForSeller（端點 35）",
			own:   func() error { _, err := complaintRepo.GetForSeller(ctx, a.ComplaintID, a.StoreID); return err },
			cross: func() error { _, err := complaintRepo.GetForSeller(ctx, a.ComplaintID, b.StoreID); return err },
		},
		{
			name: "CorrectionRequestRepository.GetForBuyer（端點 12）",
			own: func() error {
				_, err := correctionRepo.GetForBuyer(ctx, a.CorrectionID, a.BuyerID, a.OrderNo)
				return err
			},
			cross: func() error {
				_, err := correctionRepo.GetForBuyer(ctx, a.CorrectionID, b.BuyerID, a.OrderNo)
				return err
			},
		},
		{
			name:  "GetByStoreID（稅務身分）",
			own:   func() error { _, err := taxRepo.GetByStoreID(ctx, a.StoreID); return err },
			cross: func() error { _, err := taxRepo.GetByStoreID(ctx, 999999999); return err },
		},
		{
			name:  "GetByStoreID（provider 帳號）",
			own:   func() error { _, err := providerRepo.GetByStoreID(ctx, a.StoreID, "AMEGO"); return err },
			cross: func() error { _, err := providerRepo.GetByStoreID(ctx, 999999999, "AMEGO"); return err },
		},
	}

	for _, tc := range singleCases {
		t.Run(tc.name+"/自己讀得到", func(t *testing.T) {
			if err := tc.own(); err != nil {
				t.Fatalf("自己的資料讀不到：%v（述詞過嚴 ⇒ 陰性斷言失去鑑別力）", err)
			}
		})
		t.Run(tc.name+"/讀不到別人的", func(t *testing.T) {
			err := tc.cross()
			if err == nil {
				t.Fatal("越權：另一個擁有者讀到了不屬於他的資料（擁有者述詞沒進 WHERE）")
			}
		})
	}

	// CorrectionRequest 的第三段述詞（order_no）要單獨驗：申請人對、買受人也對，
	// 但訂單號不匹配時仍必須走 not-found 出口。少驗這一段的話，只驗 buyer_id
	// 的測試會全綠而述詞其實漏了一段。
	t.Run("CorrectionRequestRepository.GetForBuyer/訂單號不匹配也走 not-found", func(t *testing.T) {
		if _, err := correctionRepo.GetForBuyer(ctx, a.CorrectionID, a.BuyerID, b.OrderNo); err == nil {
			t.Fatal("order_no 不匹配卻讀得到 ⇒ 三段述詞漏了訂單那一段")
		}
	})
}

// ---------------------------------------------------------------------------
// ② GetOrderInvoice 必須是 :many
//
// VOID_AND_REISSUE 之後同一張訂單有多份文件。:one 會**靜默取任意一列**
// （沒有 LIMIT、順序未定義），症狀是買家看到作廢前的舊發票而畫面完全正常。
// ---------------------------------------------------------------------------

func TestIntegration_ListOrderDocumentsForBuyer_ReturnsEveryDocumentOfTheOrder(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	issuerID := seedIssuerFor(ctx, t, pool, 710001)
	const buyerID int64 = 810001
	orderNo := "O-REISSUE-1"

	// 作廢重開鏈：attempt 1 已 SUPERSEDED、attempt 2 是現行那一張。
	first := seedOrderDocument(ctx, t, pool, issuerID, buyerID, orderNo, "SUPERSEDED", 1, time.Now().UTC().Add(-time.Hour))
	second := seedOrderDocument(ctx, t, pool, issuerID, buyerID, orderNo, "ISSUED", 2, time.Now().UTC())

	repo := NewDocumentRepository(pool, testKeyRing(t))
	got, err := repo.ListOrderDocumentsForBuyer(ctx, orderNo, buyerID)
	if err != nil {
		t.Fatalf("ListOrderDocumentsForBuyer: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("回了 %d 份文件，want 2 —— 作廢重開之後的訂單有兩份，取一份等於讓買家看到舊發票", len(got))
	}
	// 排序：實體軸遞減 ⇒ 最新一次開立排最前。
	if got[0].ID != second || got[1].ID != first {
		t.Fatalf("順序不對：got [%d %d], want [%d %d]（issue_attempt_seq DESC）",
			got[0].ID, got[1].ID, second, first)
	}

	// 既有的 :one 版本在同一組述詞下只會回一列——留這條斷言是為了讓「為什麼需要
	// 一支新的 :many」寫在測試裡，而不是只寫在註解裡。
	one, err := repo.GetForBuyer(ctx, orderNo, buyerID)
	if err != nil {
		t.Fatalf("GetForBuyer: %v", err)
	}
	if one.ID != first && one.ID != second {
		t.Fatalf("GetForBuyer 回了不屬於這張訂單的文件：%d", one.ID)
	}
}

// ---------------------------------------------------------------------------
// ③ count ↔ list 述詞一致性（**跨頁**）
//
// 只驗第一頁抓不到述詞漂移：漂移的症狀是「總數說有 N 筆、翻到第二頁是空的」。
// 必須把每一頁的實際筆數加起來，與 Count 對帳。
// ---------------------------------------------------------------------------

// walkPages 逐頁取完，回實際取到的總筆數與看到的 id 集合（用來驗沒有重複列）。
func walkPages(t *testing.T, total int64, pageSize int32, fetch func(page domain.Page) ([]int64, error)) int {
	t.Helper()
	seen := map[int64]bool{}
	got := 0
	// 多跑一頁：如果 count 少報，最後一頁會多出資料，這裡才抓得到。
	pages := int(total/int64(pageSize)) + 2
	for i := 0; i < pages; i++ {
		ids, err := fetch(domain.Page{Limit: pageSize, Offset: int32(i) * pageSize})
		if err != nil {
			t.Fatalf("fetch page %d: %v", i, err)
		}
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("第 %d 頁出現重複列 id=%d ⇒ 排序鍵不穩定（缺第二鍵）", i, id)
			}
			seen[id] = true
		}
		got += len(ids)
	}
	return got
}

func TestIntegration_ReadViews_CountMatchesPagedList(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	issuerID := seedIssuerFor(ctx, t, pool, 720001)
	const buyerID int64 = 820001

	// 5 份文件：3 份 ISSUED、2 份 PENDING。同一張訂單只能有一份同 attempt_seq，
	// 所以用五張不同的訂單號。
	const seedCount = 5
	issuedCount := 0
	for i := 0; i < seedCount; i++ {
		status := "ISSUED"
		if i >= 3 {
			status = "PENDING"
		} else {
			issuedCount++
		}
		seedOrderDocument(ctx, t, pool, issuerID, buyerID,
			fmt.Sprintf("O-PAGE-%d", i), status, 1, time.Now().UTC().Add(time.Duration(-i)*time.Minute))
	}

	docRepo := NewDocumentRepository(pool, testKeyRing(t))

	t.Run("買家列表/不篩", func(t *testing.T) {
		q := domain.BuyerDocumentQuery{BuyerID: buyerID}
		total, err := docRepo.CountForBuyer(ctx, q)
		if err != nil {
			t.Fatalf("CountForBuyer: %v", err)
		}
		if total != seedCount {
			t.Fatalf("Count = %d, want %d", total, seedCount)
		}
		got := walkPages(t, total, 2, func(page domain.Page) ([]int64, error) {
			q := q
			q.Page = page
			rows, err := docRepo.ListForBuyer(ctx, q)
			ids := make([]int64, 0, len(rows))
			for _, r := range rows {
				ids = append(ids, r.ID)
			}
			return ids, err
		})
		if int64(got) != total {
			t.Fatalf("跨頁實際取到 %d 筆，Count 說有 %d 筆 ⇒ 兩支述詞漂移", got, total)
		}
	})

	t.Run("買家列表/帶 status 篩選", func(t *testing.T) {
		q := domain.BuyerDocumentQuery{BuyerID: buyerID, Status: "ISSUED"}
		total, err := docRepo.CountForBuyer(ctx, q)
		if err != nil {
			t.Fatalf("CountForBuyer: %v", err)
		}
		if total != int64(issuedCount) {
			t.Fatalf("Count = %d, want %d", total, issuedCount)
		}
		got := walkPages(t, total, 2, func(page domain.Page) ([]int64, error) {
			q := q
			q.Page = page
			rows, err := docRepo.ListForBuyer(ctx, q)
			ids := make([]int64, 0, len(rows))
			for _, r := range rows {
				ids = append(ids, r.ID)
			}
			return ids, err
		})
		if int64(got) != total {
			t.Fatalf("篩選路徑的跨頁筆數 %d ≠ Count %d ⇒ 篩選述詞只寫進了其中一支", got, total)
		}
	})

	t.Run("賣家列表/帶 abnormal_only", func(t *testing.T) {
		q := domain.SellerDocumentQuery{
			StoreID:          720001,
			AbnormalOnly:     true,
			AbnormalStatuses: []string{"PENDING"}, // 用 PENDING 當「異常集合」以取得非空樣本
		}
		total, err := docRepo.CountForSeller(ctx, q)
		if err != nil {
			t.Fatalf("CountForSeller: %v", err)
		}
		if total != int64(seedCount-issuedCount) {
			t.Fatalf("Count = %d, want %d", total, seedCount-issuedCount)
		}
		got := walkPages(t, total, 1, func(page domain.Page) ([]int64, error) {
			q := q
			q.Page = page
			rows, err := docRepo.ListForSeller(ctx, q)
			ids := make([]int64, 0, len(rows))
			for _, r := range rows {
				ids = append(ids, r.Document.ID)
			}
			return ids, err
		})
		if int64(got) != total {
			t.Fatalf("賣家列表跨頁筆數 %d ≠ Count %d", got, total)
		}
	})
}

// ---------------------------------------------------------------------------
// ④-b 「這件事還要不要人處理」在三個出口必須是同一份定義（#3045）
//
// A01 KPI 的 definition 字串逐字寫著「＝ A05 NEEDS_MANUAL 分頁的 job 數」，而 #3014
// 的否決條件只加在 KPI ⇒ dev 實測同一時點 KPI=0、priorityJobs=11、A05 tab=8。
// 三個數字互相矛盾，管理員無從判斷該不該動手。
// ---------------------------------------------------------------------------

func TestIntegration_ResolvedDocumentsExcludedFromAllThreeNeedsManualOutlets(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	issuerID := seedIssuerFor(ctx, t, pool, 730051)
	const buyerID int64 = 830051
	resolved := []string{"VOIDED", "SUPERSEDED", "CANCELLED_BEFORE_ISSUE",
		"NOT_APPLICABLE", "RESOLVED_EXTERNALLY"}

	// ① 已收束：原票被作廢，它底下那筆 FAILED_PERMANENT 的 ISSUE 已經沒有下一步。
	voidedDoc := seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-RESOLVED-1", "ISSUED", 1, time.Now().UTC())
	// voided_at 與 status='VOIDED' 同進同出（chk_invdoc_voided_pair）。
	mustExec(ctx, t, pool,
		`UPDATE invoice_documents SET status = 'VOIDED', voided_at = now() WHERE id = $1`, voidedDoc)
	voidedJob := seedOperation(ctx, t, pool, voidedDoc, "FAILED_PERMANENT")

	// ② 陽性對照：同樣 FAILED_PERMANENT，但 document 停在 ISSUE_FAILED ＝ 還等著 ADM
	//    安全重試 ⇒ **必須留下**。少了它，把整個清單關掉也會讓①那條斷言通過。
	openDoc := seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-RESOLVED-2", "ISSUED", 1, time.Now().UTC())
	mustExec(ctx, t, pool, `UPDATE invoice_documents SET status = 'ISSUE_FAILED' WHERE id = $1`, openDoc)
	openJob := seedOperation(ctx, t, pool, openDoc, "FAILED_PERMANENT")

	repo := NewOperationRepository(pool)
	has := func(rows []domain.OperationView, id int64) bool {
		for _, v := range rows {
			if v.Operation.ID == id {
				return true
			}
		}
		return false
	}

	t.Run("priorityJobs（端點 37）排除已收束、保留待處理", func(t *testing.T) {
		q := domain.PriorityOperationQuery{
			OpenStatuses:             []string{"FAILED_PERMANENT"},
			ManualDocumentStatuses:   []string{"AWAITING_MANUAL"},
			ResolvedDocumentStatuses: resolved,
			Limit:                    200,
		}
		rows, err := repo.ListPriorityViewsForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("ListPriorityViewsForAdmin: %v", err)
		}
		if has(rows, voidedJob) {
			t.Fatal("document 已 VOIDED 的 job 仍在 priorityJobs ⇒ 否決條件沒生效")
		}
		if !has(rows, openJob) {
			t.Fatal("ISSUE_FAILED 的 job 被一起排除了 ⇒ 這是把清單關掉，不是修好")
		}

		// 鑑別力：不傳否決集合 ⇒ 那一筆必須回來。沒有這一段，一個恆假的述詞也會
		// 讓上面通過。
		q.ResolvedDocumentStatuses = nil
		rows, err = repo.ListPriorityViewsForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("ListPriorityViewsForAdmin: %v", err)
		}
		if !has(rows, voidedJob) {
			t.Fatal("拿掉否決集合之後仍看不到 ⇒ 上面那條斷言沒有鑑別力")
		}
	})

	t.Run("A05 NEEDS_MANUAL（端點 50）與 KPI 同口徑", func(t *testing.T) {
		q := domain.AdminOperationQuery{
			StatusFilters:            []string{"FAILED_PERMANENT"},
			IncludeManualDocuments:   true,
			ManualDocumentStatuses:   []string{"AWAITING_MANUAL"},
			ExcludeResolvedDocuments: true,
			ResolvedDocumentStatuses: resolved,
			Page:                     fullPage(),
		}
		rows, err := repo.ListViewsForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("ListViewsForAdmin: %v", err)
		}
		if has(rows, voidedJob) {
			t.Fatal("document 已 VOIDED 的 job 仍在 NEEDS_MANUAL 分頁")
		}
		if !has(rows, openJob) {
			t.Fatal("ISSUE_FAILED 的 job 被一起排除了")
		}

		// Count 與 List 是兩支述詞逐字相同的查詢，改一支沒改另一支 ⇒ 分頁器與筆數
		// 打架。這裡一起驗。
		total, err := repo.CountViewsForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("CountViewsForAdmin: %v", err)
		}
		if int(total) != len(rows) {
			t.Fatalf("Count = %d，List 回 %d 筆 ⇒ 兩支述詞漂移", total, len(rows))
		}

		// 其餘分頁不得被波及：SUCCEEDED 之類的分頁本來就該看得到已作廢的票。
		q.ExcludeResolvedDocuments = false
		q.IncludeManualDocuments = false
		rows, err = repo.ListViewsForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("ListViewsForAdmin: %v", err)
		}
		if !has(rows, voidedJob) {
			t.Fatal("關掉 exclude 之後仍看不到 ⇒ 排除被套到所有分頁上了")
		}
	})
}

// ---------------------------------------------------------------------------
// ④ 端點 50 的 NEEDS_MANUAL 是**聯集**：job 狀態 OR document.status='AWAITING_MANUAL'
//
// 只寫前半的症狀是 fail-closed 落點的案件在六個分頁裡都看不到（不是報錯，是消失）。
// ---------------------------------------------------------------------------

func TestIntegration_ListInvoiceOperationsForAdmin_NeedsManualIsUnionWithDocumentStatus(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	issuerID := seedIssuerFor(ctx, t, pool, 730001)
	const buyerID int64 = 830001

	// 這份文件落在 AWAITING_MANUAL，但它的 job 還停在 READY（不在 NEEDS_MANUAL 的
	// job 狀態集合裡）⇒ 只有聯集述詞才看得到它。
	doc := seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-MANUAL-1", "ISSUED", 1, time.Now().UTC())
	mustExec(ctx, t, pool,
		`UPDATE invoice_documents SET status = 'AWAITING_MANUAL', manual_reason_code = 'PLATFORM_FUNDED_DISCOUNT'
		 WHERE id = $1`, doc)
	job := seedOperation(ctx, t, pool, doc, "READY")

	repo := NewOperationRepository(pool)
	q := domain.AdminOperationQuery{
		StatusFilters:          []string{"WAITING_CONFIRMATION", "FAILED_UNKNOWN"},
		IncludeManualDocuments: true,
		ManualDocumentStatuses: []string{"AWAITING_MANUAL"},
		Page:                   fullPage(),
	}
	got, err := repo.ListViewsForAdmin(ctx, q)
	if err != nil {
		t.Fatalf("ListViewsForAdmin: %v", err)
	}
	found := false
	for _, v := range got {
		if v.Operation.ID == job {
			found = true
			if v.DocumentManualReasonCode != "PLATFORM_FUNDED_DISCOUNT" {
				t.Fatalf("manualReasonCode 沒帶上來：%q", v.DocumentManualReasonCode)
			}
		}
	}
	if !found {
		t.Fatal("document.status='AWAITING_MANUAL' 的 job 沒有出現在 NEEDS_MANUAL 分頁 ⇒ 述詞不是聯集")
	}

	// 關掉聯集那一半 ⇒ 必須看不到它。這條是上一條的鑑別力證明：
	// 沒有它的話，一個「恆真」的述詞也會讓上面那條通過。
	q.IncludeManualDocuments = false
	got, err = repo.ListViewsForAdmin(ctx, q)
	if err != nil {
		t.Fatalf("ListViewsForAdmin: %v", err)
	}
	for _, v := range got {
		if v.Operation.ID == job {
			t.Fatal("關掉聯集之後仍看得到 ⇒ 述詞恆真，上一條斷言沒有鑑別力")
		}
	}

	count, err := repo.CountViewsForAdmin(ctx, domain.AdminOperationQuery{
		StatusFilters:          []string{"WAITING_CONFIRMATION", "FAILED_UNKNOWN"},
		IncludeManualDocuments: true,
		ManualDocumentStatuses: []string{"AWAITING_MANUAL"},
	})
	if err != nil {
		t.Fatalf("CountViewsForAdmin: %v", err)
	}
	if count < 1 {
		t.Fatalf("Count = %d，但 List 看得到 ⇒ 兩支述詞漂移", count)
	}
}

// ---------------------------------------------------------------------------
// ⑤ 端點 58 的 OVERDUE 是**橫切視圖**：status='OVERDUE' OR（未結案且 sla_due_at 已過）
//
// 只寫前半會讓「掃描還沒跑到」「賣家逾期後才回覆」「已升級到 ADM」三類逾期件
// 從 OVERDUE 分頁消失，而它們正是 ADM 最該看到的。
// ---------------------------------------------------------------------------

func TestIntegration_ListInvoiceComplaintsForAdmin_OverdueIsCrossCutting(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	issuerID := seedIssuerFor(ctx, t, pool, 740001)
	const buyerID int64 = 840001
	now := time.Now().UTC()

	// 期限已過但狀態還是 OPEN（INV.dl 還沒掃到）——單純述詞看不到它。
	docA := seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-OD-1", "ISSUED", 1, now)
	lateOpen := seedComplaint(ctx, t, pool, docA, issuerID, buyerID, "O-OD-1", "OPEN", now.Add(-time.Hour))
	// 期限未到的 OPEN——不論哪種述詞都不該出現在 OVERDUE 分頁。
	docB := seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-OD-2", "ISSUED", 1, now)
	onTime := seedComplaint(ctx, t, pool, docB, issuerID, buyerID, "O-OD-2", "OPEN", now.Add(48*time.Hour))

	repo := NewComplaintRepository(pool)
	q := domain.AdminComplaintQuery{
		StatusFilters:        []string{"OVERDUE"},
		IncludeDueOverdue:    true,
		DueOverdueStatuses:   []string{"OPEN", "SELLER_RESPONDED", "ESCALATED_TO_ADM"},
		Now:                  now,
		ViolationStatus:      "VIOLATION_CONFIRMED",
		ViolationWindowStart: now.AddDate(0, 0, -90),
		Page:                 fullPage(),
	}
	got, err := repo.ListForAdmin(ctx, q)
	if err != nil {
		t.Fatalf("ListForAdmin: %v", err)
	}
	sawLate, sawOnTime := false, false
	for _, item := range got {
		switch item.Complaint.ID {
		case lateOpen:
			sawLate = true
		case onTime:
			sawOnTime = true
		}
	}
	if !sawLate {
		t.Fatal("期限已過但狀態仍是 OPEN 的申訴沒出現在 OVERDUE 分頁 ⇒ 述詞不是 OR")
	}
	if sawOnTime {
		t.Fatal("期限未到的申訴出現在 OVERDUE 分頁 ⇒ 述詞恆真，上一條斷言沒有鑑別力")
	}

	count, err := repo.CountForAdmin(ctx, q)
	if err != nil {
		t.Fatalf("CountForAdmin: %v", err)
	}
	if int(count) != len(got) {
		t.Fatalf("Count = %d，List 回 %d 筆 ⇒ 兩支述詞漂移（OR 那一段只寫進了其中一支）", count, len(got))
	}
}

// ---------------------------------------------------------------------------
// ⑥ 端點 42 的列表必須看得見「查無 provider account 列」的賣家（NOT_STARTED）
//
// 從 invoice_provider_accounts 起手（INNER JOIN）會讓上線當天的全體賣家整批消失。
// ---------------------------------------------------------------------------

func TestIntegration_ListProviderAccountsForAdmin_IncludesIssuersWithoutAccountRow(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	withAccount := seedIssuerFor(ctx, t, pool, 750001)
	seedProviderAccountRow(ctx, t, pool, withAccount)
	without := seedIssuerFor(ctx, t, pool, 750002)

	repo := NewProviderAccountRepository(pool, testKeyRing(t))
	q := domain.AdminProviderAccountQuery{Provider: "AMEGO", Page: fullPage()}
	got, err := repo.ListForAdmin(ctx, q)
	if err != nil {
		t.Fatalf("ListForAdmin: %v", err)
	}
	sawWith, sawWithout := false, false
	for _, item := range got {
		switch item.Issuer.ID {
		case withAccount:
			sawWith = true
			if item.Account == nil {
				t.Fatal("已建 provider account 的 issuer 回了 nil Account")
			}
		case without:
			sawWithout = true
			if item.Account != nil {
				t.Fatal("沒有 provider account 列的 issuer 卻回了非 nil Account")
			}
		}
	}
	if !sawWith || !sawWithout {
		t.Fatalf("列表漏了 issuer：sawWith=%v sawWithout=%v（沒有 account 列的那一個就是 NOT_STARTED，上線當天的全體）",
			sawWith, sawWithout)
	}

	count, err := repo.CountForAdmin(ctx, q)
	if err != nil {
		t.Fatalf("CountForAdmin: %v", err)
	}
	if int(count) != len(got) {
		t.Fatalf("Count = %d，List 回 %d 筆 ⇒ Count 側的 LEFT JOIN 退化成 INNER JOIN", count, len(got))
	}
}

// ---------------------------------------------------------------------------
// ⑦ 進項列表的 recipient_note 兩個輸入必須有值
// ---------------------------------------------------------------------------

func TestIntegration_ListSellerReceivedInvoiceDocuments_ExposesNoteInputs(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	const storeID int64 = 760001
	issuerID := seedIssuerFor(ctx, t, pool, storeID)
	seedReceivedDocument(ctx, t, pool, issuerID, storeID, "2026 年 8 月平台服務費", 3)

	repo := NewDocumentRepository(pool, testKeyRing(t))
	got, err := repo.ListReceivedForSeller(ctx, domain.ReceivedDocumentQuery{StoreID: storeID, Page: fullPage()})
	if err != nil {
		t.Fatalf("ListReceivedForSeller: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("回了 %d 筆，want 1", len(got))
	}
	if got[0].FirstLineDescription != "2026 年 8 月平台服務費" {
		t.Fatalf("firstLineDescription = %q ⇒ 不是 line_seq 最小那一行", got[0].FirstLineDescription)
	}
	if got[0].LineCount != 3 {
		t.Fatalf("lineCount = %d, want 3（「等 N 項」的 N 算不出來）", got[0].LineCount)
	}
}

// ---------------------------------------------------------------------------
// ⑧ 總覽計數：口徑必須與列表一致
// ---------------------------------------------------------------------------

func TestIntegration_OverviewCounts(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	const storeID int64 = 770001
	const buyerID int64 = 870001
	issuerID := seedIssuerFor(ctx, t, pool, storeID)
	now := time.Now().UTC()

	seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-OV-1", "PENDING", 1, now)
	seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-OV-2", "ISSUED", 1, now)
	docC := seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-OV-3", "ISSUED", 1, now)
	seedComplaint(ctx, t, pool, docC, issuerID, buyerID, "O-OV-3", "OPEN", now.Add(-time.Hour))

	repo := NewOverviewRepository(pool)
	got, err := repo.SellerCounts(ctx, domain.SellerOverviewCountsQuery{
		IssuerID:                 issuerID,
		PendingDocumentStatuses:  []string{"PENDING", "ISSUING", "ISSUE_FAILED", "AWAITING_MANUAL"},
		OpenComplaintStatuses:    []string{"OPEN", "OVERDUE", "SELLER_RESPONDED", "ESCALATED_TO_ADM"},
		OverdueComplaintStatuses: []string{"OVERDUE"},
		Now:                      now,
	})
	if err != nil {
		t.Fatalf("SellerCounts: %v", err)
	}
	if got.PendingDocumentCount != 1 {
		t.Fatalf("pendingDocumentCount = %d, want 1", got.PendingDocumentCount)
	}
	if got.OpenComplaintCount != 1 {
		t.Fatalf("openComplaintCount = %d, want 1", got.OpenComplaintCount)
	}
	// 狀態還是 OPEN 但 sla_due_at 已過 ⇒ 必須被算成逾期（口徑是 OR）。
	if got.OverdueComplaintCount != 1 {
		t.Fatalf("overdueComplaintCount = %d, want 1 ⇒ 口徑寫成單純 status='OVERDUE'，"+
			"賣家在最該行動的那幾小時會看到 0", got.OverdueComplaintCount)
	}

	admin, err := repo.AdminCounts(ctx, domain.AdminOverviewCountsQuery{
		TaxReviewQueueStatuses:   []string{"SUBMITTED", "UNDER_REVIEW", "SUPPLEMENT_REQUIRED", "SELF_ISSUE_PENDING"},
		NearDeadlineBefore:       now.Add(7 * 24 * time.Hour),
		ProviderErrorStatuses:    []string{"INVALID_CREDENTIAL", "PROVIDER_REJECTED", "DEGRADED"},
		PendingJobStatuses:       []string{"CREATED", "AWAITING_TRIGGER", "READY", "HELD", "RETRY_SCHEDULED", "SENDING"},
		NeedsManualJobStatuses:   []string{"WAITING_CONFIRMATION", "FAILED_UNKNOWN"},
		ManualDocumentStatuses:   []string{"AWAITING_MANUAL"},
		OpenComplaintStatuses:    []string{"OPEN", "OVERDUE", "SELLER_RESPONDED", "ESCALATED_TO_ADM"},
		OverdueComplaintStatuses: []string{"OVERDUE"},
		Now:                      now,
	})
	if err != nil {
		t.Fatalf("AdminCounts: %v", err)
	}
	if admin.OpenComplaintCount < 1 {
		t.Fatalf("ADM openComplaintCount = %d, want >= 1", admin.OpenComplaintCount)
	}
}

// ---------------------------------------------------------------------------
// ⑨ 其餘 ADM 查詢的執行證明 ＋ count ↔ list 對帳
//
// 這幾支不帶擁有者述詞（ADM 看全平台），所以 Tier A 的陰性斷言不適用；它們真正的
// 風險是**執行期**：參數型別對不上、多型指標的 LEFT JOIN 誤配、cardinality(NULL)。
// 這些在 `go build` 與 sqlc 生成都是綠的，只有真的送進 PostgreSQL 才會現形。
// ---------------------------------------------------------------------------

func TestIntegration_ReadViews_AdminQueriesExecuteAndAgreeWithCount(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	const storeID int64 = 780001
	const buyerID int64 = 880001
	const actorID int64 = 990001
	issuerID := seedIssuerFor(ctx, t, pool, storeID)
	now := time.Now().UTC()

	doc := seedOrderDocument(ctx, t, pool, issuerID, buyerID, "O-ADM-1", "ISSUED", 1, now)
	job := seedOperation(ctx, t, pool, doc, "READY")
	seedTaxProfile(ctx, t, pool, issuerID, "SUBMITTED")

	// 稽核列：三種 resource_type 各一，用來驗五條 LEFT JOIN 的多型解析。
	// resource_id 刻意用真實主鍵，誤配的話 store_id filter 會撈到不該撈的列。
	for _, rt := range []struct {
		kind string
		id   int64
	}{{"DOCUMENT", doc}, {"JOB", job}, {"PROVIDER_ACCOUNT", seedProviderAccountRow(ctx, t, pool, issuerID)}} {
		mustExec(ctx, t, pool,
			`INSERT INTO invoice_admin_audit_logs (
			   actor_account_id, action, resource_type, resource_id, reason_code, reason_detail)
			 VALUES ($1, 'INVOICE_SEED', $2, $3, 'SEED', 'seed')`,
			actorID, rt.kind, rt.id)
	}

	resourceTypes := domain.AuditResourceTypes{
		TaxProfile:      "TAX_PROFILE",
		ProviderAccount: "PROVIDER_ACCOUNT",
		Job:             "JOB",
		Document:        "DOCUMENT",
		Complaint:       "COMPLAINT",
	}

	docRepo := NewDocumentRepository(pool, testKeyRing(t))
	opRepo := NewOperationRepository(pool)
	taxRepo := NewSellerTaxProfileRepository(pool)
	auditRepo := NewAuditRepository(pool)

	t.Run("ListInvoiceDocumentsForAdmin（端點 47）", func(t *testing.T) {
		q := domain.AdminDocumentQuery{StoreID: &[]int64{storeID}[0], Page: fullPage()}
		rows, err := docRepo.ListForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("ListForAdmin: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("store_id 篩選撈不到自己的文件")
		}
		total, err := docRepo.CountForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("CountForAdmin: %v", err)
		}
		if int(total) != len(rows) {
			t.Fatalf("Count = %d，List 回 %d 筆 ⇒ 述詞漂移", total, len(rows))
		}
		// StoreID = nil ＝ 全平台，必須是前者的超集。
		all, err := docRepo.CountForAdmin(ctx, domain.AdminDocumentQuery{})
		if err != nil {
			t.Fatalf("CountForAdmin(全平台): %v", err)
		}
		if all < total {
			t.Fatalf("全平台計數 %d < 單一賣家計數 %d ⇒ has_store_id 旗標接反了", all, total)
		}
	})

	t.Run("ListPriorityInvoiceOperationsForAdmin（端點 37）", func(t *testing.T) {
		rows, err := opRepo.ListPriorityViewsForAdmin(ctx, domain.PriorityOperationQuery{
			OpenStatuses:           []string{"READY", "RETRY_SCHEDULED"},
			ManualDocumentStatuses: []string{"AWAITING_MANUAL"},
			Limit:                  10,
		})
		if err != nil {
			t.Fatalf("ListPriorityViewsForAdmin: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("READY 的 job 沒被列進 priorityJobs")
		}
		if rows[0].DocumentSourceRef == "" {
			t.Fatal("order_no 沒從 document join 帶上來（invoice_operations 本身沒有這個欄位）")
		}
	})

	t.Run("GetViewForAdmin（端點 51）", func(t *testing.T) {
		got, err := opRepo.GetViewForAdmin(ctx, job)
		if err != nil {
			t.Fatalf("GetViewForAdmin: %v", err)
		}
		if got.IssuerRefID != storeID {
			t.Fatalf("issuerRefID = %d, want %d", got.IssuerRefID, storeID)
		}
	})

	t.Run("ListRecentForIssuer（端點 16）", func(t *testing.T) {
		rows, err := docRepo.ListRecentForIssuer(ctx, issuerID, 5)
		if err != nil {
			t.Fatalf("ListRecentForIssuer: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("最近文件區塊撈不到資料")
		}
	})

	t.Run("ListTaxReviewsForAdmin（端點 38）", func(t *testing.T) {
		q := domain.AdminTaxReviewQuery{
			StatusFilters:         []string{"SUBMITTED", "UNDER_REVIEW", "SUPPLEMENT_REQUIRED", "SELF_ISSUE_PENDING"},
			OpenComplaintStatuses: []string{"OPEN", "OVERDUE"},
			Page:                  fullPage(),
		}
		rows, err := taxRepo.ListReviewsForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("ListReviewsForAdmin: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("SUBMITTED 的稅務身分沒進 A02 佇列")
		}
		if rows[0].StoreID != storeID {
			t.Fatalf("storeID = %d, want %d（seller_tax_profiles 上沒有這個欄位，要 join）", rows[0].StoreID, storeID)
		}
		total, err := taxRepo.CountReviewsForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("CountReviewsForAdmin: %v", err)
		}
		if int(total) != len(rows) {
			t.Fatalf("Count = %d，List 回 %d 筆", total, len(rows))
		}
		// NEAR_DEADLINE 是**交集**視圖：加上旗標之後只會變少，不會變多。
		q.NearDeadlineOnly = true
		q.NearDeadlineBefore = now // 種子的 pending_valid_until 是 now+30d ⇒ 應該被濾掉
		narrowed, err := taxRepo.CountReviewsForAdmin(ctx, q)
		if err != nil {
			t.Fatalf("CountReviewsForAdmin(near_deadline): %v", err)
		}
		if narrowed >= total {
			t.Fatalf("NEAR_DEADLINE 沒有收斂（%d >= %d）⇒ 旗標沒有生效，六個 filter 會全部等價", narrowed, total)
		}
	})

	t.Run("ListInvoiceAuditLogsForAdmin（端點 61）", func(t *testing.T) {
		base := domain.AdminAuditLogQuery{ResourceTypes: resourceTypes, Page: fullPage()}

		rows, err := auditRepo.ListAdminAuditLogsForAdmin(ctx, base)
		if err != nil {
			t.Fatalf("ListAdminAuditLogsForAdmin: %v", err)
		}
		if len(rows) < 3 {
			t.Fatalf("回了 %d 列，want >= 3", len(rows))
		}
		total, err := auditRepo.CountAdminAuditLogsForAdmin(ctx, base)
		if err != nil {
			t.Fatalf("CountAdminAuditLogsForAdmin: %v", err)
		}
		if int(total) != len(rows) {
			t.Fatalf("Count = %d，List 回 %d 筆 ⇒ 五條 LEFT JOIN 有一條只寫進了其中一支", total, len(rows))
		}

		// store_id filter 必須對三種 resource_type 都成立：DOCUMENT 直接解析、
		// JOB 經 operation→document、PROVIDER_ACCOUNT 直接解析 issuer。
		byStore := base
		byStore.StoreID = &[]int64{storeID}[0]
		storeRows, err := auditRepo.ListAdminAuditLogsForAdmin(ctx, byStore)
		if err != nil {
			t.Fatalf("ListAdminAuditLogsForAdmin(store): %v", err)
		}
		if len(storeRows) != 3 {
			t.Fatalf("store_id filter 撈到 %d 列，want 3（三種 resource_type 各一）"+
				" ⇒ 多型指標的解析漏了其中一條 join", len(storeRows))
		}
		// 鑑別力：換一個不存在的 store_id 必須回 0。少了這條，一個「恆真」的
		// filter 也會讓上面那條通過（本測試的樣本剛好就是 3 列）。
		otherStore := base
		otherStore.StoreID = &[]int64{storeID + 999}[0]
		otherRows, err := auditRepo.ListAdminAuditLogsForAdmin(ctx, otherStore)
		if err != nil {
			t.Fatalf("ListAdminAuditLogsForAdmin(other store): %v", err)
		}
		if len(otherRows) != 0 {
			t.Fatalf("不存在的 store_id 撈到 %d 列 ⇒ store_id filter 是 no-op", len(otherRows))
		}

		// order_no 只解析得到 DOCUMENT 與 JOB 兩條（provider account 不掛訂單）。
		byOrder := base
		byOrder.OrderNo = "O-ADM-1"
		orderRows, err := auditRepo.ListAdminAuditLogsForAdmin(ctx, byOrder)
		if err != nil {
			t.Fatalf("ListAdminAuditLogsForAdmin(order): %v", err)
		}
		if len(orderRows) != 2 {
			t.Fatalf("order_no filter 撈到 %d 列，want 2（DOCUMENT ＋ JOB）", len(orderRows))
		}

		// job_id 只解析得到 JOB 那一條。
		byJob := base
		byJob.JobID = &[]int64{job}[0]
		jobRows, err := auditRepo.ListAdminAuditLogsForAdmin(ctx, byJob)
		if err != nil {
			t.Fatalf("ListAdminAuditLogsForAdmin(job): %v", err)
		}
		if len(jobRows) != 1 {
			t.Fatalf("job_id filter 撈到 %d 列，want 1 ⇒ resource_type 沒有夾住那條 LEFT JOIN，"+
				"bigserial 與 TSID 混用之下會誤配到別張表的同號 id", len(jobRows))
		}
		if jobRows[0].ResolvedOperationType != "ISSUE" {
			t.Fatalf("operationType = %q, want ISSUE", jobRows[0].ResolvedOperationType)
		}
	})
}
