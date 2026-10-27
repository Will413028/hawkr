package delegation

import (
	"encoding/json"
	"strings"
	"testing"
)

// 規則表以外的欄位一律不進快照，這是 fail-safe 的核心：某天 seller DTO 長出
// bankAccount，它不會因為「沒人記得登記」而自動流進一張永久保存、全體管理員可查的
// 稽核表。反向漏記（規則表有、DTO 沒有）只會少一列 diff，代價遠小於多洩一個欄位。
func TestProject_DropsUnregisteredFields(t *testing.T) {
	got := Project("storefront", map[string]any{
		"name":        "梨山盛花園",
		"bankAccount": "0123456789",
		"birthday":    "1990-01-01",
	})
	if _, leaked := got["bankAccount"]; leaked {
		t.Error("未登記欄位 bankAccount 進了快照")
	}
	if _, leaked := got["birthday"]; leaked {
		t.Error("未登記欄位 birthday 進了快照")
	}
	if got["name"] != "梨山盛花園" {
		t.Errorf("已登記欄位遺失：%v", got)
	}
}

// 變更判定用原始值，顯示值才套 formatter。順序反過來會漏記：truncate(60) 讓兩段只有
// 第 200 字不同的長文字看起來相同，先比較就直接被排除掉了。
func TestDiff_ComparesRawValuesNotFormattedOnes(t *testing.T) {
	long := strings.Repeat("甲", 199)
	before := map[string]any{"description": long + "乙"}
	after := map[string]any{"description": long + "丙"}

	fields := Diff("product", Project("product", before), Project("product", after), nil)
	if len(fields) != 1 {
		t.Fatalf("changed_fields 長度 = %d, want 1 —— 只改第 200 字仍是一次真實的內容修改", len(fields))
	}
	if fields[0].GetBefore() == "" || fields[0].GetAfter() == "" {
		t.Error("before/after 不得為空字串")
	}
	// 顯示值在此刻意相同（都被截到 60 字），這是接受的取捨：寧可顯示一列看不出差別的
	// 變更，也不要讓一次真實的內容修改在稽核上消失。
	if fields[0].GetBefore() != fields[0].GetAfter() {
		t.Logf("顯示值不同（截斷邊界之內），仍屬正確")
	}
}

func TestDiff_UnchangedFieldsAreOmitted(t *testing.T) {
	same := map[string]any{"name": "梨山盛花園", "prize": "2025 十大精品水果"}
	if fields := Diff("storefront", same, same, nil); len(fields) != 0 {
		t.Errorf("未變更的欄位不得進 changed_fields，實得 %d 列", len(fields))
	}
}

// before 缺鍵而 after 有值時，before 必須輸出該 formatter 的空值形態，不得是 null 或
// undefined：ChangedField.before/.after 是 proto string，UI 三欄表也不處理缺值。
func TestDiff_MissingSideRendersEmptyForm(t *testing.T) {
	fields := Diff("product",
		map[string]any{},
		map[string]any{"stockQty": "36", "priceMin": "1280"}, nil)

	byField := map[string]string{}
	for _, f := range fields {
		byField[f.GetField()] = f.GetBefore()
	}
	if byField["stockQty"] != "0 件" {
		t.Errorf("qty 空值形態 = %q, want \"0 件\"", byField["stockQty"])
	}
	if byField["priceMin"] != "NT$ 0" {
		t.Errorf("money 空值形態 = %q, want \"NT$ 0\"", byField["priceMin"])
	}
}

// 建立類沒有 before，所以每一個欄位都走「單側缺鍵」那條路。缺席側渲染成 formatter 的
// 空形之後，若在場側渲染成同一個字串，這一列講的是「從沒有變成沒有」——它不是變更。
//
// dev 三案獨立重現：代行建立一則沒有配圖、沒有關聯商品的貼文，稽核列固定多出
// imageKeys「0 張 → 0 張」與 productIds「0 項 → 0 項」兩列從未發生的變更。稽核列
// append-only、寫進去就永遠在那裡，而「沒有假變更列」正是 changed_fields 拿得出來當
// 證據的前提。
func TestDiff_CreateDoesNotInventZeroCountChanges(t *testing.T) {
	// POST_CREATE 的真實形狀：before 是空投影（沒有前身），after 由 handler 把
	// 集合欄位投影成計數字串。
	after := Project("post", map[string]any{
		"content":    "今日現採",
		"productIds": "0",
		"imageKeys":  "0",
	})

	got := map[string]string{}
	for _, f := range Diff("post", map[string]any{}, after, nil) {
		got[f.GetField()] = f.GetBefore() + " → " + f.GetAfter()
	}
	if len(got) != 1 || got["content"] == "" {
		t.Fatalf("changed_fields = %v, want 只有 content 一列", got)
	}

	// 鑑別力：把假變更擋掉不得順手把真變更也擋掉。「從無到有」兩側渲染必然不同。
	withImages := Project("post", map[string]any{
		"content":    "今日現採",
		"productIds": "0",
		"imageKeys":  "2",
	})
	real := map[string]string{}
	for _, f := range Diff("post", map[string]any{}, withImages, nil) {
		real[f.GetField()] = f.GetBefore() + " → " + f.GetAfter()
	}
	if real["imageKeys"] != "0 張 → 2 張" {
		t.Errorf("imageKeys = %q, want \"0 張 → 2 張\" —— 真的掛了兩張圖必須記", real["imageKeys"])
	}
	if _, invented := real["productIds"]; invented {
		t.Errorf("productIds 仍被記成變更：%v", real)
	}
}

// 同一個假變更的第二種外殼：Plain／Truncate 的空形是「—」，於是缺席側與空字串側渲染
// 成同一個「— → —」。貼文的 scheduledPublishAt／publishedAt 在未排程時就是這個形狀。
func TestDiff_MissingSideMatchingEmptyStringIsNotAChange(t *testing.T) {
	after := map[string]any{"scheduledPublishAt": "", "publishedAt": "", "content": "今日現採"}
	for _, f := range Diff("post", map[string]any{}, after, nil) {
		if f.GetField() != "content" {
			t.Errorf("%s 被記成 %q → %q，但兩側都是「沒有值」",
				f.GetField(), f.GetBefore(), f.GetAfter())
		}
	}
}

// 反方向同樣要擋：after 缺鍵而 before 是零值時，也是「從沒有變成沒有」。
func TestDiff_AbsentAfterMatchingZeroBeforeIsNotAChange(t *testing.T) {
	before := Project("product", map[string]any{"images": "0", "name": "檸檬"})
	got := map[string]bool{}
	for _, f := range Diff("product", before, map[string]any{"name": "有機檸檬"}, nil) {
		got[f.GetField()] = true
	}
	if got["images"] {
		t.Error("images 由「0 張」變成缺席被記成變更，但兩側都是「沒有圖」")
	}
	if !got["name"] {
		t.Error("name 的真實變更遺失")
	}
}

// enum 的 Label 一律查生成的 CodeMeta 表。自行意譯的失敗形狀是 drift 而非錯誤：
// 沒有任何測試會告訴你哪一邊才對。4004 是「已上架」不是「上架中」。
func TestDiff_EnumLabelsComeFromCodeMeta(t *testing.T) {
	fields := Diff("product",
		map[string]any{"status": "4003"},
		map[string]any{"status": "4004"}, nil)
	if len(fields) != 1 {
		t.Fatalf("changed_fields 長度 = %d, want 1", len(fields))
	}
	if fields[0].GetBefore() != "未上架" || fields[0].GetAfter() != "已上架" {
		t.Errorf("enum label = %q → %q, want 未上架 → 已上架", fields[0].GetBefore(), fields[0].GetAfter())
	}
	if fields[0].GetLabel() != "發布狀態" {
		t.Errorf("label = %q, want 發布狀態", fields[0].GetLabel())
	}
}

// 超限時整欄改存佔位物件，不得截斷 JSON 字串：截斷後不是合法 JSON，寫進 jsonb 會得到
// 22P02。也不得讓 intent 失敗——那是 fail-closed，會為了「快照太大」拒絕一次合法寫入。
func TestMarshalSnapshot_OversizeBecomesValidPlaceholder(t *testing.T) {
	huge := map[string]any{"description": strings.Repeat("字", 40000)}
	payload, oversize := MarshalSnapshot(huge)
	if !oversize {
		t.Fatal("40000 個中文字（>64 KB）應判為超限")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("佔位物件不是合法 JSON：%v", err)
	}
	if decoded["_truncated"] != true || decoded["_reason"] != "snapshot_too_large" {
		t.Errorf("佔位物件形狀不正確：%v", decoded)
	}
}

func TestMarshalSnapshot_NormalSizePassesThrough(t *testing.T) {
	payload, oversize := MarshalSnapshot(map[string]any{"name": "梨山盛花園"})
	if oversize {
		t.Fatal("小快照不應判為超限")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("快照不是合法 JSON：%v", err)
	}
	if decoded["name"] != "梨山盛花園" {
		t.Errorf("快照內容不正確：%v", decoded)
	}
}

// order 的規則表是空的：ORDER_PII_REVEAL 沒有任何欄位被改，兩個快照都只存
// {"revealedFields":[…]}，一律不存 PII 值本身。
func TestDiff_OrderRuleTableIsEmpty(t *testing.T) {
	if len(DelegationDiffRules["order"]) != 0 {
		t.Error("order 的規則表必須是空的——訂單面唯讀")
	}
	if fields := Diff("order", map[string]any{"recipientPhone": "0912341234"}, map[string]any{}, nil); len(fields) != 0 {
		t.Error("order 不得產生任何 changed_fields")
	}
}

// ==================== 比較範圍（SnapshotFields / Scope*） ====================

// 這是本 bug 的最小重現：before 是完整投影、after 只有一個鍵（媒體端點的寫入回應就
// 是這個形狀），於是缺席的每一欄都被記成「被清空」。dev 實測換一次 logo 得到 10 列，
// 其中 9 列是從未發生的變更。
func TestDiff_UnscopedComparisonInventsChangesForAbsentKeys(t *testing.T) {
	before := Project("storefront", map[string]any{
		"name": "梨山盛花園", "city": "台中市", "district": "和平區",
		"contactPhone": "0911111111", "logoKey": "old.png",
	})
	after := Project("storefront", map[string]any{"logoKey": "new.jpg"})

	// 反面事實先坐實：不收窄比較範圍時，這個形狀必然虛報。若這一段有一天不再成立，
	// 代表 Diff 的缺鍵語意被改過，下面那條「收窄才對」的斷言也要跟著重新評估。
	if got := len(Diff("storefront", before, after, nil)); got != 5 {
		t.Fatalf("未收窄時 changed_fields 長度 = %d, want 5（1 真 + 4 假）", got)
	}

	scoped := Diff("storefront", before, after, []string{"logoKey"})
	if len(scoped) != 1 || scoped[0].GetField() != "logoKey" {
		t.Fatalf("收窄後 changed_fields = %v, want 只有 logoKey 一列", scoped)
	}
	if scoped[0].GetBefore() != "old.png" || scoped[0].GetAfter() != "new.jpg" {
		t.Errorf("logoKey %q → %q, want old.png → new.jpg", scoped[0].GetBefore(), scoped[0].GetAfter())
	}
}

// 第二種形狀：鍵**在**、值是零值。上下架端點的寫入回應曾經帶著空的 hashtags，於是
// 每次上架都虛報一列「主題標籤被清空」，而 DB 裡一列未動。
//
// ⚠️ 這一種「兩側鍵集合相同」，所以任何只比較鍵集合的不變式都抓不到它——真正的防線
// 是把比較範圍收窄到這次 mutation 動得到的欄位。
func TestDiff_ScopeAlsoContainsZeroValuedPresentKeys(t *testing.T) {
	before := Project("product", map[string]any{"status": "4003", "hashtags": "無籽檸檬", "name": "檸檬"})
	after := Project("product", map[string]any{"status": "4004", "hashtags": "", "name": "檸檬"})

	if got := len(Diff("product", before, after, nil)); got != 2 {
		t.Fatalf("未收窄時 changed_fields 長度 = %d, want 2（status 真 + hashtags 假）", got)
	}

	scoped := Diff("product", before, after, ScopeProductStatus())
	if len(scoped) != 1 || scoped[0].GetField() != "status" {
		t.Fatalf("收窄後 changed_fields = %v, want 只有 status 一列（56 號 §6.1.11）", scoped)
	}
}

// 收窄不得吃掉正常路徑：PATCH 走全表比較，改幾欄就記幾欄。
func TestDiff_NilScopeStillComparesTheWholeRuleTable(t *testing.T) {
	before := Project("product", map[string]any{"name": "檸檬", "images": "3", "origin": "屏東"})
	after := Project("product", map[string]any{"name": "有機檸檬", "images": "5", "origin": "屏東"})

	got := map[string]bool{}
	for _, f := range Diff("product", before, after, nil) {
		got[f.GetField()] = true
	}
	if len(got) != 2 || !got["name"] || !got["images"] {
		t.Errorf("changed_fields = %v, want {name, images}", got)
	}
}

// scope 內不在規則表的欄名會被靜默略過，所以**打錯一個字就等於什麼都不比較**：
// 端點照樣回 200、稽核列照樣寫入、changed_fields 恆為空，沒有任何訊號。
// 這支測試就是那個 typo 的偵測器。
func TestDiffScopes_OnlyReferenceRegisteredFields(t *testing.T) {
	scopes := map[string]struct {
		resourceType string
		fields       []string
	}{
		"ScopeProductStatus":    {"product", ScopeProductStatus()},
		"ScopePostPublishState": {"post", ScopePostPublishState()},
	}
	for name, sc := range scopes {
		if len(sc.fields) == 0 {
			t.Errorf("%s 回了空集合 —— 空 scope 等於「什麼都不比較」，要全表請用 nil", name)
		}
		seen := map[string]bool{}
		for _, f := range sc.fields {
			if seen[f] {
				t.Errorf("%s 有重複欄位 %q —— 會產出兩列相同的 changed_field", name, f)
			}
			seen[f] = true
			if _, registered := DelegationDiffRules[sc.resourceType][f]; !registered {
				t.Errorf("%s 含未登記欄位 %q（resourceType=%s）—— 它會被 Diff 靜默略過",
					name, f, sc.resourceType)
			}
		}
	}
}

// Scope* 必須每次回新的 slice。回同一個套件層變數的話，任一呼叫端 append 一次就改掉
// 其他所有呼叫端的比較範圍，而那個錯誤不編譯失敗、不轉紅。
func TestDiffScopes_ReturnFreshSlices(t *testing.T) {
	a := ScopeProductStatus()
	a = append(a, "name")
	_ = a
	if b := ScopeProductStatus(); len(b) != 1 || b[0] != "status" {
		t.Errorf("ScopeProductStatus() 被呼叫端污染：%v", b)
	}
}

// certifications 投影的是 CertificationType **碼值**（`3701、3702`），不是標籤名。
// 這一列曾經登記成 truncate/60，於是稽核三欄表的「產品驗證」永久存成一串數字 ——
// 而稽核快照是永久紀錄，diff.go 的檔頭又明令 admin repo 不得自建第二份對照表，
// 那個顯示是前端補不了的。
func TestDiff_CertificationCodesResolveToLabels(t *testing.T) {
	fields := Diff("product",
		map[string]any{"certifications": "3701"},
		map[string]any{"certifications": "3701、3702"}, nil)
	if len(fields) != 1 {
		t.Fatalf("changed_fields 長度 = %d, want 1", len(fields))
	}
	if fields[0].GetBefore() != "有機農產品" {
		t.Errorf("before = %q, want 有機農產品（不是碼值）", fields[0].GetBefore())
	}
	if fields[0].GetAfter() != "有機農產品、產銷履歷" {
		t.Errorf("after = %q, want 有機農產品、產銷履歷 —— 多值要逐段解析", fields[0].GetAfter())
	}
	if fields[0].GetLabel() != "產品驗證" {
		t.Errorf("label = %q, want 產品驗證", fields[0].GetLabel())
	}
}

// 認不得的碼值原樣輸出，且**不得**因為其中一段查不到就讓整格退成 —— 或空字串：
// 認證是法規宣稱，因為隔壁一段不認得就把它從稽核上抹掉是最糟的失敗形態。
func TestDiff_UnknownCertificationSegmentSurvivesAlongsideKnownOnes(t *testing.T) {
	fields := Diff("product",
		map[string]any{"certifications": "3701"},
		map[string]any{"certifications": "3701、9999"}, nil)
	if len(fields) != 1 {
		t.Fatalf("changed_fields 長度 = %d, want 1", len(fields))
	}
	if fields[0].GetAfter() != "有機農產品、9999" {
		t.Errorf("after = %q, want 有機農產品、9999", fields[0].GetAfter())
	}
}

// 規則表裡每一個 enum 名都必須在 enumLabel 的 switch 裡有對應的 case。
//
// 少一個 case 不會編譯失敗、不會有既有測試轉紅 —— default 分支原樣回傳碼值，於是那個
// 欄位從登記那天起就在稽核上顯示數字。這正是 certifications 當初的失效形狀，只是它
// 當時連 enum 都沒登記。⚠️ 判準用「解析後 != 原碼值」而不是「switch 有沒有這個字串」：
// 後者可以靠寫一個回傳 value 的 case 騙過去。
func TestDiff_EveryRegisteredEnumNameResolves(t *testing.T) {
	// 每個 enum 名給一個該 enum 確實存在的碼值（值取自 pkg/codemeta/labels.go 的生成表）。
	// 新增 enum 到規則表時這裡要一起加，漏加會在下面轉紅並指名要補什麼。
	// ⚠️ 探針值寫錯會讓斷言誤報成「switch 缺 case」——本測試第一版就把 PostPublishStatus
	// 寫成 5002（實際是 4401–4403），於是它報了一個不存在的缺口。探針要照生成表抄。
	probes := map[string]string{
		"ProductStatus":         "4004",
		"PostPublishStatus":     "4402",
		"OrderStatus":           "10",
		"DelegationAuditAction": "PRODUCT_UPDATE",
		"DelegationReasonCode":  "seller_request",
		"CertificationType":     "3701",
	}

	seen := map[string]bool{}
	for resourceType, rules := range DelegationDiffRules {
		for field, rule := range rules {
			if rule.Formatter != FormatterEnum {
				continue
			}
			seen[rule.Arg] = true
			probe, ok := probes[rule.Arg]
			if !ok {
				t.Errorf("%s.%s 用了 enum %q，但本測試沒有對應探針 —— 補一個已知碼值進 probes",
					resourceType, field, rule.Arg)
				continue
			}
			if got := enumLabel(rule.Arg, probe); got == probe {
				t.Errorf("%s.%s：enumLabel(%q, %q) 原樣回傳碼值 —— switch 缺 case，稽核會顯示數字",
					resourceType, field, rule.Arg, probe)
			}
		}
	}

	// 鑑別力：規則表真的掃到了 enum 欄位（掃不到的話上面整段是空跑）。
	if len(seen) == 0 {
		t.Fatal("規則表裡一個 FormatterEnum 都沒掃到 —— 斷言空過")
	}
	if !seen["CertificationType"] {
		t.Error("CertificationType 未出現在規則表的任何 enum 欄位")
	}
}
