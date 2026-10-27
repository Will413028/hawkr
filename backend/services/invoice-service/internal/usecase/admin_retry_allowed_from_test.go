package usecase

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// ---------------------------------------------------------------------------
// 「同一判斷兩份實作」——#3089 第 8 項的缺陷類
//
// 讀端（can_retry，賣家／ADM 畫面上那顆按鈕的可見性）與寫端（applyRetry，端點 52）
// 曾經各存一份狀態清單：讀端只認 FAILED_PERMANENT、寫端接受三個。後果是
// FAILED_RETRY_EXHAUSTED 與 SELLER_ACTION_REQUIRED 下端點 52 實打回 200、
// status→READY，而 ADM 畫面上那顆「安全重試」鍵**根本不渲染**（前端是條件渲染
// 不是 disabled）⇒ 功能做好了，使用者按不到。
//
// 兩份清單已經合併成 adminRetryKindOf 一支。本測試守的是「未來有人又把它拆開」，
// 以及**更上游的那一層**：Go 的清單與 SQL 的 allowed-from 是不是還對得上。
//
// 🔴 **fixture 從 SQL 原始碼解析，不手寫**（§0.3）：手寫一份「我覺得 SQL 是這樣寫的」
// 清單，就是自己跟自己對，SQL 改了它不會紅。那正是這個缺陷類的成因。
// ---------------------------------------------------------------------------

// allowedFromInSQL 從 db/queries/invoice_operations.sql 解出某一支 CAS 的 allowed-from。
//
// 只認 `AND status = 'X'` 與 `AND status IN ('X', 'Y')` 兩種形態；解不出來就 Fatal
// ——**看不到就要報錯，不是看不到就放行**（§3.10）。SQL 改成別的寫法時，
// 正確的反應是本測試爆炸要人來看，而不是靜默回一個空集合然後全綠。
func allowedFromInSQL(t *testing.T, queryName string) map[string]bool {
	t.Helper()
	path := filepath.Join("..", "..", "db", "queries", "invoice_operations.sql")
	raw, err := os.ReadFile(path) //nolint:gosec // 固定的 repo 內相對路徑
	if err != nil {
		t.Fatalf("讀不到 %s（本測試已經在守一個不存在的東西）：%v", path, err)
	}
	src := string(raw)

	start := strings.Index(src, "-- name: "+queryName+" ")
	if start < 0 {
		t.Fatalf("SQL 裡找不到 %q —— 它被改名或刪掉了，而 Go 這邊沒跟上", queryName)
	}
	// 到下一支 query 為止（或檔尾）。
	body := src[start:]
	if next := strings.Index(body[len("-- name: "):], "-- name: "); next >= 0 {
		body = body[:next+len("-- name: ")]
	}
	// 只看述詞，不看註解：註解裡也寫著狀態名，算進來就會得到一份「看起來對」的假清單。
	var stmt strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		stmt.WriteString(line)
		stmt.WriteString("\n")
	}

	out := map[string]bool{}
	if m := regexp.MustCompile(`AND\s+status\s+IN\s*\(([^)]*)\)`).FindStringSubmatch(stmt.String()); m != nil {
		for _, v := range regexp.MustCompile(`'([A-Z_]+)'`).FindAllStringSubmatch(m[1], -1) {
			out[v[1]] = true
		}
	} else if m := regexp.MustCompile(`AND\s+status\s*=\s*'([A-Z_]+)'`).FindStringSubmatch(stmt.String()); m != nil {
		out[m[1]] = true
	}
	if len(out) == 0 {
		t.Fatalf("%s 解不出 allowed-from —— SQL 的述詞寫法變了，本測試已失效", queryName)
	}
	return out
}

func TestAdminRetryKindMatchesSQLAllowedFrom(t *testing.T) {
	afterFix := allowedFromInSQL(t, "AdminRetryInvoiceOperationAfterFix")
	reset := allowedFromInSQL(t, "AdminResetInvoiceOperationAttempts")

	// 兩支 CAS 的 allowed-from 不得重疊：重疊代表同一個狀態有兩條 J 邊，
	// 而 applyRetry 只會走其中一條 ⇒ 另一條永遠不會發生，卻沒有人知道。
	for s := range afterFix {
		if reset[s] {
			t.Errorf("%s 同時出現在兩支 CAS 的 allowed-from —— applyRetry 只走得到一條", s)
		}
	}

	for _, st := range codemeta.AllInvoiceOperationStatuss {
		want := adminRetryNone
		switch {
		case afterFix[st.Value]:
			want = adminRetryAfterFix
		case reset[st.Value]:
			want = adminRetryResetAttempts
		}
		if got := adminRetryKindOf(st.Value); got != want {
			t.Errorf("adminRetryKindOf(%q) = %v，SQL 的 allowed-from 說應該是 %v", st.Value, got, want)
		}
	}
}

// TestCanRetryOperationMatchesApplyRetry 讓讀端與寫端不能再分家。
//
// 這一支在兩者合併之後是恆真的——**那正是重點**：它把「不可能漂移」變成一個
// 會在有人拆開時立刻轉紅的斷言，而不是一句寫在註解裡的期望。
func TestCanRetryOperationMatchesApplyRetry(t *testing.T) {
	var readable, writable []string
	for _, st := range codemeta.AllInvoiceOperationStatuss {
		if CanRetryOperation(st.Value) {
			readable = append(readable, st.Value)
		}
		if adminRetryKindOf(st.Value) != adminRetryNone {
			writable = append(writable, st.Value)
		}
	}
	sort.Strings(readable)
	sort.Strings(writable)
	if strings.Join(readable, ",") != strings.Join(writable, ",") {
		t.Errorf("can_retry 與 applyRetry 的可重試集合不一致：\n  讀端 %v\n  寫端 %v\n"+
			"兩者必須走同一支 adminRetryKindOf——分家的後果是按鈕亮在按不動的狀態上，"+
			"或功能做好了使用者按不到（#3089 第 8 項）", readable, writable)
	}
	if len(readable) == 0 {
		t.Fatal("可重試集合是空的 —— 枚舉來源壞了，而壞掉的症狀是兩邊都空＝全綠")
	}
}
