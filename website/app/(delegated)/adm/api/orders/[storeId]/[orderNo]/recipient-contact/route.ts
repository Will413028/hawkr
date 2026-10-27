import {
	forwardDelegated,
	numericSegment,
	orderNoSegment,
} from "../../../../_forward";

/**
 * `GET /adm/api/orders/{storeId}/{orderNo}/recipient-contact` →
 * `GET /admin/sellers/{storeId}/orders/{orderNo}/recipient-contact`
 * （`orders.read` ＋ **必落稽核**，§6.1.10）。
 *
 * **全平台唯一會回傳未遮罩收件資訊的端點。** 稽核 action ＝ `ORDER_PII_REVEAL`，
 * `risk_level = high`，計入結束摘要的「風險操作」計數；稽核 intent 必須先寫成功才回
 * 資料（寫不下去即 9279／409，不揭露）。
 *
 * ⚠️ 它是 GET，所以理由走 **query `operation_reason`**（GET 沒有 body），trim 後
 * 1–100 字必填。這是 allow-list 裡唯一一個非分頁的 query key。
 */

type RouteContext = { params: Promise<{ storeId: string; orderNo: string }> };

const REVEAL_QUERY = ["operation_reason"] as const;

export const dynamic = "force-dynamic";

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, orderNo } = await context.params;
	return forwardDelegated(request, () => ({
		method: "GET",
		path: `/admin/sellers/${numericSegment(storeId)}/orders/${orderNoSegment(orderNo)}/recipient-contact`,
		timeout: "mutation",
		query: REVEAL_QUERY,
	}));
}
