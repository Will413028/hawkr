package grpc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 「proto 有這一欄、gateway 照實透傳、生產者從來沒填」——本服務重複三次的缺陷類
//
// #3017（store_name／buyer_masked_name）、#3019（paid_at／shipment_status）、
// 以及 #3017 順帶挖出的 A04 店名，全部是同一個形狀：
//
//	欄位在 proto 上存在 → gateway 的 conv 函式照實 GetXxx() → **沒有任何生產者賦值**
//	⇒ 前端拿到空字串，畫面上是「這筆沒有資料」而不是「這個功能沒做」。
//
// 三次都不是被測試抓到的，是**人在 dev 上用眼睛看到的**。原因寫在 #3017 的 body：
// gateway 的契約測試 fixture 是手寫的（自己填了 `StoreName: "測試商店"`），
// 於是驗的是「gateway 會不會透傳」而不是「這個欄位會不會有值」——
// 生產者永遠回空、測試永遠有值，兩邊都綠。
//
// 本測試把「有沒有人填」變成機械判準。
//
// 🔴 **它擋得住什麼、擋不住什麼（§3.10：gate 要誠實標明自己的觀測窗）**
//
//	擋得住：整個 internal/ 沒有任何一處對該欄位賦值 ⇒ 一定是空的
//	擋不住：① 有賦值但賦的是常數空字串（原例 `CaseNo: ""`，已於 #3054 修掉——
//	          它從來不是「設計上恆為空」，58 §5.1.12 早就定義案件編號＝該列 TSID）
//	        ② 有賦值但那條路徑走不到（dead path）
//	        ③ 有賦值但取錯 map／取錯 id（#3021 的第二個賦值點就是這種）
//	        ④ 欄位在 gateway DTO 被漏掉（那是 gateway-json-contract 的守備範圍）
//
// ⇒ 它是**下界不是上界**：綠燈只代表「至少有人填過」，不代表「填對了」。
// 逐欄的正確性仍然要靠各自的行為測試（見 buyer_names_test.go／shipment_status_test.go）。
// ---------------------------------------------------------------------------

// knownEmptyFields 是「確實沒有生產者，而且那是對的」的登記表。
//
// 🔴 **每一筆都必須有理由**，而理由只有兩種合法形態：
//   - 「設計上恆為空」＋ 依據（哪一節、為什麼）
//   - 「KNOWN GAP #NNNN」＋ 票號（＝已經有人要去修）
//
// 不准寫「暫時沒填」「之後再說」——那正是這三次事故的共同前身。
var knownEmptyFields = map[string]string{
	// ⚠️ **「設計上恆為空」那一類不進本表**：它們是 `Foo: ""` 這種**有賦值**的形態，
	// 本 gate 看得到那個賦值 ⇒ 本來就不會被舉報。登記它反而會被「登記表保鮮」那一段
	// 判成殘留（實測：`InvoiceComplaintInfo.case_no` 第一版就這樣紅了）。
	// 本表只收「整個 internal/ 一個賦值都沒有」的欄位。
	//
	// —— 以下五欄：#3031 盤出的 19 欄裡，**填不了而且理由是結構性的**那幾個 ——
	//
	// ⚠️ #3031 已於 2026-09-03 關閉（14 欄已填並在 dev 實機驗到有值）。這五筆
	// **不是待辦**，是「設計上恆為空」＋依據。每一筆都寫了「要填它得先做什麼」，
	// 那個前置條件全部在本服務之外（accounts 沒有名字欄／稽核表沒有該欄／
	// 條款全文未發布／沒有 coupon client）。
	// 本表目前**零筆 KNOWN GAP 形式的登記**——有票號的那一類已全部消化完。
	// result／terms_version：invoice_admin_audit_logs **沒有對應欄**（逐欄查過）。
	// 🔴 不猜、不填 "SUCCESS"：那會讓失敗的操作在稽核表上看起來都成功，
	// 而稽核表上最不能說的謊就是這個。terms_version 同理——它只對「條款相關」
	// 那幾種 action 有意義，填一個當期版本會讓每一列都像簽過同一版。
	// 要真的填得出來得先在該表加欄並由**寫入端**填，不是查詢端能補的。
	// actor_display_name：**平台沒有任何地方存得住帳號的人名**。
	// accounts 表只有 account_id／email／phone／status／account_type／時間戳，
	// 一個名字欄都沒有；account-service 的 GetAccountInfoBatch 雖然回一個 nickname 欄，
	// 但 usecase 的 toAccountInfo **從來不填它**（只有 Google OAuth 那條路徑會設，
	// 而那是另一個 DTO）⇒ 那支 RPC 對批次查詢結構性恆回空字串。
	//
	// ⚠️ 2026-09-02 曾接上那支 RPC（含 client／netpol／env），dev 實測後撤回：
	// 它每一次都回空字串，等於加了一個永遠沒有值的跨服務依賴。**那正是本 gate
	// 第一種擋不住的形態**（有賦值、但值恆為空）——gate 會轉綠而畫面沒有任何改變。
	// 🔴 不得改用 email 頂替：那是把 PII 放進一張保存 5 年的稽核表。
	// 要真的填它得先讓 accounts 有名字欄，那不是本服務能補的。
	"InvoiceAuditLogItem.actor_display_name": "設計上恆為空：accounts 表沒有任何名字欄，GetAccountInfoBatch 的 nickname 在批次路徑上結構性恆空；不得用 email 頂替",
	"InvoiceAuditLogItem.result":             "設計上恆為空：invoice_admin_audit_logs 無此欄；填 SUCCESS 會讓失敗的操作看起來都成功",
	"InvoiceAuditLogItem.terms_version":      "設計上恆為空：invoice_admin_audit_logs 無此欄；只對條款相關 action 有意義，猜當期版本會讓每一列都像簽過同一版",
	// terms_key ＝「條款全文的 media objectKey」，而**那份文件目前不存在於系統裡**：
	// 條款版本只是 usecase 的一個日期常數（SelfIssueTermsVersion），docs/legal/ 也還沒建。
	// 🔴 不得拿一個猜的 key 填：前端會拿它去組下載連結，而那條連結會 404。
	// 要填它得先把條款全文發布成一份 media 資產——那是內容作業，不是這一層補得出來的。
	"SelfIssueAttestationInfo.terms_key": "設計上恆為空：條款全文尚未發布成 media 資產；猜一個 key 會產生 404 的下載連結",
	// created_by 的來源是 **coupon 的 created_by**，而本服務沒有 coupon client
	// ——與同一個 message 上的 funded_by 同一個依賴。funded_by 已經用 UNDETERMINED
	// 把「不知道」如實說出來（E4 fail-closed，§3.7 D9a），而 created_by 的值域只有
	// SELLER／ADMIN、沒有第三個值可以誠實地填 ⇒ 空字串是這一欄目前唯一不說謊的輸出。
	// 🔴 **不得猜一個 SELLER**：proto 逐字寫「不得寫成常數」，而 fail-closed 的那一格
	// 正是它要的常數。要真的填它得先有 coupon client，屆時 funded_by 也一起改。
	"SellerInvoiceDocumentDetail.created_by": "設計上恆為空：來源是 coupon 的 created_by，本服務無 coupon client；同 message 的 funded_by 已用 UNDETERMINED 承擔 fail-closed（§3.7 D9a）",

	// —— 收窄「複合字面量必須綁型別」之後才浮出來的 6 欄（同一批，同一張票）——
	// 它們原本被別的 struct 的同名欄位遮住 ⇒ 這 6 欄是**這道 gate 自己的假陰性**
	// 被修掉之後才看得見的，正是「假陰性比沒有 gate 更糟」的實例。

	// ⚠️ 曾經記在這裡的假陰性（`TaxDocumentInfo.file_name` 從未被填、卻因為
	// `usecase/document_file.go` 有一句 `res.FileName = name` 而讓本 gate 綠）
	// 已於 2026-09-02 一併修掉：toTaxDocuments 現在真的填它。
	//
	// 🔴 **2026-09-04 回灌（#3089 第 7 項）**：上一行原本寫著「指派形態全檔比對的
	// 殘留假陰性仍在，只是目前沒有已知的受害欄位」——**那句話當時就已經不成立**。
	// 第 3 輪 E2E 用黑箱打出兩個活的受害欄位（`AdminInvoiceJobDetail.shipment_status`
	// 與 `ProviderExchangeItem.provider_message` 恆空而本 gate 綠），本輪重查又多兩個
	// （`SellerProviderAccountData.consent_version`、`TaxDocumentInfo.kind`），
	// 而且成因不只指派形態一種，是三種：註解被當程式碼、指派不綁型別、
	// 複合字面量的巢狀內文。三者已於本輪一併換成 AST 判定（見 producerIndex）。
	// **教訓**：「殘留假陰性目前沒有受害欄位」這句話是一個**沒有被查證過的宣稱**——
	// 它需要對照組（黑箱實打），而不是靠「實務上生產者幾乎都用複合字面量」的推理。

	// kind ＝ 稅務證明文件的業務分類（TAX_REGISTRATION／EXEMPTION_CERT／
	// ID_DOCUMENT／OTHER）。🔴 **平台從來沒有收集過它**：上傳端（proto 的
	// `evidence_object_keys` ／ `evidence_staged_tokens`）只收 media objectKey 的
	// **純字串陣列**，`seller_tax_profiles.evidence_object_keys` 也只存那個陣列，
	// media 的 BatchGetAssetMetadata 回的是 assetID／fileName／sizeBytes／uploadedAt
	// ——一路上沒有任何一格存得住業務分類。
	// 🔴 **不得填 `OTHER` 頂替**：那個值的語意是「賣家選了『其他』」，而事實是
	// 「沒有人問過賣家」。ADM 審件時會據此以為賣家分過類。與同一份表上
	// `SellerInvoiceDocumentDetail.created_by` 同一個推理：值域裡沒有一個值
	// 能誠實表達「不知道」時，空字串是唯一不說謊的輸出。
	// 要真的填它得先讓上傳端逐檔收分類（proto 從 `repeated string` 改成
	// `repeated message`），那是產品範圍的新功能，不是查詢層補得出來的。
	// ⚠️ 「恆為空」是對的，它**曾經還有一個附帶代價**：#3149 之前 file_name 也只是
	// object key 的 basename（media 從不儲存原始檔名），ADM 面對同一賣家的多份稅務文件時
	// 兩個欄位都給不出資訊、只能逐一點開。
	// ✅ **#3149 已落地**（media-service 的 upload_session 與 media_asset 各加
	// original_filename，V9；讀取側統一走 AssetFilename.resolveDisplayName）⇒ file_name
	// 現在是使用者上傳當下的原始檔名；只有 V9 之前建立、原始檔名無從回填的舊資產
	// 才仍回退為 object key 尾段（剝去 .attempt-{uuid} 後綴）。
	// 🔴 **但本欄仍然恆為空**：檔名與業務分類是兩件事，#3149 沒有讓上傳端開始收分類。
	"TaxDocumentInfo.kind": "設計上恆為空：上傳端只收 objectKey 純字串陣列，DB 與 media metadata 皆無分類欄；填 OTHER 會讓 ADM 以為賣家分過類",
}

func TestEveryResponseFieldHasAProducer(t *testing.T) {
	protoPath := filepath.Join("..", "..", "..", "..", "api", "proto", "invoice", "invoice.proto")
	raw, err := os.ReadFile(protoPath) //nolint:gosec // 固定的 repo 內相對路徑
	if err != nil {
		t.Fatalf("讀不到 proto（本測試已經在守一個不存在的東西）：%v", err)
	}
	proto := string(raw)

	messages := parseMessages(proto)
	if len(messages) == 0 {
		t.Fatal("一個 message 都沒解到 —— proto 格式變了，本測試已失效")
	}
	reachable := reachableFromRPCOutputs(proto, messages)
	if len(reachable) < 50 {
		t.Fatalf("只走訪到 %d 個 response message，遠少於預期 —— 解析壞了，"+
			"而壞掉的症狀是「什麼都沒查到」＝ 全綠", len(reachable))
	}

	idx := buildProducerIndex(t)

	var offenders []string
	var undecidable []string
	used := map[string]bool{}
	for _, msg := range sortedKeys(reachable) {
		for _, f := range protoFields(messages[msg]) {
			key := msg + "." + f
			goField := goFieldName(f)
			produced := idx.has(msg, goField)
			_, registered := knownEmptyFields[key]
			// 🔴 已登記的欄位不進 undecidable：登記本身就是「人已經看過並下了結論」，
			// 再報一次會讓登記表變成無法消音的紅燈。
			undecided := !produced && !registered && idx.unresolvedAssign[goField]
			if undecided {
				// 接收者型別解析不出來的指派（`a.b.Field =`／`out[i].Field =`／
				// 變數宣告在本函式之外）。**不當成生產者**（§3.10：看不到就要報錯，
				// 不是看不到就放行），但單獨列出來——它需要人看一眼，而不是被靜默放行。
				undecidable = append(undecidable, key)
			}
			if registered {
				used[key] = true
				// 🔴 登記過的欄位**仍然要查有沒有生產者**。第一版在這裡就 continue，
				// 於是「有人把它填上了、卻忘了移除登記」完全查不出來——而那正是
				// 登記表最常見的腐化方式（欄位被刪掉反而少見）。變異驗證抓到的。
				if produced {
					offenders = append(offenders,
						key+"（已登記為「沒有生產者」，但實際上有人填了 ⇒ 移除該登記）")
				}
				continue
			}
			if produced || undecided {
				// undecided 已經在 undecidable 裡報過了——同一件事列兩次會讓讀的人
				// 以為是兩個獨立問題。
				continue
			}
			offenders = append(offenders, key)
		}
	}

	if len(offenders) > 0 {
		t.Errorf("以下 response 欄位在 internal/ 內**沒有任何賦值**⇒ 前端一定拿到零值。\n"+
			"這是 #3017／#3019 的缺陷類：畫面上看起來是「這筆沒有資料」，實際是「這個功能沒做」。\n"+
			"修法二選一：把它填上，或登記進 knownEmptyFields 並寫下理由（設計上恆為空／KNOWN GAP #票號）。\n  %s",
			strings.Join(offenders, "\n  "))
	}

	if len(undecidable) > 0 {
		t.Errorf("以下 response 欄位**只**被「接收者型別無法靜態解析」的指派命中"+
			"（`a.b.Field =`／`out[i].Field =`／宣告在本函式之外）⇒ 本 gate 無法判定它有沒有生產者。\n"+
			"看不到就要報錯，不是看不到就放行。修法三選一：改成複合字面量、把接收者的宣告移進同一函式、"+
			"或確認它確實沒有生產者後登記進 knownEmptyFields。\n  %s",
			strings.Join(undecidable, "\n  "))
	}

	// 🔴 登記表也要保鮮：欄位被刪掉或補上之後，殘留的條目會讓下一個人以為那還是缺口。
	for key := range knownEmptyFields {
		if !used[key] {
			t.Errorf("knownEmptyFields 的 %q 已經不成立（欄位被刪掉、或已經有人填了）⇒ 移除該條目", key)
		}
	}
}

var (
	reMessage = regexp.MustCompile(`(?ms)^message\s+(\w+)\s*\{(.*?)^\}`)
	reRPCOut  = regexp.MustCompile(`rpc\s+\w+\s*\([^)]*\)\s*returns\s*\(\s*([\w.]+)\s*\)`)
	reField   = regexp.MustCompile(`^(?:repeated\s+|optional\s+)?(?:map<[^>]+>|[\w.]+)\s+(\w+)\s*=\s*\d+\s*;`)
	reFType   = regexp.MustCompile(`^(?:repeated\s+|optional\s+)?([\w.]+)\s+\w+\s*=\s*\d+\s*;`)
	reMapVal  = regexp.MustCompile(`^map<[^,]+,\s*([\w.]+)>\s+\w+\s*=`)
)

func parseMessages(proto string) map[string]string {
	out := map[string]string{}
	for _, m := range reMessage.FindAllStringSubmatch(proto, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// stripComment 去掉行尾註解。**不做這一步，欄位名會被註解裡的字污染**。
func stripComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

func protoFields(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if m := reField.FindStringSubmatch(stripComment(line)); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

func nestedTypes(body string, messages map[string]string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		l := stripComment(line)
		if m := reMapVal.FindStringSubmatch(l); m != nil {
			if _, ok := messages[m[1]]; ok {
				out = append(out, m[1])
			}
			continue
		}
		if m := reFType.FindStringSubmatch(l); m != nil {
			if _, ok := messages[m[1]]; ok {
				out = append(out, m[1])
			}
		}
	}
	return out
}

func reachableFromRPCOutputs(proto string, messages map[string]string) map[string]bool {
	seen := map[string]bool{}
	var stack []string
	for _, m := range reRPCOut.FindAllStringSubmatch(proto, -1) {
		stack = append(stack, m[1])
	}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		body, ok := messages[n]
		if !ok {
			continue
		}
		seen[n] = true
		stack = append(stack, nestedTypes(body, messages)...)
	}
	return seen
}

// goFieldName 把 proto 的 snake_case 折成 protoc-gen-go 的欄位名。
//
// ⚠️ protoc-gen-go 用的是**單純的 CamelCase**（`last_request_id` → `LastRequestId`），
// 不是 Go 慣用的 `LastRequestID`。寫錯的話每個含 id/url 的欄位都會被誤報成缺口。
func goFieldName(snake string) string {
	parts := strings.Split(snake, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

// producerIndex 是「哪個訊息型別的哪個欄位被賦過值」的索引，由 **AST** 建立。
//
// 🔴 **為什麼從 regex 換成 AST**（#3089 第 7 項：這道 gate 自己有活的假陰性）
//
// 舊版用兩條 regex 掃原始碼字串，有三個各自獨立的假陰性來源，實測各有受害欄位：
//
//	① **註解被當成程式碼** —— `AdminInvoiceJobDetail.shipment_status` 全服務零賦值，
//	   卻因為 `grpc/shipment_status.go` 的**註解**裡寫著一句
//	   `d.ShipmentStatus = m.GetShipmentStatus()` 而讓 gate 綠。
//	② **指派形態不綁型別** —— `\.Field\s*=` 是全檔比對，任何一個別的型別有同名欄位
//	   就足以讓它綠。受害欄位 `ProviderExchangeItem.provider_message`：唯一命中是
//	   `usecase/recheck.go` 對**另一個**型別的 `out.ProviderMessage = &msg`。
//	③ **複合字面量的巢狀內文** —— 舊版取 `Foo{ … }` 的整段大括號內文（含巢狀），
//	   於是 `pb.Foo{Bar: pb.Baz{Field: v}}` 會讓 `Foo.Field` 誤判成有生產者。
//
// AST 版對三者天然免疫：註解不是 AST 節點；複合字面量只看**直屬** key；
// 指派的接收者型別在同一函式內靜態解析，解析不出來就**不算生產者**（見 unresolvedAssign）。
//
// 🔴 **它仍然擋不住什麼**（§3.10：gate 要誠實標明自己的觀測窗，不得被當成牆）：
//
//   - 有賦值但賦的是常數零值（`Foo: ""`）
//   - 有賦值但那條路徑走不到（dead path）
//   - 有賦值但取錯 map／取錯 id
//   - 跨函式的間接賦值（`fill(&msg)` 而 fill 收的是介面）
//
// ⇒ 它是**下界不是上界**：綠燈只代表「至少有人填過」，不代表「填對了」。
type producerIndex struct {
	// produced 的 key 是 "MsgType.GoField"。
	produced map[string]bool
	// unresolvedAssign 收「接收者型別解析不出來」的指派欄位名（`a.b.Field =`／
	// `out[i].Field =`／宣告在本函式之外）。**它不是生產者證據**，只是「這裡有一筆
	// 我看不懂的東西」——主測試會把只被它命中的欄位單獨報出來要人看一眼。
	unresolvedAssign map[string]bool
}

func (idx producerIndex) has(msgType, goField string) bool {
	return idx.produced[msgType+"."+goField]
}

// buildProducerIndex 解析 internal/ 底下所有非測試 Go 檔。
//
// 🔴 **排除 _test.go 是本測試的關鍵**：測試檔裡的手寫 fixture 正是讓這個缺陷類
// 三次都沒被抓到的原因（#3017 的 body 逐字寫了這件事）。把測試檔算進來，
// 這道 gate 會被它要抓的東西騙過去。
//
// 🔴 **parse 失敗一律 Fatal，不得跳過**：跳過的症狀是「那個檔案裡的賦值全部消失」
// ＝ 一批欄位突然變成缺口（全紅，還算看得見），但若跳過的是**唯一**的生產者檔，
// 就變成靜默誤報。兩種都不能靠運氣。
func buildProducerIndex(t *testing.T) producerIndex {
	t.Helper()
	idx := producerIndex{produced: map[string]bool{}, unresolvedAssign: map[string]bool{}}
	fset := token.NewFileSet()
	var files []*ast.File
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("解析 %s 失敗（本 gate 對這個檔完全失明）：%v", path, perr)
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		t.Fatalf("走訪 internal/ 失敗：%v", err)
	}
	if len(files) < 30 {
		t.Fatalf("只讀到 %d 個 .go 檔，遠少於預期 —— 走訪範圍壞了，"+
			"而壞掉的症狀是「每個欄位都查不到賦值」＝ 全紅，或路徑錯掉時全綠", len(files))
	}

	// 🔴 **兩趟不是一趟**：`row := s.toTaxReviewItem(item)` 之後的 `row.StoreName = …`
	// 要先知道那支 mapper 回什麼型別才判得出來。第一版只有一趟，於是把一個
	// **確實有填**的欄位（`TaxReviewItem.store_name`）報成缺口——假陽性會讓下一個人
	// 學會忽略這道 gate，那比假陰性更難救（§2.1.2：假的保證排擠真的警覺）。
	returns := collectFuncReturnTypes(files)
	for _, f := range files {
		collectCompositeLiterals(f, idx.produced)
		collectAssignments(f, returns, idx.produced, idx.unresolvedAssign)
	}
	return idx
}

// collectCompositeLiterals 蒐集 `pb.Foo{Field: v}` 形態，只取**直屬** key。
func collectCompositeLiterals(f *ast.File, produced map[string]bool) {
	ast.Inspect(f, func(node ast.Node) bool {
		cl, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		walkCompositeLit(cl, typeNameOf(cl.Type), produced)
		return true
	})
}

// walkCompositeLit 處理 `[]Foo{{A: 1}}` 這種內層無型別的元素：型別由容器的元素型別給。
func walkCompositeLit(cl *ast.CompositeLit, typeHint string, produced map[string]bool) {
	elemHint := ""
	switch tt := cl.Type.(type) {
	case *ast.ArrayType:
		elemHint = typeNameOf(tt.Elt)
	case *ast.MapType:
		elemHint = typeNameOf(tt.Value)
	}
	self := typeNameOf(cl.Type)
	if self == "" {
		self = typeHint
	}
	for _, e := range cl.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok && self != "" {
				produced[self+"."+id.Name] = true
			}
			if inner, ok := kv.Value.(*ast.CompositeLit); ok && inner.Type == nil && elemHint != "" {
				walkCompositeLit(inner, elemHint, produced)
			}
			continue
		}
		if inner, ok := e.(*ast.CompositeLit); ok && inner.Type == nil && elemHint != "" {
			walkCompositeLit(inner, elemHint, produced)
		}
	}
}

// collectAssignments 蒐集 `x.Field = v`，接收者型別在同一函式內靜態解析。
func collectAssignments(f *ast.File, returns map[string]string, produced, unresolved map[string]bool) {
	ast.Inspect(f, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		local := localVarTypes(fn, returns)
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			as, ok := m.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range as.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok {
					// `a.b.Field =`／`out[i].Field =` —— 接收者不是單純的識別字。
					unresolved[sel.Sel.Name] = true
					continue
				}
				if typ, ok := local[id.Name]; ok {
					produced[typ+"."+sel.Sel.Name] = true
					continue
				}
				unresolved[sel.Sel.Name] = true
			}
			return true
		})
		return true
	})
}

// localVarTypes 蒐集函式內可靜態解析的具名型別：receiver、參數、var 宣告、
// `:=` 後接複合字面量／取址／new()。**解析不出來就不收**——寧可回報「無法判定」，
// 不可猜一個型別（猜錯的方向是靜默放行，那正是本次要修的假陰性）。
func localVarTypes(fn *ast.FuncDecl, returns map[string]string) map[string]string {
	out := map[string]string{}
	add := func(name string, e ast.Expr) {
		if name == "" || name == "_" {
			return
		}
		if t := typeNameOf(e); t != "" {
			out[name] = t
		}
	}
	addRHS := func(name string, e ast.Expr) {
		if name == "" || name == "_" {
			return
		}
		if te := rhsTypeExpr(e); te != nil {
			add(name, te)
			return
		}
		// `x := s.toFoo(...)` —— 型別來自被呼叫函式的回傳值。
		if t := callReturnType(e, returns); t != "" {
			out[name] = t
		}
	}
	for _, fl := range []*ast.FieldList{fn.Recv, fn.Type.Params} {
		if fl == nil {
			continue
		}
		for _, fld := range fl.List {
			for _, nm := range fld.Names {
				add(nm.Name, fld.Type)
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.DeclStmt:
			gd, ok := v.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, nm := range vs.Names {
					if vs.Type != nil {
						add(nm.Name, vs.Type)
					} else if i < len(vs.Values) {
						addRHS(nm.Name, vs.Values[i])
					}
				}
			}
		case *ast.AssignStmt:
			if v.Tok != token.DEFINE {
				return true
			}
			for i, l := range v.Lhs {
				id, ok := l.(*ast.Ident)
				if !ok || i >= len(v.Rhs) {
					continue
				}
				addRHS(id.Name, v.Rhs[i])
			}
		}
		return true
	})
	return out
}

// collectFuncReturnTypes 建「函式／方法名 → 回傳的具名型別」表。
//
// 只取名字的最後一段（忽略接收者），因為呼叫端寫的是 `s.toFoo(...)` 或 `toFoo(...)`。
// 🔴 **同名而回傳型別不同時一律設為空**（＝無法解析）：猜錯的方向是把一個沒有生產者的
// 欄位判成有，那正是本次要修的假陰性。寧可回報「無法判定」讓人看一眼。
func collectFuncReturnTypes(files []*ast.File) map[string]string {
	out := map[string]string{}
	conflict := map[string]bool{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
				continue
			}
			t := typeNameOf(fn.Type.Results.List[0].Type)
			if t == "" {
				continue
			}
			if prev, seen := out[fn.Name.Name]; seen && prev != t {
				conflict[fn.Name.Name] = true
				continue
			}
			out[fn.Name.Name] = t
		}
	}
	for name := range conflict {
		delete(out, name)
	}
	return out
}

// callReturnType 解析 `s.toFoo(...)`／`toFoo(...)` 的回傳型別。
func callReturnType(e ast.Expr, returns map[string]string) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	var name string
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name = fun.Name
	case *ast.SelectorExpr:
		name = fun.Sel.Name
	default:
		return ""
	}
	return returns[name]
}

func rhsTypeExpr(e ast.Expr) ast.Expr {
	switch v := e.(type) {
	case *ast.UnaryExpr:
		if v.Op == token.AND {
			return rhsTypeExpr(v.X)
		}
	case *ast.CompositeLit:
		return v.Type
	case *ast.CallExpr:
		if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "new" && len(v.Args) == 1 {
			return v.Args[0]
		}
	}
	return nil
}

// typeNameOf 取具名型別的最後一段（`pb.Foo` → `Foo`，`*pb.Foo` → `Foo`）。
// 只取最後一段是刻意的：proto message 名在各處的 import 別名不同（pb／invoicepb／…），
// 綁別名會讓 gate 隨 import 寫法失明。
func typeNameOf(e ast.Expr) string {
	switch v := e.(type) {
	case nil:
		return ""
	case *ast.StarExpr:
		return typeNameOf(v.X)
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.Ident:
		return v.Name
	}
	return ""
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
