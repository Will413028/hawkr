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
)

var delegatedForbiddenFields = []string{
	"shipmentno", "labelkey", "labelurl", "labelexpireat",
	"odno", "validationno", "partnerid", "pickupstorename",
	"shipmentphotokeys", "receiptphotokeys", "cvsstoreid",
	"transactionid", "providerstatus", "providermessage",
	"refundhistory",
	"review", "isreviewed", "canreview", "reviewdeadline",
	"likecount", "commentcount", "viewcount", "sharecount", "isliked",
	"seotitle", "seodescription", "birthday", "gender", "foodbusinessregno",
	"email", "avatarkey", "avatarurl", "lifecyclestatus", "suspensionreasoncode",
	"overallcertificationstatus", "currentcertificationid",
	"recipientphone", "shippingaddress",
	"imageurl", "imageurls", "logourl", "coverurl", "previewurl",
}

var delegatedForbiddenFieldExceptions = map[string]struct{}{}

func delegatedResponseDTOs() map[string]any {
	return map[string]any{
		"DelegatedProductListItem":        DelegatedProductListItem{},
		"DelegatedProductDetail":          DelegatedProductDetail{},
		"DelegatedOrderListItem":          DelegatedOrderListItem{},
		"DelegatedOrderDetail":            DelegatedOrderDetail{},
		"DelegatedAuditLogEntry":          DelegatedAuditLogEntry{},
		"DelegatedSessionSummary":         DelegatedSessionSummary{},
		"DelegatedOperationSessionRecord": DelegatedOperationSessionRecord{},
		"DelegatedBatchUnpublishData":     DelegatedBatchUnpublishData{},
		"DelegatedRecipientContactData":   DelegatedRecipientContactData{},
		"delegatedMutationData":           delegatedMutationData{},
		"delegatedProductMutationData":    delegatedProductMutationData{},
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
	for key := range delegatedForbiddenFieldExceptions {
		if !usedExceptions[key] {
			t.Errorf("例外 %q 已不對應任何 DTO 欄位，請一併移除", key)
		}
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
			continue
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
