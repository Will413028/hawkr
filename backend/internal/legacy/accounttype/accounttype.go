// Package accounttype 是舊契約的帳號類型（CodeMeta 的 AccountType）：帳號是角色的 bitmask（買家 1、賣家 2、兩者 3），
// 管理員是 4。單體不把買家存成角色（帳號活著就是買家，Will 2026-10-01 W5），identity 只有另外申請的 SELLER；舊契約
// 需要的買家位元在這裡、也只在這裡合成（anti-corruption layer）：gateway 的 X-Account-Type 與 X-Login-Role、
// AccountService 回傳的 account_type、media-service 的 x-account-type。擁有者是 identity，CodeMeta 那一段是投影
// （TestLegacyAccountTypeMatchesCodeMeta）。隨最後一個讀舊契約的服務搬進單體刪除。
package accounttype

import "github.com/Will413028/hawkr/backend/internal/modules/identity"

// 舊契約的值。
const (
	Buyer  = 1
	Seller = 2
	Admin  = 4
)

// OfLiveAccount 是存活帳號的 bitmask：一定有買家位元（帳號活著就是買家），有 SELLER 時加上賣家位元。呼叫方只對
// 存活的帳號呼叫它（刪除的帳號在舊契約查不到）。
func OfLiveAccount(roles []identity.Role) int {
	bits := Buyer
	for _, r := range roles {
		if r == identity.RoleSeller {
			bits |= Seller
		}
	}
	return bits
}

// OfLoginRole 是帳號這個請求在舊契約的登入身分（X-Login-Role）：gateway 的 handler 與 IM 在同一條路由內以它選擇以買家
// 或賣家的身分動作。hint 是舊 REST 的 client 送來的 X-Active-Role（工作區是 client 的 UI 狀態，Connect 不讀它）：
// 提示是 "SELLER" 而且帳號有賣家角色時是賣家，其他情況（沒有提示、看不懂、帳號不是賣家）是買家，因為每個帳號都是買家；
// 管理員沒有登入身分的提示，回 0（呼叫方不送）。退場條件是最後一個讀 loginRole 的 gateway handler 或 IM 消費端退場，
// 屆時 X-Active-Role 與這個推導一起刪除（ADR 2026-10-07-connect-without-perspective）。
func OfLoginRole(p identity.Principal, hint string) int {
	if p.Kind != identity.PrincipalAccount {
		return 0
	}
	if hint == "SELLER" && p.HasRole(identity.RoleSeller) {
		return Seller
	}
	return Buyer
}
