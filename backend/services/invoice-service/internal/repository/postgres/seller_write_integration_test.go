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


const sellerStoreA int64 = 2685914682

func TestIntegration_Complaint_SellerRespondGuards(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewComplaintRepository(pool)
	issuerID := seedIssuerFor(ctx, t, pool, sellerStoreA)

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
		if reviewDue == nil {
			t.Fatal("buyer_review_due_at 未寫入 —— K06 永遠撈不到這一列")
		}
		if firstResponded == nil {
			t.Fatal("first_responded_at 未寫入")
		}
		first := *firstResponded

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

	count, err := repo.CountOpenForIssuer(ctx, issuerID)
	if err != nil {
		t.Fatalf("CountOpenForIssuer: %v", err)
	}
	if count != int64(len(openStatuses)) {
		t.Fatalf("pendingOrderCount = %d, want %d（述詞與 FailOpen 的 allowed-from 應逐字相同）",
			count, len(openStatuses))
	}

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
	for i, id := range terminalIDs {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM invoice_operations WHERE id=$1`, id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != terminalStatuses[i] {
			t.Fatalf("終態 %s 被改成 %s —— allowed-from 放行了不該放行的狀態", terminalStatuses[i], status)
		}
	}

	count, err = repo.CountOpenForIssuer(ctx, issuerID)
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(len(openStatuses)) {
		t.Fatalf("FAILED_PERMANENT 不是終態，二次計數應仍為 %d，實得 %d", len(openStatuses), count)
	}
}

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
