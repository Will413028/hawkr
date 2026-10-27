package usecase

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestResolvePayuniError_MatchesGolden 把全部 136 筆代碼的解析結果釘死在
// testdata/payuni_error_catalog.golden。
//
// 這支測試存在的原因：TestResolvePayuniError_CoversEverySnapshotCode 只驗
// 「remedy 落在三個合法值之一」，驗不出「分類到的是不是*對*的那一種」——
// 有人把某個代碼搬錯組（例如該進 CONTACT_SUPPORT 的碼被搬進 SELLER_FIX），
// 覆蓋率測試照樣全綠。golden 檔逐碼比對 remedy／retryStep／文案來源，任何一
// 個代碼的分類被改動，這支測試就會紅。
//
// messageSource 欄位另外堵一個縫：覆蓋率測試的 `Message != code` 斷言分不出
// 「CodeMeta 真實文案」與「fail-closed 通用文案」——若某代碼在
// payuniErrorGroups 卻在 CodeMeta 漏登，ResolvePayuniError 會靜默落回
// payuniUnknownErrorMessage，覆蓋率測試不會發現。這裡把來源也釘進 golden，
// 任何一筆是 fallback 都判定失敗。
//
// golden 檔本身是純資料、不帶註解行，故意讓 diff 乾淨可讀。這代表：
// 這個檔案的任何 diff 都是某個代碼的處置分類被改動了，review 時要當成
// 行為變更看待，不是格式變更。golden 檔由實作實際跑出來的結果產生，不是
// 手打；重新產生時對照 payuniErrorGroups 的分組意圖抽驗，不要照單全收。
func TestResolvePayuniError_MatchesGolden(t *testing.T) {
	codes := loadPayuniErrorCodeSnapshot(t)

	f, err := os.Open("testdata/payuni_error_catalog.golden")
	if err != nil {
		t.Fatalf("open golden: %v", err)
	}
	defer f.Close()

	var golden []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		golden = append(golden, s.Text())
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan golden: %v", err)
	}

	if len(golden) != len(codes) {
		t.Fatalf("golden has %d lines, snapshot has %d codes — regenerate testdata/payuni_error_catalog.golden from ResolvePayuniError output", len(golden), len(codes))
	}

	for i, code := range codes {
		got := ResolvePayuniError(code, "")
		source := "catalog"
		if got.Message == payuniUnknownErrorMessage() {
			source = "fallback"
		}
		wantLine := golden[i]
		gotLine := fmt.Sprintf("%s\t%s\t%s\t%s", code, got.Remedy, got.RetryStep, source)
		if gotLine != wantLine {
			t.Errorf("code %s: golden mismatch\n  want: %q\n  got:  %q", code, wantLine, gotLine)
		}
		if source == "fallback" {
			t.Errorf("code %s: message fell back to the generic fail-closed text — it is in payuniErrorGroups but missing from CodeMeta", code)
		}
	}

	// 順帶確保 golden 的第一欄與 code 一致，避免有人手改成兩邊行序不同步。
	for i, line := range golden {
		firstCol := line[:strings.IndexByte(line, '\t')]
		if firstCol != codes[i] {
			t.Fatalf("golden line %d code %q does not match snapshot code %q at the same position — golden and testdata/payuni_error_codes.txt are out of sync", i, firstCol, codes[i])
		}
	}
}
