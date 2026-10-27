package delegation

// Fixed literals for delegated_operation_sessions.revoke_reason (56 號 §12.4 ①).
//
// The column is how an auditor tells the sources of one terminal state apart, so
// four of the five sources are closed literals rather than free text; only the
// remaining one — a human revocation carrying a ticket number or an operational
// judgement — is prose. A source that writes prose where a literal belongs makes
// every `WHERE revoke_reason = '...'` query silently miss it.
//
// ⚠️ authorization_revoked 與 store_not_operable 不可混用，即使兩者都是「系統自動
// 撤銷」：前者是「這家店不再被授權代行」（店家本身正常營業），後者是「這家店已停權／
// 關店」。兩者的後續處置相反——前者要問營運「授權為什麼被撤」，後者要問賣家治理
// 「店為什麼被停」。沿用另一個會讓稽核紀錄說一件沒發生的事。
//
// 新增第六種必須同步 §12.4 ① 的總表、§8.1.3 的同名清單與 §7.5.2 的觸發者表。
const (
	// RevokeReasonAdminLogout: ADM 登出連動（自動）。revoked_by_admin_id ＝ 他自己。
	RevokeReasonAdminLogout = "admin_logout"
	// RevokeReasonStoreNotOperable: 授權鏈第 8 項命中（自動，§8.1.3）。
	// revoked_by_admin_id 填當下觸發那次請求的 ADM 自己；排程路徑填該 session 的
	// actor_admin_id。
	RevokeReasonStoreNotOperable = "store_not_operable"
	// RevokeReasonDelegationDisabled: 取消該 admin 的代行資格（人工，§12.4 ①）。
	// revoked_by_admin_id 填執行取消的營運人員，不是被取消的那位。
	RevokeReasonDelegationDisabled = "delegation_disabled"
	// RevokeReasonAuthorizationRevoked: 治理端撤回該店的代行授權，同交易連帶撤銷
	// （自動，§6.1.15 端點 6）。revoked_by_admin_id ＝ 授權列的 revoked_by_admin_id，
	// 兩張表同一個人。
	RevokeReasonAuthorizationRevoked = "authorization_revoked"
)
