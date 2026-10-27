import SessionListConsole from "@/features/delegated-operations/components/SessionListConsole";
import "@/styles/pages/DelegatedOperations.css";

/**
 * 代行紀錄的入口：工作階段列表 ＋ 直查。
 *
 * ⚠️ 這裡曾經只有一個查詢框，檔頭寫著「因為本期沒有『工作階段列表』端點」。那句話
 * 在 gateway 交付 `GET /api/v1/admin/delegated-operations`（56 §11.3 P1）之後就不再
 * 成立，而它留下的實際後果是：營運要回看代行紀錄，得先從別處拿到工作階段 ID——
 * §11.3 P1 的觸發條件之一正是「營運開始自己手寫 SQL 查 session_id 並且填錯」。
 *
 * ⚠️ 那段舊註解還提過「要做得先有分級」。列表確實列得出全平台的代行活動，但那與既有的
 * `/{sessionId}/summary`、`/{sessionId}/audit-logs` 是**同一個暴露面**：兩支從來就沒有
 * 「只能看自己開的」這道限制，`requireAdmin()` 之下猜得到 ID 就讀得到。列表把「猜得到」
 * 變成「列得出來」，不是新開一個權限缺口。真要分級是後端 capability 層的事（56 §7），
 * 前端不隱藏入口來假裝它已經有了。
 */
export default function DelegatedOperationListPage() {
	return <SessionListConsole />;
}
