import { forwardDelegated, numericSegment } from "../../_forward";

/**
 * `GET /adm/api/orders/{storeId}` → `GET /admin/sellers/{storeId}/orders`
 * （`orders.read`，§6.1.10）。
 *
 * ⚠️ 代行的訂單面**只有列表與明細兩支唯讀端點**：沒有彙總統計 RPC，KPI 一律取本端點
 * 的 `total` 或不渲染。收件資訊在這裡就是遮罩過的（遮罩發生在 order-service，不是
 * Gateway，也不是這裡）。
 *
 * ⚠️ 這一層**不得**出現任何履約動作。訂單頁的 row checkbox、批次列與出貨 CTA 一律
 * **不渲染**（不是 `disabled`）—— 沒有可打的端點，畫出來就只能是死按鈕。
 */

type RouteContext = { params: Promise<{ storeId: string }> };

/**
 * Gateway `ListDelegatedOrders` 實際讀取的六個 key（§6.1.10 的 Query 契約全集）。
 *
 * ⚠️ `date_from`／`date_to` 走 **ISO 8601 UTC**（`YYYY-MM-DDTHH:mm:ssZ`），不是使用者
 * 在日期框裡看到的台北曆日 —— 展開在 `features/delegated-operations/orders/repo.ts`，
 * 別在這一層再折一次。
 *
 * ⚠️ `keyword` 的比對範圍由後端界定，且**不含收件人電話與完整地址**（56 號 §8.1.5）。
 * 這一層只轉送，不得自己補上任何「順便也搜一下」的欄位。
 */
const LIST_QUERY = [
	"page",
	"page_size",
	"status",
	"keyword",
	"date_from",
	"date_to",
] as const;

export const dynamic = "force-dynamic";

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "GET",
		path: `/admin/sellers/${numericSegment(storeId)}/orders`,
		timeout: "mutation",
		query: LIST_QUERY,
	}));
}
