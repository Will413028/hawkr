package handler

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 三層鎖定的路由禁字斷言只掃**路徑**（§6.1.1 的 10 個禁字）。DTO 層漏掉一個履約或
// 金流欄位不會 panic、不會有測試轉紅，只會靜默把託運單資訊、金流交易明細或未遮罩的
// 收件資訊交給代行者——20 號 #8d 明講這是該條最容易出錯的一項。
//
// 這支測試就是那道缺的鎖：以**欄位名**為錨掃過每一個代行 DTO 的完整結構（含巢狀），
// 命中禁列即失敗。
//
// 為什麼是欄位名而不是型別：投影是人手寫的，出錯的形狀是「有人從上游 message 多抄了
// 一欄過來」，而那一欄會沿用上游的名字。

// delegatedForbiddenFields 逐項對應 §8.1.5「明確排除」欄與三層鎖定的履約禁區。
// 比對前一律轉小寫，所以 labelKey / label_key / LabelURL 全部命中同一條。
var delegatedForbiddenFields = []string{
	// 履約物件（託運單、出貨便、取件門市、出貨與收貨照片）
	"shipmentno", "labelkey", "labelurl", "labelexpireat",
	"odno", "validationno", "partnerid", "pickupstorename",
	"shipmentphotokeys", "receiptphotokeys", "cvsstoreid",
	// 金流
	"transactionid", "paymentmerid", "atmbankcode", "atmvirtualaccount",
	"atmexpiredate", "providerstatus", "providermessage", "availablepaymentmethods",
	// 退款與撥款
	"refundhistory", "needrefundpage",
	// 買家側行為與社群互動計數
	"review", "isreviewed", "canreview", "reviewdeadline",
	"likecount", "commentcount", "viewcount", "sharecount", "isliked",
	// 帳號與治理內部狀態（storefront allow-list 的排除項）
	"seotitle", "seodescription", "birthday", "gender", "foodbusinessregno",
	"email", "avatarkey", "avatarurl", "lifecyclestatus", "suspensionreasoncode",
	"overallcertificationstatus", "currentcertificationid",
	// 完整收件資訊只走 recipient-contact 端點，其他 DTO 一律不得出現
	"recipientphone", "shippingaddress",
	// 媒體欄位只回 objectKey（*Key）；URL 由前端組
	"imageurl", "imageurls", "logourl", "coverurl", "previewurl",
}

// delegatedForbiddenFieldExceptions 是上面那份清單的具名例外，鍵為 `DTO.欄位名`。
//
// 例外要逐個具名、不得以「某個 DTO 整個豁免」的形式存在：後者會讓那個 DTO 日後長出
// 任何一個禁用欄位都不再被守到。目前只有一條 ——
//
// DelegatedMediaCompleteData.previewUrl：`previewurl` 進禁列的理由是「資源 DTO 的媒體
// 欄位只回 objectKey，URL 由前端組」，而這支不是資源 DTO，是三步上傳第 ③ 步的回執
// （56 §6.1.13 的步驟表明載回傳 `stagedToken` ＋ `previewUrl`）。賣家自助那支
// （handler/media.go CompleteUploadSession）一直都回這兩欄，而代行工作區與賣家共用同一
// 支瀏覽器端上傳函式（website lib/media/upload.ts），它在任一欄缺席時 fail-fast ⇒ 少了
// 它，代行的每一次圖片上傳都在位元組已經進 R2 之後才失敗。
var delegatedForbiddenFieldExceptions = map[string]struct{}{
	"DelegatedMediaCompleteData.previewUrl": {},
}

// delegatedResponseDTOs 是四類資源端點與稽核端點對外吐出的每一個 struct。
// 新增一個代行 DTO 卻忘了列進來，這支測試就守不到它——所以清單本身也是契約的一部分。
func delegatedResponseDTOs() map[string]any {
	return map[string]any{
		"DelegatedProductListItem":        DelegatedProductListItem{},
		"DelegatedProductDetail":          DelegatedProductDetail{},
		"DelegatedPostListItem":           DelegatedPostListItem{},
		"DelegatedPostDetail":             DelegatedPostDetail{},
		"DelegatedOrderListItem":          DelegatedOrderListItem{},
		"DelegatedOrderDetail":            DelegatedOrderDetail{},
		"DelegatedAuditLogEntry":          DelegatedAuditLogEntry{},
		"DelegatedSessionSummary":         DelegatedSessionSummary{},
		"DelegatedOperationSessionRecord": DelegatedOperationSessionRecord{},
		"DelegatedBatchUnpublishData":     DelegatedBatchUnpublishData{},
		"DelegatedMediaUploadSessionData": DelegatedMediaUploadSessionData{},
		"DelegatedMediaCompleteData":      DelegatedMediaCompleteData{},
		"DelegatedRecipientContactData":   DelegatedRecipientContactData{},
		// 每一支寫入端點的統一出口。它一度不在清單內，而它正是所有代行寫入
		// 回應的形狀 —— 漏掉它等於整條寫入路徑沒有被這支契約守到。
		"delegatedMutationData": delegatedMutationData{},
		// 代行商品建立／更新的專屬出口（共用三欄 ＋ certificationWarnings，19 §9.5）。
		// 它是 delegatedMutationData 的旁支而不是取代品，所以兩個都要在清單裡——
		// 少列這一個，商品寫入路徑就從禁用欄位契約裡整條掉出去。
		"delegatedProductMutationData": delegatedProductMutationData{},
	}
}

func TestDelegatedDTOs_CarryNoFulfilmentOrFinanceFields(t *testing.T) {
	forbidden := map[string]struct{}{}
	for _, f := range delegatedForbiddenFields {
		forbidden[f] = struct{}{}
	}

	usedExceptions := map[string]bool{}
	for name, dto := range delegatedResponseDTOs() {
		for _, field := range collectJSONFieldNames(reflect.TypeOf(dto), map[reflect.Type]bool{}) {
			if _, bad := forbidden[strings.ToLower(field)]; !bad {
				continue
			}
			key := name + "." + field
			if _, allowed := delegatedForbiddenFieldExceptions[key]; allowed {
				usedExceptions[key] = true
				continue
			}
			t.Errorf("%s 含禁用欄位 %q —— 路徑禁字斷言只掃路徑，DTO 層漏掉它不會 panic、"+
				"不會轉紅，只會靜默把履約／金流／未遮罩 PII 交給代行者", name, field)
		}
	}
	// 例外清單本身也要被守：一條不再對應任何欄位的例外會靜默留在那裡，而下一個同名
	// 欄位長出來時它就是一個沒人記得為什麼存在的豁免。
	for key := range delegatedForbiddenFieldExceptions {
		if !usedExceptions[key] {
			t.Errorf("例外 %q 已不對應任何 DTO 欄位，請一併移除", key)
		}
	}
}

// DelegatedMediaCompleteData 是三步上傳第 ③ 步的回執，兩欄缺一不可（56 §6.1.13）。
//
// 這支端點的失敗形狀特別貴：位元組已經 PUT 進 R2、gateway 也回了 200，前端才在
// 「上傳完成回應缺少 previewUrl 或 stagedToken」這一步 fail-fast ⇒ 操作者看到的是一次
// 沒有任何後端錯誤的上傳失敗。賣家自助那支（handler/media.go）一直都回兩欄，代行這支
// 一度只回一欄，兩邊對同一個瀏覽器端函式給出不同形狀的回應。
func TestDelegatedMediaCompleteData_CarriesBothStagedTokenAndPreviewURL(t *testing.T) {
	raw, err := json.Marshal(DelegatedMediaCompleteData{StagedToken: "stg_x", PreviewURL: "seller/2685914682/logo.jpg"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, want := range []string{"stagedToken", "previewUrl"} {
		if _, ok := m[want]; !ok {
			t.Errorf("DelegatedMediaCompleteData 缺少 %q（實際 keys=%v）", want, delegatedSortedKeys(m))
		}
	}
	// 白名單：這支回執只承載這兩樣，不得順手多帶 objectKey 以外的媒體資訊或憑證。
	if len(m) != 2 {
		t.Errorf("DelegatedMediaCompleteData 欄位數 = %d, want 2（實際 keys=%v）", len(m), delegatedSortedKeys(m))
	}
}

func delegatedSortedKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// 反向的鑑別力：唯一允許回未遮罩收件資訊的 DTO 必須確實帶著那些欄位。少了它，
// recipient-contact 端點就只是一支多寫一列稽核卻什麼都不揭露的端點。
func TestDelegatedRecipientContact_IsTheOnlyUnmaskedShape(t *testing.T) {
	fields := collectJSONFieldNames(reflect.TypeOf(DelegatedRecipientContact{}), map[reflect.Type]bool{})
	want := map[string]bool{"name": false, "phone": false, "fullAddress": false, "zipCode": false}
	for _, f := range fields {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for f, seen := range want {
		if !seen {
			t.Errorf("DelegatedRecipientContact 缺少 %q", f)
		}
	}
	// 揭露欄位集合是白名單：不得順手多回 email、身分證、付款資訊。
	if len(fields) != len(want) {
		t.Errorf("DelegatedRecipientContact 欄位數 = %d, want %d（揭露集合是白名單）", len(fields), len(want))
	}
}

// 稽核列的兩個快照欄位沒有任何 API 出口。加回來會同時破壞體積上界（單列 64 KB × 兩欄
// × pageSize 50 ⇒ 最壞 6.4 MB）與「快照白名單 ≠ DTO allow-list」那條禁令。
func TestDelegatedAuditLogEntry_HasNoSnapshotFields(t *testing.T) {
	for _, f := range collectJSONFieldNames(reflect.TypeOf(DelegatedAuditLogEntry{}), map[reflect.Type]bool{}) {
		lower := strings.ToLower(f)
		if strings.Contains(lower, "snapshot") {
			t.Errorf("DelegatedAuditLogEntry 含 %q —— 快照兩欄沒有 API 出口，爭議舉證走維運 SQL", f)
		}
	}
}

// 跨工作階段列表是全案第一支列出**全平台**代行活動的端點：一頁裡的每一列都是不同的
// admin 與不同的店家。因此它的欄位集合必須是白名單，而不是「沒有出現禁字就算過」——
// 上游 message 日後長出任何一欄，投影只要順手多抄一次就流出去了，而那不會轉紅。
//
// 兩個方向都要斷言：欄位集合恰等於白名單（多一欄即紅），以及三個憑證欄位確實不在裡面
// （少了這一半，白名單被人「順手補上」時就沒有東西講得出為什麼不行）。
func TestDelegatedOperationSessionRecord_FieldSetIsAWhitelist(t *testing.T) {
	want := map[string]bool{
		"sessionId": false, "displayCode": false,
		"target": false, "storeId": false, "sellerAccountId": false, "storeName": false,
		"actor": false, "adminId": false, "displayName": false,
		"reason": false, "code": false, "label": false, "caseReferenceRequired": false,
		"caseReference": false, "capabilities": false,
		"issuedAt": false, "expiresAt": false, "endedAt": false, "status": false,
		"revokedAt": false, "revokedByAdminId": false,
		"revokedByAdminDisplayName": false, "revokeReason": false,
	}
	got := collectJSONFieldNames(reflect.TypeOf(DelegatedOperationSessionRecord{}), map[reflect.Type]bool{})
	for _, f := range got {
		if _, ok := want[f]; !ok {
			t.Errorf("DelegatedOperationSessionRecord 多出欄位 %q —— 這支列的是全平台的代行活動，"+
				"欄位集合是白名單，多抄一欄就是跨店流出且不會有東西轉紅", f)
			continue
		}
		want[f] = true
	}
	for f, seen := range want {
		if !seen {
			t.Errorf("DelegatedOperationSessionRecord 缺少 %q", f)
		}
	}

	// 憑證與重放鍵永遠不在裡面。前兩者單獨持有即可操作，第三者是可預測的重放鍵。
	for _, f := range got {
		switch strings.ToLower(f) {
		case "operationcredential", "handoffcode", "handoffcodehash", "credentialhash", "idempotencykey":
			t.Errorf("DelegatedOperationSessionRecord 含 %q —— 它是憑證或重放鍵，不是「這次代行做了什麼」的一部分", f)
		}
	}
}

// 撤銷四件套必須輸出 JSON null 而不是消失（#2483 A9）。
//
// 少了它們，撤銷理由就是一個「UI 強制填、系統永不回顯」的欄位——那會訓練操作者亂填，
// 而它正是爭議時唯一能舉證「為什麼中止」的字串。用 omitempty 讓欄位在未撤銷時整個消失
// 同樣不行：undefined 與 null 在前端是兩種判斷，而這裡要表達的是「確定沒有被撤銷」。
func TestDelegatedRevocationFields_SerialiseAsNullNotAbsent(t *testing.T) {
	for name, v := range map[string]any{
		"DelegatedSessionSummary":         DelegatedSessionSummary{},
		"DelegatedOperationSessionRecord": DelegatedOperationSessionRecord{},
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal %s: %v", name, err)
		}
		for _, key := range []string{"revokedAt", "revokedByAdminId", "revokedByAdminDisplayName", "revokeReason"} {
			got, present := m[key]
			if !present {
				t.Errorf("%s 未撤銷時 %q 整個消失 —— 應為 JSON null（多半是加了 omitempty）", name, key)
				continue
			}
			if got != nil {
				t.Errorf("%s 未撤銷時 %q = %v，應為 null", name, key, got)
			}
		}
	}
}

// 列表 DTO 不得宣告只有詳情拿得到的 maskedPhone。上面那支守的是「不准給」，這支守的是
// 「上游根本沒有」：GetSellerOrders 的 OrderSummaryView 沒有收件人電話欄位，宣告它等於
// 每一列都送出一個恆為空字串的 PII 欄位——讀契約的人會以為列表拿得到，前端會據此寫出
// 一條永遠不執行的分支。反向斷言詳情確實還帶著它，否則「型別分離」會退化成整個功能消失。
func TestDelegatedOrderList_DeclaresNoDetailOnlyMaskedPhone(t *testing.T) {
	const field = "maskedPhone"
	for _, f := range collectJSONFieldNames(reflect.TypeOf(DelegatedOrderListItem{}), map[reflect.Type]bool{}) {
		if strings.EqualFold(f, field) {
			t.Errorf("DelegatedOrderListItem 宣告了 %q —— OrderSummaryView 沒有這個來源欄位，"+
				"宣告它只會讓每一列回一個空字串，讀契約的人以為列表拿得到電話", field)
		}
	}
	found := false
	for _, f := range collectJSONFieldNames(reflect.TypeOf(DelegatedOrderDetail{}), map[reflect.Type]bool{}) {
		if strings.EqualFold(f, field) {
			found = true
		}
	}
	if !found {
		t.Errorf("DelegatedOrderDetail 缺少 %q —— 詳情是遮罩電話唯一的出口", field)
	}
}

// collectJSONFieldNames walks a struct and returns every json field name it can emit,
// including nested structs and slice/pointer element types.
func collectJSONFieldNames(t reflect.Type, seen map[reflect.Type]bool) []string {
	for t != nil && (t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct || seen[t] {
		return nil
	}
	seen[t] = true

	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // unexported
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = f.Name
		}
		out = append(out, name)
		out = append(out, collectJSONFieldNames(f.Type, seen)...)
	}
	return out
}

// TestDelegatedDTOCatalog_CoversEveryResponseOutlet 守住上面那份手寫清單本身。
//
// 原本的設計是「新增代行 DTO 卻忘了列進來，這支測試就守不到它——所以清單本身也是
// 契約的一部分」。但那句話當時沒有任何東西在執行：清單漏了三個實際出口
// （delegatedMutationData／DelegatedMediaCompleteData／DelegatedRecipientContactData），
// 其中第一個是每一支寫入端點的統一回應形狀，而所有測試照樣全綠。
//
// 這裡直接解析四支 admin_delegated_*.go 的 AST，把每一個 `response.OK(c, X{...})`
// 的 X 抓出來比對清單。手寫清單守 DTO，這支守手寫清單。
func TestDelegatedDTOCatalog_CoversEveryResponseOutlet(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := filepath.Glob("admin_delegated_*.go")
	if err != nil || len(entries) == 0 {
		t.Fatalf("找不到代行 handler 原始碼（glob err=%v, n=%d）", err, len(entries))
	}

	catalog := delegatedResponseDTOs()
	var missing []string
	scanned := 0

	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "OK" {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "response" {
				return true
			}
			// response.OK(c, X{...}) — 取第二個實參的複合字面型別名
			lit, ok := call.Args[1].(*ast.CompositeLit)
			if !ok {
				return true
			}
			id, ok := lit.Type.(*ast.Ident)
			if !ok {
				return true
			}
			scanned++
			if _, listed := catalog[id.Name]; !listed {
				missing = append(missing,
					fmt.Sprintf("%s (%s)", id.Name, fset.Position(call.Pos())))
			}
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("AST 掃描沒有找到任何 response.OK 出口 —— 這支測試失去鑑別力，" +
			"多半是回應寫法改了（例如改走 helper），掃描條件要跟著更新")
	}
	if len(missing) > 0 {
		t.Errorf("下列回應型別是代行端點的實際出口，但不在 delegatedResponseDTOs() 清單內，"+
			"因此禁用欄位契約守不到它們：\n  %s", strings.Join(missing, "\n  "))
	}
}

// 代行貼文列表必須帶得出「這則是不是草稿」。
//
// 少了它，草稿與已發布在列表上長得一模一樣（草稿沒有任何狀態標記），而代行工作區的
// 管理員正是靠列表判斷這家店的經營狀態 —— 他會把一則從未發布的草稿當成已上線的貼文。
// dev 驗收實際踩到：新建的草稿在列表裡沒有 pill，其他 10 則都有「已下架」。
//
// 這條在 Gateway 這一層守，因為它跨三層：上游 PostListItem 曾經根本沒有 publish_status
// （只有 GetPostDetail 的 PostDetailInfo 有），漏了任一層前端就永遠拿到空字串 —— 而空
// 字串在 UI 上與「沒有這個欄位」無法分辨，兩者都是不畫 pill。
// 未登錄的發布狀態碼必須原樣穿透，不得變成空字串。
//
// 兩支 DTO 的 publishStatusLabel 都是 codemeta.PostPublishStatusLabel 查表的結果，而碼
// 值是會長的：CodeMeta 新增 4404 而 Gateway 還沒重新生成 labels.go 的那個窗口一定存在
// （生成檔與後端服務是兩次部署）。此時若查不到就回空字串，前端收到的
// publishStatusLabel 與「這欄不存在」無法分辨 —— 而它已經有一個真的碼值可以顯示。
//
// 原樣回傳是刻意的：畫面上出現「4404」很醜，但看得出發生了什麼、追得到；空白格追不到。
// ⚠️ labels.go 是 generate.sh 的生成檔（紅線 6），這支測試釘的是它的行為契約，
// 不是要求誰去改它。
func TestDelegatedPostPublishStatusLabel_UnknownCodePassesThrough(t *testing.T) {
	const future = "4404" // 尚未登錄的發布狀態碼

	got := codemeta.PostPublishStatusLabel(future)
	if got == "" {
		t.Fatalf("未登錄碼的標籤是空字串 —— 前端分不出「沒這個欄位」與「不認識這個碼」")
	}
	if got != future {
		t.Fatalf("PostPublishStatusLabel(%q) = %q，want %q（原樣穿透）", future, got, future)
	}

	// 已登錄的碼仍必須翻成中文，否則上面那條會被一個「什麼都原樣回傳」的實作滿足。
	if l := codemeta.PostPublishStatusLabel("4401"); l != "未發布" {
		t.Fatalf("PostPublishStatusLabel(\"4401\") = %q, want \"未發布\"", l)
	}
}

func TestDelegatedPostListItem_CarriesPublishStatus(t *testing.T) {
	typ := reflect.TypeOf(DelegatedPostListItem{})
	want := map[string]bool{"publishStatus": false, "publishStatusLabel": false}
	for i := range typ.NumField() {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Errorf("DelegatedPostListItem 缺少 %q —— 前端分不出草稿與已發布", key)
		}
	}

	// 標籤與碼要成對：只給碼等於要前端自己維護第二份 CodeMeta 對照表（那份必然漂移），
	// 只給標籤則讓前端無法據以分支。詳情頁 DTO 就是兩者都給，列表沿用同一口徑。
	detail := reflect.TypeOf(DelegatedPostDetail{})
	for _, key := range []string{"publishStatus", "publishStatusLabel"} {
		var inDetail bool
		for i := range detail.NumField() {
			if strings.Split(detail.Field(i).Tag.Get("json"), ",")[0] == key {
				inDetail = true
			}
		}
		if !inDetail {
			t.Errorf("詳情 DTO 少了 %q，列表與詳情的口徑就對不齊了", key)
		}
	}
}
