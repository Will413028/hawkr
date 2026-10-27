package config

import (
	"testing"
)

// 守 #3333：PAYUNI_SANDBOX_MER_ID 的覆寫**只能在 sandbox 模式生效**。
//
// 原本的寫法在 usecase 裡直接 os.Getenv 且不看 IsSandbox ⇒ 無條件覆寫。而 prod 的
// hawkr-payuni secret 實際帶著 PAYUNI_SANDBOX_MER_ID=HAWK9（那是
// docker-compose-local.yml 的本地預設值），所以只要有人開始消費
// SellerShippingProfile.MerID，prod 就會拿到測試商店代號。
//
// 🔴 陽性（sandbox 會覆寫）人人記得驗；真正的修復本體是**陰性**——
// 非 sandbox 模式下即使 env 有值也不得帶進來。少了陰性那一半，
// 這支測試對本次修復零鑑別力。
func TestPayuniSandboxMerIDOverride_OnlyInSandbox(t *testing.T) {
	cases := []struct {
		name      string
		sandbox   string // PAYUNI_SANDBOX 的值；"__ABSENT__" 代表不設
		merID     string // PAYUNI_SANDBOX_MER_ID 的值；"__ABSENT__" 代表不設
		wantValue string // 期望的 SandboxMerIDOverride（硬寫字面值，不由被測物推導）
	}{
		{"sandbox=true 且有 override：帶進來", "true", "HAWK9", "HAWK9"},
		{"🔴 sandbox=false 但 env 有值：一律不帶（prod 的實際情境）", "false", "HAWK9", ""},
		{"sandbox 未設（預設 true）且有 override：帶進來", "__ABSENT__", "HAWK9", "HAWK9"},
		{"sandbox=true 但 env 沒設：空", "true", "__ABSENT__", ""},
		{"sandbox=false 且 env 沒設：空", "false", "__ABSENT__", ""},
		{"sandbox 是其他字串（非 \"true\"）⇒ 視為正式：不帶", "1", "HAWK9", ""},
		{"override 前後有空白：TrimSpace 後帶進來", "true", "  HAWK9  ", "HAWK9"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// GetEnvOrDefault("PAYUNI_SANDBOX", "true") 的語意：env 缺席或空 ⇒ "true"。
			// 測純函式時把那一步展開，避免測試依賴 Load()（它內含 Validate，
			// 必填清單會變）。
			sandboxFlag := c.sandbox
			if sandboxFlag == "__ABSENT__" {
				sandboxFlag = "true"
			}
			raw := c.merID
			if raw == "__ABSENT__" {
				raw = ""
			}

			if got := resolveSandboxMerIDOverride(sandboxFlag, raw); got != c.wantValue {
				t.Errorf("resolveSandboxMerIDOverride(%q, %q) = %q，預期 %q",
					sandboxFlag, raw, got, c.wantValue)
			}
		})
	}
}
