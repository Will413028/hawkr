//go:build integration

// 本檔跑真的 SQL，守兩件單元測試驗不到的事：
//
//	① **端點 36 的三條述詞（allowed-from ＋ 擁有者 ＋ 版本）真的在 SQL 裡**。
//	   fake 的 casFails 旗標是測試自己設的，它證明「呼叫端把 rows=0 當守衛不成立」，
//	   證明不了述詞本身。
//	② **FailOpenForIssuer 這句 `UPDATE ... FROM` 真的送得到 PostgreSQL**。
//	   它的 SET 右手邊同時解析兩張表，而 invoice_operations 與 invoice_documents
//	   都有 version 欄 —— 寫成 `version = version + 1` 會在執行時 42702
//	   （column reference "version" is ambiguous）。sqlc generate 過、go build 過、
//	   單元測試過，只有真的送到 PostgreSQL 才會現形（該 query 的註解逐字寫了這件事）。
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
// 端點 36：K02／K03 的述詞
// ---------------------------------------------------------------------------

const sellerStoreA int64 = 2685914682

func TestIntegration_Complaint_SellerRespondGuards(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewComplaintRepository(pool)
	issuerID := seedIssuerFor(ctx, t, pool, sellerStoreA)

	// newCase 直接 INSERT 指定狀態，並補上該狀態的**配對欄位**。
	//
	// 🔴 三條雙向等式的 CHECK 讓「只寫 status」在 DB 上直接 23514：
	//   chk_invcomp_overdue_pair  ：OVERDUE ⇒ overdue_at 非空
	//   chk_invcomp_resolved_pair ：RESOLVED／VIOLATION_CONFIRMED ⇒ resolved_at 非空
	// 這正是為什麼賣家回覆那兩句 UPDATE 必須各自把配對欄一起寫（或一起不碰）。
	newCase := func(status string) int64 {
		orderNo := fmt.Sprintf("O%d", nextID()%1_000_000_000_000)
		docID := seedOrderDocument(ctx, t, pool, issuerID, iBuyerA, orderNo, "ISSUED", 1, time.Now().UTC())
		id := nextID()
		base := time.Now().UTC()
		var overdueAt, resolvedAt any
		switch status {
		case "OVERDUE":
			overdueAt = base.Add(-time.Hour)
		case "RESOLVED", "VIOLATION_CONFIRMED":
			resolvedAt = base.Add(-time.Hour)
		}
		mustExec(ctx, t, pool,
			`INSERT INTO invoice_complaints (
			   id, document_id, issuer_id, order_no, complainant_buyer_id,
			   reason_code, status, sla_due_at, overdue_at, resolved_at)
			 VALUES ($1, $2, $3, $4, $5, 'NOT_RECEIVED', $6, $7, $8, $9)`,
			id, docID, issuerID, orderNo, iBuyerA, status, base.Add(48*time.Hour), overdueAt, resolvedAt)
		return id
	}
	now := time.Now().UTC()
	due := now.Add(72 * time.Hour)

	// K02 的 allowed-from ＝ OPEN／OVERDUE。
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{"OPEN", true},
		{"OVERDUE", true},
		{"SELLER_RESPONDED", false},
		{"ESCALATED_TO_ADM", false},
		{"RESOLVED", false},
		{"WITHDRAWN", false},
	} {
		t.Run("K02/"+tc.status, func(t *testing.T) {
			id := newCase(tc.status)
			ok, err := repo.MarkSellerResponded(ctx, id, sellerStoreA, 0, due, now)
			if err != nil {
				t.Fatalf("MarkSellerResponded 不該回錯：%v", err)
			}
			if ok != tc.want {
				t.Fatalf("status=%s: ok = %v, want %v", tc.status, ok, tc.want)
			}
		})
	}

	// 🔴 K03 的 allowed-from **只有 OPEN**（不含 OVERDUE）：與 K02 分成兩支正是為了
	// 這個差別。合成一句 2 值 allowed-from ⇒「逾期之後才請求平台協助」會靜默成功。
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{"OPEN", true},
		{"OVERDUE", false},
	} {
		t.Run("K03/"+tc.status, func(t *testing.T) {
			id := newCase(tc.status)
			ok, err := repo.EscalateBySeller(ctx, id, sellerStoreA, 0, now)
			if err != nil {
				t.Fatalf("EscalateBySeller 不該回錯：%v", err)
			}
			if ok != tc.want {
				t.Fatalf("status=%s: ok = %v, want %v", tc.status, ok, tc.want)
			}
		})
	}

	// 🔴 擁有者述詞：別家店拿同一個 complaint_id 一律 rows=0。
	// 這一條是 Tier A 在資料層的落點；放在呼叫端的 if 上，兩句之間就有窗口。
	t.Run("擁有者述詞", func(t *testing.T) {
		id := newCase("OPEN")
		ok, err := repo.MarkSellerResponded(ctx, id, 2632049831, 0, due, now)
		if err != nil {
			t.Fatalf("不該回錯：%v", err)
		}
		if ok {
			t.Fatal("別家店改到了不屬於自己的申訴 —— 擁有者述詞沒有進 SQL")
		}
		ok, err = repo.EscalateBySeller(ctx, id, 2632049831, 0, now)
		if err != nil {
			t.Fatalf("不該回錯：%v", err)
		}
		if ok {
			t.Fatal("K03 的擁有者述詞沒有進 SQL")
		}
	})

	// 🔴 版本述詞：expectedVersion 是必填欄，而「必填」只有寫進述詞才是保證。
	t.Run("版本述詞", func(t *testing.T) {
		id := newCase("OPEN")
		ok, err := repo.MarkSellerResponded(ctx, id, sellerStoreA, 99, due, now)
		if err != nil {
			t.Fatalf("不該回錯：%v", err)
		}
		if ok {
			t.Fatal("版本不符仍然改到了 —— version CAS 沒有進 SQL")
		}
	})

	// buyer_review_due_at 與 first_responded_at 的落地行為。
	t.Run("K02 寫入 buyer_review_due_at 並保留首次回覆時間", func(t *testing.T) {
		id := newCase("OPEN")
		if ok, err := repo.MarkSellerResponded(ctx, id, sellerStoreA, 0, due, now); err != nil || !ok {
			t.Fatalf("首次回覆應成功：ok=%v err=%v", ok, err)
		}
		var reviewDue, firstResponded *time.Time
		if err := pool.QueryRow(ctx,
			`SELECT buyer_review_due_at, first_responded_at FROM invoice_complaints WHERE id=$1`, id).
			Scan(&reviewDue, &firstResponded); err != nil {
			t.Fatal(err)
		}
		// 🔴 少寫 buyer_review_due_at **不會報錯**：那一列只是從 K06 的候選集合裡消失，
		// 該案永遠不會自動結案。所以它必須被斷言。
		if reviewDue == nil {
			t.Fatal("buyer_review_due_at 未寫入 —— K06 永遠撈不到這一列")
		}
		if firstResponded == nil {
			t.Fatal("first_responded_at 未寫入")
		}
		first := *firstResponded

		// 逾期後再回覆一次（OVERDUE → SELLER_RESPONDED）：first_responded_at 走 COALESCE
		// 保留第一次，它是事實時間戳不因二次回覆而後移。
		// overdue_at 必須一起寫：chk_invcomp_overdue_pair 是「OVERDUE ⇒ 非空」的單向要求。
		// （它只有單向 ⇒ K02 從 OVERDUE 轉出時**不必**清 overdue_at，那一欄是事實時間戳。）
		mustExec(ctx, t, pool,
			`UPDATE invoice_complaints SET status='OVERDUE', overdue_at=$2 WHERE id=$1`,
			id, now.Add(-time.Hour))
		var version int64
		if err := pool.QueryRow(ctx, `SELECT version FROM invoice_complaints WHERE id=$1`, id).Scan(&version); err != nil {
			t.Fatal(err)
		}
		if ok, err := repo.MarkSellerResponded(ctx, id, sellerStoreA, version, due.Add(time.Hour), now.Add(time.Hour)); err != nil || !ok {
			t.Fatalf("逾期後仍可回覆：ok=%v err=%v", ok, err)
		}
		if err := pool.QueryRow(ctx,
			`SELECT first_responded_at FROM invoice_complaints WHERE id=$1`, id).Scan(&firstResponded); err != nil {
			t.Fatal(err)
		}
		if !firstResponded.Equal(first) {
			t.Fatalf("first_responded_at 被二次回覆覆寫了：%s → %s", first, *firstResponded)
		}
	})
}

// ---------------------------------------------------------------------------
// 端點 45／67：FailOpenForIssuer ＋ CountOpenForIssuer
// ---------------------------------------------------------------------------

// TestIntegration_Operation_FailOpenForIssuer 是這句 `UPDATE ... FROM` 的**首次實跑**。
//
// 三件事一起驗：
//
//	① 語句真的送得出去（42702 ambiguous 只有在這裡才會現形）
//	② allowed-from ＝ 20 值減四個終態，**正面列出**：終態的四筆不得被打到
//	③ Count 的述詞與 Fail 的 allowed-from 逐字相同 ⇒ 兩者的數字必須一致
func TestIntegration_Operation_FailOpenForIssuer(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewOperationRepository(pool)
	issuerID := seedIssuerFor(ctx, t, pool, sellerStoreA)

	orderNo := fmt.Sprintf("O%d", nextID()%1_000_000_000_000)
	docID := seedOrderDocument(ctx, t, pool, issuerID, iBuyerA, orderNo, "ISSUING", 1, time.Now().UTC())

	// 非終態（會被打掉）與終態（不得被打掉）各取樣。
	openStatuses := []string{"READY", "RETRY_SCHEDULED", "WAITING_CONFIRMATION", "FAILED_PERMANENT", "MANUAL_PROCESS_ALLOWED"}
	terminalStatuses := []string{"SUCCEEDED", "NOT_APPLICABLE", "CANCELLED_BEFORE_ISSUE", "RESOLVED_EXTERNALLY"}

	openIDs := make([]int64, 0, len(openStatuses))
	for _, s := range openStatuses {
		openIDs = append(openIDs, seedOperationAs(ctx, t, pool, docID, s))
	}
	terminalIDs := make([]int64, 0, len(terminalStatuses))
	for _, s := range terminalStatuses {
		terminalIDs = append(terminalIDs, seedOperationAs(ctx, t, pool, docID, s))
	}

	// ③ Count 必須先與 Fail 對得上。
	count, err := repo.CountOpenForIssuer(ctx, issuerID)
	if err != nil {
		t.Fatalf("CountOpenForIssuer: %v", err)
	}
	if count != int64(len(openStatuses)) {
		t.Fatalf("pendingOrderCount = %d, want %d（述詞與 FailOpen 的 allowed-from 應逐字相同）",
			count, len(openStatuses))
	}

	// ① ＋ ② 實跑。
	n, err := repo.FailOpenForIssuer(ctx, domain.FailOpenOperationsForIssuer{
		IssuerID:         issuerID,
		CancelReasonCode: "PROVIDER_SUSPENDED_BY_ADMIN",
		LastError:        "ADM 停用連線",
		Now:              time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("FailOpenForIssuer: %v", err)
	}
	if n != int64(len(openStatuses)) {
		t.Fatalf("轉終態 %d 筆，want %d", n, len(openStatuses))
	}

	for i, id := range openIDs {
		var status string
		var reason *string
		if err := pool.QueryRow(ctx,
			`SELECT status, cancel_reason_code FROM invoice_operations WHERE id=$1`, id).Scan(&status, &reason); err != nil {
			t.Fatal(err)
		}
		if status != "FAILED_PERMANENT" {
			t.Fatalf("%s 應轉 FAILED_PERMANENT，實得 %s", openStatuses[i], status)
		}
		if reason == nil || *reason != "PROVIDER_SUSPENDED_BY_ADMIN" {
			t.Fatalf("%s 的 cancel_reason_code 不符：%v", openStatuses[i], reason)
		}
	}
	// 🔴 終態不得被打到：allowed-from 正面列出的用意就是這個。用 NOT IN 寫的話，
	// 值域日後長胖時新狀態會被靜默納入。
	for i, id := range terminalIDs {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM invoice_operations WHERE id=$1`, id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != terminalStatuses[i] {
			t.Fatalf("終態 %s 被改成 %s —— allowed-from 放行了不該放行的狀態", terminalStatuses[i], status)
		}
	}

	// 打完之後 Count 應歸零（全部已是終態或 FAILED_PERMANENT？）
	// ⚠️ FAILED_PERMANENT **仍在** allowed-from 內（它不是終態，ADM 還能重試）
	// ⇒ 再數一次不會是 0，而是被打掉的那幾筆。這不是 bug，是 §7.3.2「終態只有四個」
	// 的直接後果，寫成 0 的斷言會逼實作把 FAILED_PERMANENT 誤加進終態集合。
	count, err = repo.CountOpenForIssuer(ctx, issuerID)
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(len(openStatuses)) {
		t.Fatalf("FAILED_PERMANENT 不是終態，二次計數應仍為 %d，實得 %d", len(openStatuses), count)
	}
}

// seedOperationAs 是 seedOperation 的可指定狀態版本。
//
// 直接 INSERT 指定 status 而不是先建再 UPDATE：後者會多踩一次狀態機的 CHECK，
// 而本測試要驗的是 FailOpen 的述詞，不是建列路徑。
func seedOperationAs(ctx context.Context, t *testing.T, pool *pgxpool.Pool, documentID int64, status string) int64 {
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
