import { forwardDelegated } from "../../_forward";

/**
 * `GET /adm/api/session/current` → Gateway `GET /admin/delegated-operations/current`
 * （56 號 §4.5 L1041、§6.1.4）。
 *
 * **工作區倒數與終態偵測的唯一來源**，開著時每 30 秒一次 ＋ focus ／ reconnect 各
 * 一次。
 *
 * ⚠️ 它打的是 `counts_as_activity = false` 的那一支 —— **輪詢不算活動**。這不是潔癖：
 * 把輪詢算成活動，`last_active_at` 就永遠不會老，閒置逾時在工作區開著時**永遠不會
 * 觸發**。前端也不得為了「保持連線」另打任何端點。
 *
 * ⚠️ **無 query、無 body**：目標店家只能由 credential 反查工作階段快照取得。加一個
 * `storeId` query 會讓這支端點直接死（Gateway 不收，F17）。
 */

export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
	return forwardDelegated(request, {
		method: "GET",
		path: "/admin/delegated-operations/current",
		timeout: "session",
	});
}
