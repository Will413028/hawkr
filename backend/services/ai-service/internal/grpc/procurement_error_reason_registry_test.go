package grpc

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/stretchr/testify/require"
)

// procurementStatus 的 reason 參數是對外契約的一部分：它是 gateway error envelope
// 的 `reason` 欄位，hawkr-website 直接拿它當 ProcurementError.code 並對其中
// 幾個值做字串比對。值域的 SSOT 是 docs/代碼管理（CodeMeta）/業務狀態碼.md 的
// ProcurementErrorReason，經 generate.sh --go-registry 產出 pkg/codemeta/registry.go。
//
// 這支測試把「原始碼裡實際送出的 reason」與「registry 登記的值域」對起來。沒有它，
// 新增一個 inline 字面值不會有任何訊號——既不會編譯失敗，也不會有 lint 抱怨，
// 而契約文件就這樣悄悄落後於實作。
//
// 新增 reason 的正確順序：先在 Markdown 加一列 → 跑 generate.sh --go-registry →
// 才在這裡使用。反過來做，本測試會紅。
var procurementReasonLiteral = regexp.MustCompile(`procurementStatus\([^,]+,\s*"([A-Z_]{3,})"`)

func TestProcurementErrorReasonsAreRegistered(t *testing.T) {
	// 掃整個 package 而不是寫死 procurement_handler.go：那個檔案目前 1200+ 行、承載
	// 18 支 RPC，拆檔是完全可預期的重構，而綁死檔名的話拆完之後測試仍然全綠、覆蓋卻
	// 已經悄悄漏掉搬走的那些 reason。
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	var matches [][][]byte
	var scanned []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		require.NoErrorf(t, err, "讀不到 %s", f)
		if m := procurementReasonLiteral.FindAllSubmatch(src, -1); len(m) > 0 {
			matches = append(matches, m...)
			scanned = append(scanned, f)
		}
	}

	require.NotEmpty(t, matches,
		"在整個 package 掃不到任何 reason 字面值。這幾乎一定是 regex 對不上新的寫法，"+
			"而不是真的沒有 reason——空結果會讓本測試變成永遠通過的擺設，所以這裡 fail-loud。")

	seen := map[string]bool{}
	var unregistered []string
	for _, m := range matches {
		reason := string(m[1])
		if seen[reason] {
			continue
		}
		seen[reason] = true
		if _, ok := codemeta.ProcurementErrorReasonFromCode(reason); !ok {
			unregistered = append(unregistered, reason)
		}
	}
	sort.Strings(unregistered)
	require.Emptyf(t, unregistered,
		"以下 reason 出現在原始碼但沒有登記在 CodeMeta 的 ProcurementErrorReason：%v\n"+
			"請先在 docs/代碼管理（CodeMeta）/業務狀態碼.md 補上，再跑 generate.sh --go-registry。",
		unregistered)

	t.Logf("已從 %v 比對 %d 個 distinct reason，全部登記在案", scanned, len(seen))
}
