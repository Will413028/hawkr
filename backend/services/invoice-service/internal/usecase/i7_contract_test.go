package usecase

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// I7 契約測試：**每一次 status 轉移都有一列 invoice_status_history**（#3109）。
//
// 設計書 §7 不變式表把 I7 的保證寫成「transition helper 是唯一寫入點」，而那個
// helper 在 2026-09-04 之前**不存在**，所以 I7 從上線起沒有任何東西在守。
// dev 實證：operation 軌跡自相矛盾（中間兩步整段消失）、COMPLAINT 軸的歷程全表只有 1 列。
//
// 本測試的判準**錨在 SQL 事實**而不是命名慣例（`.claude/rules/02-judgment.md` §5：
// 「命令本身的鑑別力也要驗」）：
//
//	① db/queries/*.sql 裡真的有 `SET status =` 的 query ＝ 會改狀態的那些（對照組）
//	② repository adapter 呼叫它們 ⇒ 得到 domain 方法名（不靠 Mark*／Claim* 這種前綴猜）
//	③ usecase 呼叫那些方法的函式，自己／委派的／外層三者之一必須寫 history
//
// 🔴 **fail-closed**：①②任一步找不到東西就 t.Fatal。「掃不到所以沒問題」是
// §3.10 列的機制共同死因——這裡讓它報錯而不是靜默通過。
//
// ─────────────────────────────────────────────────────────────────────────
// 🔴 **這道 gate 擋不住什麼**（誠實標明邊界，`.claude/rules/06-no-small-tails.md` §3.10）
//
// 它是**函式層**的判準，不是流程層的。`writesHistory`／`writesSubject` 會遞迴
// 追委派（深度 2），所以它證明的是「這個函式可達的某處有寫」，
// **不是「我正在看的這個分支有寫」**。
//
// 具體後果：一個函式有兩條分支，A 分支寫了、B 分支沒寫 ⇒ gate 放行。
// 2026-09-04 實測到的正是這個——`correction.Execute` 的 R03 分支漏寫，
// 而它委派的 `reject`（R05 分支）有寫 ⇒ 拿掉 R03 的留痕，gate 不轉紅。
//
// ⇒ **本 gate 抓的是「整個函式完全沒寫」，分支層級的漏寫要靠 code review
// 與那一軸的 dev 實證**（查 `invoice_status_history` 某個 subject_type 的
// 轉移種類分布，缺哪幾條一目了然——R03／R05 就是這樣被發現的）。
//
// 要補上分支層級，需要真正的控制流分析（go/ssa 之類），成本遠高於收益：
// 那會讓這支測試變成一個需要維護的靜態分析器。**先讓它誠實地擋住它擋得住的。**
// ─────────────────────────────────────────────────────────────────────────

// i7Exempt 是**經人工坐實**的豁免，每一條都要寫清楚為什麼不是缺口。
//
// ⚠️ 新增條目前先讀 §5.1.12 的 chk_invhist_subject 七值：只有那七個 aggregate
// 需要狀態歷程。不在七值內的（冪等鍵、租約、outbox）本來就不該寫。
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
		// 🔴 先看 subject：改了哪個 aggregate 就要寫那個 subject 的歷程。
		// 只檢查「有沒有寫 history」會讓「寫錯軸」通過（見 i7SubjectOf 的紅字）。
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
		// 外層：同檔案內呼叫它的函式有沒有寫
		// 外層：同檔案內呼叫它的函式有沒有寫。
		//
		// 🔴 這一格也必須看 subject（2026-09-04 第二次修）：第一次只改了自己那一格，
		// 而 caller 這一格仍用 writesHistory ⇒ `correction.Submit` 寫 DOCUMENT 軸
		// 照樣讓 `Execute` 通過，變異測試不轉紅。**同一個盲點修了一半比沒修更糟**——
		// 它讓人以為判準已經有 subject 意識了。
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

	// 新鮮度：豁免的對象消失了就要清掉，否則清單會腐化成「沒有人記得為什麼」。
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

// i7HistCalls 是「會寫 invoice_status_history」的呼叫名單。
//
// ⚠️ 這份名單同時被 writesHistory 與 writesSubject 使用（**同一個 slice，不是兩份拷貝**）——
// 新增一支寫入 helper 只要加在這裡，兩處都會認得。反過來說，漏加的話兩處會一起失明，
// 而症狀是「明明有寫卻被判成沒寫」（誤報，會被人當成 gate 壞掉而繞過去）。
//
// ⚠️ 名單比對是子字串：`appendStatusHistory(` **不會**命中 `appendStatusHistoryActor(`
// （中間隔著 Actor），所以後者必須自己列一行。
// i7StripComments 把 Go 註解從函式本體裡拿掉。
//
// 🔴 為什麼非有不可：本判準是**掃原始碼字串**的，而註解裡提到某個呼叫名是很正常的事
// （這份檔案自己就充滿這種說明）。不剝註解的後果 2026-09-05 當場發生過——
// 我在 `admin_complaint.go` 寫了一段解釋「判準認不得 `AppendStatusHistory(`」的註解，
// 判準就命中了那段**註解**、把它當成真的呼叫，`writesSubject` 取第一次命中之後
// 往後 200 字元找 subject 常數，找到的是我的中文說明 ⇒ 誤判成沒寫。
// 語言學上這是 use／mention 之分：被**提到**的 token 不是被**說出**的 token。
//
// 🔴 `//` 要贏過 `/*`：一行裡先出現 `//` 的話，後面的 `/*` 只是註解內容，
// 不會開啟一個區塊。反過來寫會讓 `// 這裡用 /* 舉例` 之後的整段程式碼被吃掉
// （`.claude/rules/06-no-small-tails.md` §3.10 記過同型事故，一支 ADM 腳本因此吃掉 60% 檔案）。
//
// 剝過頭只會造成**漏認**（判準報「沒留痕」而其實有），那是大聲的失敗；
// 剝不夠才會造成靜默的誤放。方向刻意選前者。
func i7StripComments(src string) string {
	// 逐行收集再 Join：直接 WriteString("\n") 會在結尾多一個換行
	// （Split 對結尾是 "\n" 的字串會產生一個空的最後元素）。
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
				b.WriteString(line[:lc]) // 行註解：`//` 之後整行不要
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

// i7HistCalls 是**終端寫入者**的呼叫名單——真的會產生一列 invoice_status_history 的那些。
//
// 🔴 這裡**只列終端**，不列 `recordTransition`／`recordClaimTransition`／
// `recordCorrectionTransition`／`appendHistory` 這類同 package 的中介 helper。
// 理由是 2026-09-05 抓到的一個真實假綠：名單裡有 `recordTransition(` 時，判準驗的是
// **名字**不是行為 —— 把那支 helper 的本體掏空，`applyDecision` 照樣被判成「有留痕」，
// 兩次變異都不轉紅。中介 helper 交給 writesHistory 的遞迴（深度 2）去追它自己的本體，
// 掏空就會轉紅。
//
// ⚠️ `AppendStatusHistory(`（**大寫 A**，直接呼叫 repository）必須列。漏了它的後果不是
// 漏報而是**誤報**：判準把一個已經有留痕的函式判成「沒有」，而照著錯誤指引修，結果是
// 同一次 status 轉移寫兩列（dev 實證：ADM 裁決一次、history 兩列，上面那列 actor 還是錯的）。
//
// ⚠️ 名單比對是子字串：`appendStatusHistory(` **不會**命中 `appendStatusHistoryActor(`
// （中間隔著 Actor），所以後者必須自己列一行。
var i7HistCalls = []string{
	"appendStatusHistory(", "appendStatusHistoryDetail(", "appendStatusHistoryActor(",
	"AppendStatusHistory(",
}

// i7SubjectOf 回一個 repository 方法所屬的 aggregate（invoice_status_history.subject_type）。
//
// 🔴 **這一格是 2026-09-04 補的，補之前判準有一個真實的盲點**：原本只檢查
// 「函式或它的呼叫者有沒有呼叫 appendStatusHistory」，而沒有檢查**寫的是哪個 subject**。
// `correction.Execute` 的外層確實有呼叫——但那是寫 DOCUMENT 軸的
// （ReviseSnapshot 那一段），CORRECTION_REQUEST 軸一列都沒有。
// dev 實證：一筆剛走完 R03 的 correction，`subject_type='CORRECTION_REQUEST'`
// 的歷程是 **0 列**，而判準卻放行了它。
//
// 判準因此改成：**改了哪個 aggregate 的 status，就要有寫那個 subject 的呼叫**。
// 對應不到 aggregate 的方法回空字串，退回舊判準（只要有寫任何 history 就算）。
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

// i7SubjectAliases 記同一個 subject_type 值的其他常數名。
//
// 判準錨在常數**名字**（原始碼），而值相同的常數可以有很多個名字——這正是
// 「錨在名字」的固有代價。列在這裡而不是改成解析常數值：後者要跑型別檢查，
// 成本遠大於維護這張三行的表。
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

// writesHistory 判斷 name 自己或它委派的函式（深度 2）有沒有寫歷程。
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
			// 🔴 錨在**欄位賦值**而不是裸字串（2026-09-04 實測踩到）：
			// 用 Contains("ProviderMessage") 會命中寫在同一個 literal 附近的
			// **註解**，於是拿掉真正的賦值之後測試照樣綠——判準被自己的說明文字騙過。
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

// reProviderMsgField 只認**欄位賦值**（行首空白 ＋ ProviderMessage:），
// 不認註解裡提到的同名字串。
var reProviderMsgField = regexp.MustCompile(`(?m)^\s*ProviderMessage:\s*\S`)

// writesSubject 判斷 name 自己或它委派的函式有沒有寫**指定 subject** 的歷程。
//
// 判定材料是原始碼裡的 subject 常數名（subjectComplaint／subjectCorrectionRequest…），
// 而不是字串字面值——本 repo 禁硬編碼，字面值 grep 會系統性失明
// （memory `reference_grep_blind_on_param_injection` 的同一個形狀）。
func writesSubject(name string, funcs []i7Func, subject string, depth int) bool {
	for _, f := range funcs {
		if f.name != name {
			continue
		}
		for _, h := range i7HistCalls {
			// 🔴 掃**全部**出現位置而不是第一個：`strings.Index` 只回第一次命中，
			// 一個函式若先寫了別的軸、後面才寫這一軸，就會被判成沒寫這一軸。
			for off := 0; ; {
				rel := strings.Index(f.body[off:], h)
				if rel < 0 {
					break
				}
				idx := off + rel
				off = idx + len(h)
				// 呼叫點之後 200 字元內出現 subject 常數 ⇒ 算它寫的是那一軸。
				end := idx + 200
				if end > len(f.body) {
					end = len(f.body)
				}
				if strings.Contains(f.body[idx:end], subject) {
					return true
				}
				// 同一個 subject_type 值在本 package 有兩個常數名
				// （`subjectComplaint` 與 `complaintResourceType` 都是 "COMPLAINT"）。
				// 只認其中一個 ⇒ 用另一個名字寫的留痕會被判成「寫的是別的軸」。
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
	// 函式名逐一從呼叫端列出來（`awk '/^func /{fn=$0} /appendStatusHistoryActor\(/{print fn}'`），
	// **不憑記憶拼**——第一版寫成 Reopen／Withdraw／applyAction，三個都不存在於這一層
	// （買家軸共用 transition、ADM 軸在 Act 而不是它委派的 applyAction）。
	want := []struct {
		fn    string // usecase 函式名
		actor string // 呼叫點附近必須出現的 actor 常數
	}{
		{"Respond", "complaintActorSeller"},   // seller_complaint.go：賣家回覆（K02／K03）
		{"transition", "complaintActorBuyer"}, // complaint.go：買家 reject／withdraw／reopen 共用
		{"Act", "jobActorAdmin"},              // admin_complaint.go：ADM 裁決（K09／K10），走直接呼叫
	}
	// 對照組：先證明我找得到這些函式。找不到 ⇒ 判準失明（fail-closed，不是靜默通過）。
	byName := map[string]string{}
	for _, f := range funcs {
		byName[f.name] = f.body
	}
	for _, w := range want {
		body, ok := byName[w.fn]
		if !ok {
			t.Fatalf("找不到函式 %s —— 判準失明（改名了就把這份清單一起改）", w.fn)
		}
		// 兩種合格寫法：帶 actor 的 helper，或直接呼叫 repository 自己填 ActorType。
		// 不合格的只有一種——用把 ActorType 硬寫成 SYSTEM 的那兩支 helper。
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
		want string // 期望硬寫字面值，不由被測函式產生
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
