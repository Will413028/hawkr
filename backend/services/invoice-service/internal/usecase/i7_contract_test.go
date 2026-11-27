package usecase

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)


var i7Exempt = map[string]string{
	"idempotency.go:ClaimIdempotency": "invoice_idempotency_keys 不是 chk_invhist_subject 的七個 aggregate 之一；" +
		"它是冪等鍵的租約，不是業務狀態",
}

func TestI7_EveryStatusTransitionIsRecorded(t *testing.T) {
	statusQueries := sqlQueriesThatSetStatus(t)
	if len(statusQueries) < 20 {
		t.Fatalf("只掃到 %d 個會改 status 的 query，遠少於預期 ⇒ 掃描沒打到對的地方（fail-closed）", len(statusQueries))
	}
	methods := repositoryMethodsFor(t, statusQueries)
	if len(methods) < 20 {
		t.Fatalf("只對應到 %d 個 repository 方法 ⇒ adapter 掃描失效（fail-closed）", len(methods))
	}

	funcs := usecaseFuncs(t)
	var missing []string
	for _, f := range funcs {
		if !callsAny(f.body, methods) {
			continue
		}
		key := f.file + ":" + f.name
		if _, ok := i7Exempt[key]; ok {
			continue
		}
		wantSubject := ""
		for m := range methods {
			if !strings.Contains(f.body, "."+m+"(") {
				continue
			}
			if s := i7SubjectOf(m); s != "" {
				wantSubject = s
				break
			}
		}
		if wantSubject != "" {
			if writesSubject(f.name, funcs, wantSubject, 0) {
				continue
			}
		} else if writesHistory(f.name, funcs, 0) {
			continue
		}
		callers := callersOf(f, funcs)
		if len(callers) > 0 {
			all := true
			for _, c := range callers {
				ok := false
				if wantSubject != "" {
					ok = writesSubject(c, funcs, wantSubject, 0)
				} else {
					ok = writesHistory(c, funcs, 0)
				}
				if !ok {
					all = false
					break
				}
			}
			if all {
				continue
			}
		}
		missing = append(missing, key)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("I7：以下 %d 處改了 status 卻沒有留痕（自己／委派／外層都沒寫）：\n  %s\n\n"+
			"修法：在 CAS **成功之後**呼叫 appendStatusHistory／appendStatusHistoryDetail／appendStatusHistoryActor；"+
			"CAS rows=0 不得寫（此時快照狀態不是真實狀態）；自轉移不寫。\n"+
			"確定不是缺口就加進 i7Exempt 並寫明理由。",
			len(missing), strings.Join(missing, "\n  "))
	}

	for key := range i7Exempt {
		found := false
		for _, f := range funcs {
			if f.file+":"+f.name == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("i7Exempt 有過期條目 %q（那個函式已不存在），請刪除", key)
		}
	}
}

var reSQLName = regexp.MustCompile(`(?s)-- name: (\w+) :\w+\n(.*?)(?:\n-- name: |\z)`)
var reSetStatus = regexp.MustCompile(`(?i)\bSET\s+status\s*=`)

func sqlQueriesThatSetStatus(t *testing.T) map[string]bool {
	t.Helper()
	dir := filepath.Join("..", "..", "db", "queries")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("讀不到 %s：%v（fail-closed）", dir, err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // 固定的 repo 內相對路徑
		if err != nil {
			t.Fatalf("讀不到 %s：%v", e.Name(), err)
		}
		for _, m := range reSQLName.FindAllStringSubmatch(string(raw), -1) {
			if reSetStatus.MatchString(m[2]) {
				out[m[1]] = true
			}
		}
	}
	return out
}

var reRepoMethod = regexp.MustCompile(`func \(r \*\w+\) (\w+)\(`)

func repositoryMethodsFor(t *testing.T, queries map[string]bool) map[string]bool {
	t.Helper()
	dir := filepath.Join("..", "repository", "postgres")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("讀不到 %s：%v（fail-closed）", dir, err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // 同上
		if err != nil {
			t.Fatalf("讀不到 %s：%v", e.Name(), err)
		}
		src := string(raw)
		locs := reRepoMethod.FindAllStringSubmatchIndex(src, -1)
		for i, loc := range locs {
			end := len(src)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			body := src[loc[0]:end]
			name := src[loc[2]:loc[3]]
			for q := range queries {
				if strings.Contains(body, "."+q+"(") {
					out[name] = true
					break
				}
			}
		}
	}
	return out
}

type i7Func struct{ file, name, body string }

var reFuncStart = regexp.MustCompile(`(?m)^func `)
var reFuncName = regexp.MustCompile(`^func (?:\([^)]*\) )?(\w+)`)

func usecaseFuncs(t *testing.T) []i7Func {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("讀不到 usecase 目錄：%v", err)
	}
	var out []i7Func
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(e.Name()) //nolint:gosec // 同上
		if err != nil {
			t.Fatalf("讀不到 %s：%v", e.Name(), err)
		}
		src := string(raw)
		idx := reFuncStart.FindAllStringIndex(src, -1)
		for i, loc := range idx {
			end := len(src)
			if i+1 < len(idx) {
				end = idx[i+1][0]
			}
			body := src[loc[0]:end]
			m := reFuncName.FindStringSubmatch(body)
			if m == nil {
				continue
			}
			out = append(out, i7Func{file: e.Name(), name: m[1], body: i7StripComments(body)})
		}
	}
	return out
}

func i7StripComments(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, line := range lines {
		var b strings.Builder
		for len(line) > 0 {
			if inBlock {
				if k := strings.Index(line, "*/"); k >= 0 {
					line = line[k+2:]
					inBlock = false
					continue
				}
				line = ""
				break
			}
			lc := strings.Index(line, "//")
			bc := strings.Index(line, "/*")
			switch {
			case lc < 0 && bc < 0:
				b.WriteString(line)
				line = ""
			case bc < 0 || (lc >= 0 && lc < bc):
				b.WriteString(line[:lc])
				line = ""
			default:
				b.WriteString(line[:bc])
				line = line[bc+2:]
				inBlock = true
			}
		}
		out = append(out, b.String())
	}
	return strings.Join(out, "\n")
}

var i7HistCalls = []string{
	"appendStatusHistory(", "appendStatusHistoryDetail(", "appendStatusHistoryActor(",
	"AppendStatusHistory(",
}

func i7SubjectOf(method string) string {
	switch {
	case strings.Contains(method, "Complaint") || method == "MarkOverdue" ||
		method == "DecideEscalated" || method == "ReturnToSeller" ||
		method == "EscalateByBuyer" || method == "EscalateBySeller" ||
		method == "MarkSellerResponded" || method == "Withdraw" || method == "Reopen":
		return "subjectComplaint"
	case strings.Contains(method, "Correction") || method == "MarkUpdatedBeforeIssue" ||
		method == "StartVoidReissue" || method == "RetryReissue":
		return "subjectCorrectionRequest"
	default:
		return ""
	}
}

var i7SubjectAliases = map[string][]string{
	"subjectComplaint": {"complaintResourceType"},
}

func callsAny(body string, names map[string]bool) bool {
	for n := range names {
		if strings.Contains(body, "."+n+"(") {
			return true
		}
	}
	return false
}

func writesHistory(name string, funcs []i7Func, depth int) bool {
	for _, f := range funcs {
		if f.name != name {
			continue
		}
		for _, h := range i7HistCalls {
			if strings.Contains(f.body, h) {
				return true
			}
		}
		if depth < 2 {
			for _, g := range funcs {
				if g.name == name {
					continue
				}
				if strings.Contains(f.body, "."+g.name+"(") && writesHistory(g.name, funcs, depth+1) {
					return true
				}
			}
		}
	}
	return false
}

func callersOf(target i7Func, funcs []i7Func) []string {
	var out []string
	for _, f := range funcs {
		if f.file != target.file || f.name == target.name {
			continue
		}
		if strings.Contains(f.body, "."+target.name+"(") || strings.Contains(f.body, " "+target.name+"(") {
			out = append(out, f.name)
		}
	}
	return out
}

// TestProviderResultAlwaysCarriesMessage 守 #3082 的另一半。
//
// `MarkOperationProviderResult` 有 ProviderCode／ProviderMessage 兩欄，而 2026-09-04
// dev 實測發現 4 個寫入點裡有 3 個只填了 code：重試一筆 REISSUE 之後
// `provider_code=3040122` 有落庫、`provider_message` 仍是 NULL。
//
// 🔴 **LastError 不能代替它**：那是我方組出來的診斷字串
// （`amego: /json/f0401: code=… class=… kind=…`），而 provider_message 要的是
// 對方**原樣回的那句話**（APIError.Msg）——§8.2.1 稱它「落庫供稅務舉證」，
// 被稽核時拿得出來的是它，不是我方的 log 格式。
//
// 判準是結構性的（掃 composite literal），不依賴命名或註解。
func TestProviderResultAlwaysCarriesMessage(t *testing.T) {
	funcs := usecaseFuncs(t)
	if len(funcs) == 0 {
		t.Fatal("掃不到任何 usecase 函式（fail-closed）")
	}
	re := regexp.MustCompile(`(?s)domain\.MarkOperationProviderResult\{(.*?)\}\)`)
	_ = reProviderMsgField
	total, bad := 0, []string{}
	for _, f := range funcs {
		for _, m := range re.FindAllStringSubmatch(f.body, -1) {
			total++
			if !reProviderMsgField.MatchString(m[1]) {
				bad = append(bad, f.file+":"+f.name)
			}
		}
	}
	if total == 0 {
		t.Fatal("掃不到任何 MarkOperationProviderResult 寫入點 ⇒ 判準失效（fail-closed）")
	}
	if len(bad) > 0 {
		t.Fatalf("以下 %d 處只填了 provider_code 沒填 provider_message：\n  %s\n\n"+
			"provider_message 要的是 AMEGO 原樣回的那句話（apiErr.Msg），"+
			"不是我方組的 LastError。", len(bad), strings.Join(bad, "\n  "))
	}
}

var reProviderMsgField = regexp.MustCompile(`(?m)^\s*ProviderMessage:\s*\S`)

func writesSubject(name string, funcs []i7Func, subject string, depth int) bool {
	for _, f := range funcs {
		if f.name != name {
			continue
		}
		for _, h := range i7HistCalls {
			for off := 0; ; {
				rel := strings.Index(f.body[off:], h)
				if rel < 0 {
					break
				}
				idx := off + rel
				off = idx + len(h)
				end := idx + 200
				if end > len(f.body) {
					end = len(f.body)
				}
				if strings.Contains(f.body[idx:end], subject) {
					return true
				}
				for _, alias := range i7SubjectAliases[subject] {
					if strings.Contains(f.body[idx:end], alias) {
						return true
					}
				}
			}
		}
		if depth < 2 {
			for _, g := range funcs {
				if g.name == name {
					continue
				}
				if strings.Contains(f.body, "."+g.name+"(") && writesSubject(g.name, funcs, subject, depth+1) {
					return true
				}
			}
		}
	}
	return false
}

// TestI7_ComplaintAxisRecordsRealActor 守申訴軸的 **actor_type**。
//
// 🔴 I7 的主測試只問「有沒有寫」，不問「寫對了誰」。而
// `appendStatusHistory`／`appendStatusHistoryDetail` 把 `ActorType` **硬寫成 SYSTEM**，
// 於是 #3109 接上申訴軸留痕時，三條路徑（買家 K12/K13/K14、賣家 K02/K03、ADM K09/K10）
// 全部把「誰按的」抹成系統。dev 實測（2026-09-05，complaint 7501677659433205760）：
// 賣家按下「已重新寄送」之後的 `OPEN → SELLER_RESPONDED` 那一列
// `actor_type = SYSTEM`、`actor_account_id = NULL` —— 而 A10 稽核與 AuditTrailDrawer
// 讀的就是這一欄。
//
// 判準錨在**原始碼**（同本檔其餘部分）：這三個函式必須呼叫帶 actor 的變體，
// 且呼叫點附近要出現該路徑對應的 actor 常數。
// ⚠️ 期望值硬寫字面值，不引用被測 code 用到的常數——否則常數一改，斷言跟著變。
func TestI7_ComplaintAxisRecordsRealActor(t *testing.T) {
	funcs := usecaseFuncs(t)
	want := []struct {
		fn    string
		actor string
	}{
		{"Respond", "complaintActorSeller"},
		{"transition", "complaintActorBuyer"},
		{"Act", "jobActorAdmin"},
	}
	byName := map[string]string{}
	for _, f := range funcs {
		byName[f.name] = f.body
	}
	for _, w := range want {
		body, ok := byName[w.fn]
		if !ok {
			t.Fatalf("找不到函式 %s —— 判準失明（改名了就把這份清單一起改）", w.fn)
		}
		idx := strings.Index(body, "appendStatusHistoryActor(")
		if idx < 0 {
			idx = strings.Index(body, "AppendStatusHistory(")
		}
		if idx < 0 {
			t.Errorf("%s 既沒用 appendStatusHistoryActor 也沒直接呼叫 AppendStatusHistory —— "+
				"另外兩支 helper 把 actor_type 硬寫成 SYSTEM，稽核就看不出是誰按的", w.fn)
			continue
		}
		end := idx + 400
		if end > len(body) {
			end = len(body)
		}
		if !strings.Contains(body[idx:end], w.actor) {
			t.Errorf("%s 的留痕沒有帶 %s —— actor 寫錯比不寫更難查（畫面上看起來有值）",
				w.fn, w.actor)
		}
	}
}

// TestI7StripComments 守剝註解本身。
//
// 它是本判準的前提：剝錯的後果不是「少抓一個」而是「整段程式碼被吃掉」，
// 而那會讓判準對真正的缺口靜默放行。案例①②來自 2026-09-05 的實際誤報，
// ③ 來自 `.claude/rules/06-no-small-tails.md` §3.10 記載的 ADM 事故形狀。
func TestI7StripComments(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"① 行註解裡提到的呼叫名不算數",
			"a := 1 // 這裡解釋 AppendStatusHistory( 的用法\nb := 2\n",
			"a := 1 \nb := 2\n"},
		{"② 區塊註解整段不算數",
			"x()\n/* 說明 appendStatusHistoryActor( */\ny()\n",
			"x()\n\ny()\n"},
		{"③ 行註解裡的 /* 不得開啟區塊（否則後面整段被吃掉）",
			"p()\n// 舉例 /* 開頭\nq()\n",
			"p()\n\nq()\n"},
		{"④ 真正的呼叫要留下來",
			"\tu.audit.AppendStatusHistory(ctx, e)\n",
			"\tu.audit.AppendStatusHistory(ctx, e)\n"},
		{"⑤ 跨行區塊註解",
			"m()\n/* 第一行\n第二行 */ n()\n",
			"m()\n\n n()\n"},
	}
	for _, c := range cases {
		if got := i7StripComments(c.in); got != c.want {
			t.Errorf("%s\n  got  %q\n  want %q", c.name, got, c.want)
		}
	}
}
