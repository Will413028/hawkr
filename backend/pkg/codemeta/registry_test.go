package codemeta

import (
	"regexp"
	"testing"
)

// registry.go 由 generate.sh --go-registry 從 CodeMeta Markdown 生成，而 Markdown
// 是人手維護的表格。這幾條不變式全部被 procurement 契約文件明文倚賴，卻沒有任何
// 機制擋住打錯：
//
//   - Value == Code：這 7 組是字串碼（同 ClientType／PricebookQualityReasonCode 慣例），
//     不是數字碼。但同一份 Markdown 的多數 enum 用數字 Value（OrderStatus 是 0/10/50），
//     照那個慣例編輯就會產出 Code="SUCCEEDED"、Value="50" 而全綠通過——DB 存的是 Value、
//     契約比對的是 Code，兩者一分家就是靜默的資料寫錯。
//   - Code 形狀符合 AIP-193 的 [A-Z][A-Z0-9_]*：契約文件把這條寫成硬規定。
//   - 組內唯一：表格複製貼上漏改名字時，後者會覆蓋前者的查表結果。
//   - FromCode 與 FromValue 命中同一筆：兩支查詢函式都是生成的，但只有在 Value==Code
//     成立時它們才等價；這條測試順便鎖住「用哪一支查都一樣」這個呼叫端假設。
//
// 若未來有 procurement 以外的 enum 加進 GO_REGISTRY_ENUMS，把它補進下面的表格。

var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type registryEntry struct{ code, value, label string }

func TestProcurementRegistryInvariants(t *testing.T) {
	groups := map[string][]registryEntry{}

	for _, v := range AllProcurementRunStatuss {
		groups["ProcurementRunStatus"] = append(groups["ProcurementRunStatus"], registryEntry{v.Code, v.Value, v.Label})
	}
	for _, v := range AllProcurementStepStatuss {
		groups["ProcurementStepStatus"] = append(groups["ProcurementStepStatus"], registryEntry{v.Code, v.Value, v.Label})
	}
	for _, v := range AllProcurementLaneStatuss {
		groups["ProcurementLaneStatus"] = append(groups["ProcurementLaneStatus"], registryEntry{v.Code, v.Value, v.Label})
	}
	for _, v := range AllProcurementArtifactStatuss {
		groups["ProcurementArtifactStatus"] = append(groups["ProcurementArtifactStatus"], registryEntry{v.Code, v.Value, v.Label})
	}
	for _, v := range AllProcurementConfirmationStatuss {
		groups["ProcurementConfirmationStatus"] = append(groups["ProcurementConfirmationStatus"], registryEntry{v.Code, v.Value, v.Label})
	}
	for _, v := range AllProcurementRunFailureReasons {
		groups["ProcurementRunFailureReason"] = append(groups["ProcurementRunFailureReason"], registryEntry{v.Code, v.Value, v.Label})
	}
	for _, v := range AllProcurementErrorReasons {
		groups["ProcurementErrorReason"] = append(groups["ProcurementErrorReason"], registryEntry{v.Code, v.Value, v.Label})
	}

	if len(groups) != 7 {
		t.Fatalf("預期 7 組 procurement registry，實際 %d 組——新增組別請一併補進本測試", len(groups))
	}

	for name, entries := range groups {
		t.Run(name, func(t *testing.T) {
			if len(entries) == 0 {
				t.Fatalf("%s 是空的：generate.sh 可能沒把它產出來（GO_REGISTRY_ENUMS 打錯字不會報錯，只會靜默少生成）", name)
			}
			seen := map[string]bool{}
			for _, e := range entries {
				if e.value != e.code {
					t.Errorf("%s.%s: Value(%q) != Code(%q)。這 7 組是字串碼，Value 必須等於 Code；"+
						"若照數字碼 enum 的慣例填了數字，DB 存的值就會與契約比對的值分家",
						name, e.code, e.value, e.code)
				}
				if !codePattern.MatchString(e.code) {
					t.Errorf("%s: Code %q 不符 AIP-193 形狀 ^[A-Z][A-Z0-9_]*$", name, e.code)
				}
				if e.label == "" {
					t.Errorf("%s.%s: Label 為空——前端 PROCUREMENT_*_LABEL 會顯示空白", name, e.code)
				}
				if seen[e.code] {
					t.Errorf("%s: Code %q 重複，後者會覆蓋前者的查表結果", name, e.code)
				}
				seen[e.code] = true
			}
		})
	}
}

// FromCode 與 FromValue 是兩支獨立生成的查表函式。呼叫端普遍假設「用哪一支都一樣」，
// 但那個假設只有在 Value == Code 成立時才對。這裡對每一組各抽一個代表值鎖住。
func TestProcurementRegistryLookupsAgree(t *testing.T) {
	t.Run("RunStatus", func(t *testing.T) {
		byCode, okC := ProcurementRunStatusFromCode("SUCCEEDED")
		byValue, okV := ProcurementRunStatusFromValue("SUCCEEDED")
		if !okC || !okV || byCode != byValue {
			t.Fatalf("FromCode/FromValue 不一致: %+v(%v) vs %+v(%v)", byCode, okC, byValue, okV)
		}
	})
	t.Run("ErrorReason", func(t *testing.T) {
		byCode, okC := ProcurementErrorReasonFromCode("IDEMPOTENCY_KEY_CONFLICT")
		byValue, okV := ProcurementErrorReasonFromValue("IDEMPOTENCY_KEY_CONFLICT")
		if !okC || !okV || byCode != byValue {
			t.Fatalf("FromCode/FromValue 不一致: %+v(%v) vs %+v(%v)", byCode, okC, byValue, okV)
		}
	})
	t.Run("unknown code returns false", func(t *testing.T) {
		if _, ok := ProcurementRunStatusFromCode("NOT_A_REAL_STATUS"); ok {
			t.Fatal("未登記的碼必須回 false，否則呼叫端無法區分「查到」與「零值」")
		}
	})
}

// ===== #2450 publication approval 的三組值域 =====
//
// 這三組與上面 7 組一樣是字串碼，所以 TestProcurementRegistryInvariants 的
// Value==Code／AIP-193 形狀等不變式同樣適用；下面三支另外釘住的是**內容**：
// 值本身與順序被 DB CHECK、狀態機文件與 audienceHash preimage 逐字倚賴。

// approval 的狀態機值域。順序照 doc 64 §4.1 的生命週期敘述，不是字母序——
// 這份切片會被拿去產 DB CHECK 與前端顯示順序。
func TestProcurementRfqPublicationApprovalStates(t *testing.T) {
	got := make([]string, 0, len(AllProcurementRfqPublicationApprovalStates))
	for _, s := range AllProcurementRfqPublicationApprovalStates {
		got = append(got, s.Value)
	}
	want := []string{"PENDING", "EXECUTING", "SUCCEEDED", "REJECTED", "EXPIRED", "INVALIDATED", "FAILED"}
	if len(got) != len(want) {
		t.Fatalf("值域大小不符：want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 個值不符：want %q, got %q", i, want[i], got[i])
		}
	}
}

// 兩支 pipeline 的受眾種類。⛔ 不得新增第三種而不改 doc 64 §4.2——
// 兩者不共用完成定義，加值等於加一條沒有完成定義的路徑。
func TestProcurementPublicationAudienceModes(t *testing.T) {
	if got := ProcurementPublicationAudienceModeWishPool.Value; got != "WISH_POOL" {
		t.Errorf("WishPool.Value = %q, want %q", got, "WISH_POOL")
	}
	if got := ProcurementPublicationAudienceModeDirectRecipients.Value; got != "DIRECT_RECIPIENTS" {
		t.Errorf("DirectRecipients.Value = %q, want %q", got, "DIRECT_RECIPIENTS")
	}
	if n := len(AllProcurementPublicationAudienceModes); n != 2 {
		t.Errorf("受眾種類共 %d 個，want 2——新增第三種必須先改 doc 64 §4.2 的完成定義", n)
	}
}

// v1 的 policy 是常數不是資料表：值釘死，改動要走 v2 而不是改這裡。
// PolicyVersion 不在 CodeMeta（生成器的 struct 只有 Code/Value/Label/Description），
// 它釘在 ai-service 的 domain 層，見 domain.WishPoolPolicyVersionV1。
func TestProcurementWishPoolPolicyV1IsFrozen(t *testing.T) {
	if got := ProcurementWishPoolPolicyAllActiveSellersV1.Value; got != "WISH_POOL_ALL_ACTIVE_SELLERS_V1" {
		t.Errorf("policy Value = %q, want %q", got, "WISH_POOL_ALL_ACTIVE_SELLERS_V1")
	}
	if n := len(AllProcurementWishPoolPolicys); n != 1 {
		t.Errorf("wish pool policy 共 %d 個，want 1——v2 是新增值不是改 v1", n)
	}
}

// publication 的狀態值域。PUBLISHED 只代表 server transaction 完成，
// 不代表 Wish Pool 已 indexed 或賣家已收到（issue #2450 Scope 9）。
func TestProcurementPublicationStates(t *testing.T) {
	got := make([]string, 0, len(AllProcurementPublicationStates))
	for _, s := range AllProcurementPublicationStates {
		got = append(got, s.Value)
	}
	want := []string{"PUBLISHED", "SUPERSEDED", "CANCELLED"}
	if len(got) != len(want) {
		t.Fatalf("值域大小不符：want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 個值不符：want %q, got %q", i, want[i], got[i])
		}
	}
}
