import AllAuditLogConsole from "@/features/delegated-operations/components/AllAuditLogConsole";
import "@/styles/pages/DelegatedOperations.css";

/**
 * 跨工作階段代行紀錄（56 §11.3 P1 的第二支端點；接線見 backend#2740）。
 *
 * 與同層的 `[sessionId]` 是**靜態段 vs 動態段**：Next.js 先比對靜態段，所以
 * `/admin/delegated-operations/audit-logs` 落在本頁而不是 `[sessionId]`。這不是巧合
 * 也不是脆弱的依賴 —— `isDelegatedSessionId` 只認純數字，`audit-logs` 本來就不可能
 * 被當成一個工作階段 ID。gateway 側的路由樹是同一個形狀（`routes_delegated.go:49`
 * 有一段註解專門講這件事）。
 */
export default function DelegatedAllAuditLogPage() {
	return <AllAuditLogConsole />;
}
