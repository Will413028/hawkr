package usecase

import (
	"context"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 端點 29 的 allowed-from 在兩個地方各寫了一次：SQL 的 WHERE（真正生效的守衛）與
// receiptAuthEligibleStatuses（決定 UI 看到 eligible 與否）。兩份漂移不會編譯錯、
// 不會有任何測試自然轉紅，而兩個方向的症狀都很難查（見那個 var 的註解）。
//
// 這支測試**從 SQL 解析**出清單，不手寫第二份 —— 手寫的話它就只是第三份會漂移的拷貝。
const receiptAuthQueryName = "ConfirmSellerReceiptAuthorization"

var reviewStatusInRe = regexp.MustCompile(`(?s)AND\s+review_status\s+IN\s*\(([^)]*)\)`)

func sqlReceiptAuthAllowedFrom(t *testing.T) []string {
	t.Helper()
	const path = "../../db/queries/seller_tax_profiles.sql"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀不到 %s：%v", path, err)
	}
	src := string(raw)

	// 先切出那一句，否則會抓到同檔別句的 IN(...)。
	head := "-- name: " + receiptAuthQueryName + " :execrows"
	i := strings.Index(src, head)
	if i < 0 {
		t.Fatalf("在 %s 找不到 %q —— 查詢被改名或搬走了，這支測試就失去對象，"+
			"不得改成 skip（那會讓它在最需要的時候靜默轉綠）", path, head)
	}
	rest := src[i+len(head):]
	if j := strings.Index(rest, "\n-- name: "); j >= 0 {
		rest = rest[:j]
	}
	m := reviewStatusInRe.FindStringSubmatch(rest)
	if m == nil {
		t.Fatalf("%s 這一句裡找不到 `AND review_status IN (...)`。"+
			"若它改回單值等號寫法，本測試要跟著改，不是刪掉", receiptAuthQueryName)
	}
	var out []string
	for _, part := range strings.Split(m[1], ",") {
		v := strings.TrimSpace(part)
		v = strings.Trim(v, "'")
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		t.Fatal("解析出 0 個狀態 —— 解析器壞了，不是 SQL 空了")
	}
	sort.Strings(out)
	return out
}

func TestReceiptAuthEligibleStatusesMatchSQL(t *testing.T) {
	want := sqlReceiptAuthAllowedFrom(t)

	got := make([]string, 0, len(receiptAuthEligibleStatuses))
	for s := range receiptAuthEligibleStatuses {
		got = append(got, s)
	}
	sort.Strings(got)

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Go 的 receiptAuthEligibleStatuses 與 SQL 的 allowed-from 漂移了\n"+
			"  SQL: %v\n  Go : %v", want, got)
	}

	// 鑑別力：解析器真的看得到那五個值，而不是回了一個剛好也等於空集合的東西。
	if len(want) < 2 {
		t.Fatalf("解析結果只有 %d 個值，這支測試等於沒在比對：%v", len(want), want)
	}
}

// receiptEligibility 的逐狀態表。**不是**把上面那份 map 抄過來 —— 這裡要驗的是
// 「三個 ineligible 值互斥且窮舉」這句宣稱本身，所以十個狀態一個不漏。
func TestReceiptEligibility_AllTenStatuses(t *testing.T) {
	valid := tNow.Add(300 * 24 * time.Hour)
	cases := map[string]struct {
		eligible bool
		reason   string
	}{
		// NOT_STARTED 是「還沒有 profile 列」的虛擬態，正常路徑走不到 receiptEligibility；
		// 仍逐字列出，才不會在它哪天變成真狀態時靜默落進 NOT_APPROVED 而沒人發現。
		"NOT_STARTED":               {false, receiptIneligibleNotApproved},
		"DRAFT":                     {false, receiptIneligibleNotApproved},
		"SUBMITTED":                 {true, ""},
		"UNDER_REVIEW":              {true, ""},
		"SUPPLEMENT_REQUIRED":       {true, ""},
		"SELF_ISSUE_PENDING":        {true, ""},
		"APPROVED_INVOICE_REQUIRED": {false, receiptIneligibleNotApproved},
		"APPROVED_INVOICE_EXEMPT":   {true, ""},
		"REJECTED":                  {false, receiptIneligibleNotApproved},
		"EXPIRED":                   {false, receiptIneligibleNotApproved},
		"REVOKED":                   {false, receiptIneligibleApprovalRevoked},
	}
	if len(cases) != len(codemeta.AllSellerTaxReviewStatuss) {
		t.Fatalf("本表 %d 個狀態，碼表有 %d 個 —— 新增狀態時必須在這裡表態，"+
			"否則新狀態會靜默落進 NOT_APPROVED", len(cases), len(codemeta.AllSellerTaxReviewStatuss))
	}
	for st, want := range cases {
		t.Run(st, func(t *testing.T) {
			p := domain.SellerTaxProfile{ReviewStatus: st}
			if st == "APPROVED_INVOICE_EXEMPT" {
				p.ApprovalValidUntil = &valid
			}
			gotOK, gotReason := receiptEligibility(p, tNow)
			if gotOK != want.eligible || gotReason != want.reason {
				t.Fatalf("%s：得 (%v,%q)，want (%v,%q)", st, gotOK, gotReason, want.eligible, want.reason)
			}
		})
	}

	// APPROVAL_EXPIRED 只對已核定成立；審核中的賣家 approval_valid_until 恆為 NULL，
	// 拿它去判會把「還沒核定」誤報成「已過期」。
	t.Run("已核定但屆期", func(t *testing.T) {
		past := tNow.Add(-time.Hour)
		p := domain.SellerTaxProfile{
			ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
			ApprovalValidUntil: &past,
		}
		if ok, reason := receiptEligibility(p, tNow); ok || reason != receiptIneligibleApprovalExpired {
			t.Fatalf("得 (%v,%q)", ok, reason)
		}
	})
}

// T08 守衛的陰陽兩面。陰性是本節的主體：approved-only 的保證，實質內容是
// 「沒授權就不會核定」，那一面沒有獨立證據的話，陽性通過完全不代表守衛存在。
func TestTaxProfile_Decide_ApproveExemptRequiresReceiptAuthorization(t *testing.T) {
	newExemptDecision := func() TaxDecisionInput {
		return TaxDecisionInput{
			ActorAccountID: tAdmin, StoreID: tStoreA,
			Decision:  codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
			Reason:    "檢附國稅局核定函",
			Confirmed: true,
		}
	}

	t.Run("未授權_必須擋下且不得改動狀態", func(t *testing.T) {
		p := profileAt("UNDER_REVIEW")
		p.TaxID = nil // chk_stp_exempt_no_taxid
		// 刻意不給 ReceiptAuthorizedAt —— 這就是被測的情境。
		e := newTaxEnv(t, p)

		_, err := e.uc.Decide(context.Background(), newExemptDecision())
		wantBiz(t, err, bizcode.InvoiceTaxApprovalReceiptAuthMissing)

		row := e.profiles.byIssuer[tIssuerA]
		if row.ReviewStatus != "UNDER_REVIEW" {
			t.Fatalf("守衛不成立時不得核定，實得 %q", row.ReviewStatus)
		}
		if row.ApprovedAt != nil {
			t.Fatalf("approved_at 不得被寫入：%v", row.ApprovedAt)
		}
		if len(e.outbox.events) != 0 {
			t.Fatalf("擋下時不得發事件：%+v", e.outbox.events)
		}
	})

	t.Run("已授權_核定成立", func(t *testing.T) {
		p := profileAt("UNDER_REVIEW")
		p.TaxID = nil
		authorized := tNow.Add(-24 * time.Hour)
		consent := "v1"
		p.ReceiptAuthorizedAt, p.ReceiptConsentVersion = &authorized, &consent
		e := newTaxEnv(t, p)

		res, err := e.uc.Decide(context.Background(), newExemptDecision())
		if err != nil {
			t.Fatalf("已授權應可核定：%v", err)
		}
		if res.ReviewStatus != "APPROVED_INVOICE_EXEMPT" {
			t.Fatalf("status = %s", res.ReviewStatus)
		}
	})

	// 鏡像的另一半：核定「應開統一發票」與收據授權無關，不得被這道守衛波及。
	t.Run("核定應開發票_不受授權守衛拘束", func(t *testing.T) {
		e := newTaxEnv(t, profileAt("UNDER_REVIEW")) // 有統編、無授權
		in := newExemptDecision()
		in.Decision = codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value
		if _, err := e.uc.Decide(context.Background(), in); err != nil {
			t.Fatalf("應開發票不需要收據授權，卻被擋：%v", err)
		}
	})
}
