import { forwardDelegated } from "../../_forward";

/**
 * `GET /adm/api/session/summary` → Gateway
 * `GET /admin/delegated-operations/current/summary`（56 號 §4.5 L1042、§6.1.11 C）。
 *
 * **`OperationEndDialog` 結束摘要綠卡的唯一資料來源。**
 *
 * ⚠️ **不能靠 `session/current` 兜住，也不能改打 `{sessionId}/summary`** —— 後者掛
 * `requireAdmin()`，www host 上既沒有 ADM session、代行 BFF 也沒有 ⇒ 一律 9261／401。
 *
 * ⚠️ **這一支漏掉的失敗形狀是靜默的**：綠卡拿不到數字時，前端唯一還做得出來的事就是
 * 把「未執行任何出貨或財務操作」寫死，而那句話會從 **P0 事故訊號**
 * （`fulfillmentActionCount`／`financeActionCount` 恆為 `"0"`）退化成永遠顯示綠色的
 * 裝飾文案。
 *
 * ⚠️ **終態仍須可讀**：這支端點在 `POST session/end` 之後還要回得出完整六欄 —— 綠卡
 * 正是在結束之後才顯示的。
 */

export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
	return forwardDelegated(request, {
		method: "GET",
		path: "/admin/delegated-operations/current/summary",
		timeout: "session",
	});
}
