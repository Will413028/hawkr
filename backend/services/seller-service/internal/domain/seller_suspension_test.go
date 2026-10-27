package domain

import "testing"

// issue #1534：SellerSuspensionReason 全鏈路統一 CodeMeta Value 數字碼，Value→Label
// 對照表為手動維護（Go 服務尚未接線 CodeMeta codegen，見 seller_suspension.go 註解），
// 本檔表驅動測試釘住三值字面與 Label 對照，防未來 CodeMeta .md 改動後此表靜默
// drift，並釘住「#1514 Name 輸入相容已移除」不回歸。權威來源：
// docs/代碼管理（CodeMeta）/業務狀態碼.md SellerSuspensionReason（9601/9602/9603）。

// TestSellerSuspensionReasonConstants_PinCodeMetaValues 以字面值釘住常數 =
// CodeMeta Value（常數若被改回 Name 或改成其他碼，這裡直接紅）。
func TestSellerSuspensionReasonConstants_PinCodeMetaValues(t *testing.T) {
	pins := []struct {
		name string
		got  string
		want string
	}{
		{"POLICY_VIOLATION", SellerSuspensionReasonPolicyViolation, "9601"},
		{"REPEATED_WARNING_IGNORED", SellerSuspensionReasonRepeatedWarningIgnored, "9602"},
		{"OTHER", SellerSuspensionReasonOther, "9603"},
	}
	for _, p := range pins {
		if p.got != p.want {
			t.Errorf("常數 %s = %q, want CodeMeta Value %q", p.name, p.got, p.want)
		}
	}
}

func TestIsValidSellerSuspensionReason(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		// Value 數字碼（#1534 起唯一合法形態）
		{"Value 9601 合法", "9601", true},
		{"Value 9602 合法", "9602", true},
		{"Value 9603 合法", "9603", true},
		// #1514 的 Name 輸入相容已移除——Name 不再合法（釘住不回歸）
		{"Name POLICY_VIOLATION 已不合法", "POLICY_VIOLATION", false},
		{"Name REPEATED_WARNING_IGNORED 已不合法", "REPEATED_WARNING_IGNORED", false},
		{"Name OTHER 已不合法", "OTHER", false},
		// 其他非法輸入
		{"非法數字碼", "9999", false},
		{"label 文字非合法輸入", "違反平台規範", false},
		{"空字串", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidSellerSuspensionReason(tt.in); got != tt.want {
				t.Errorf("IsValidSellerSuspensionReason(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSellerSuspensionReasonLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"9601 → 違反平台規範", SellerSuspensionReasonPolicyViolation, "違反平台規範"},
		{"9602 → 屢勸不改", SellerSuspensionReasonRepeatedWarningIgnored, "屢勸不改"},
		{"9603 → 其他", SellerSuspensionReasonOther, "其他"},
		{"Name 輸入不查 label（僅 Value 鍵）", "POLICY_VIOLATION", ""},
		{"非法碼 → 空字串", "9999", ""},
		{"空字串 → 空字串", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SellerSuspensionReasonLabel(tt.in); got != tt.want {
				t.Errorf("SellerSuspensionReasonLabel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestSellerSuspensionReasonLabels_CoverAllValidReasons 釘住「合法 Value 全集 ⇔
// Label 對照表鍵全集」的一致性：未來在常數區新增第四值卻漏補對照表時，此測試
// 直接抓出（Label 對合法 Value 回空字串 = 表漏項）。
func TestSellerSuspensionReasonLabels_CoverAllValidReasons(t *testing.T) {
	validValues := []string{
		SellerSuspensionReasonPolicyViolation,
		SellerSuspensionReasonRepeatedWarningIgnored,
		SellerSuspensionReasonOther,
	}
	for _, v := range validValues {
		if !IsValidSellerSuspensionReason(v) {
			t.Errorf("IsValidSellerSuspensionReason(%q) = false, want true", v)
		}
		if SellerSuspensionReasonLabel(v) == "" {
			t.Errorf("SellerSuspensionReasonLabel(%q) 為空——labels 對照表漏項", v)
		}
	}
}
