import {
	forwardDelegated,
	numericSegment,
	orderNoSegment,
} from "../../../_forward";

/**
 * `GET /adm/api/orders/{storeId}/{orderNo}` →
 * `GET /admin/sellers/{storeId}/orders/{orderNo}`（`orders.read`，§6.1.10）。
 *
 * 收件資訊一律遮罩（手機保留前 4 後 3、地址止於 city ＋ district）。要看完整值走
 * `recipient-contact` 那一支，而那是**必落稽核**的獨立揭露動作。
 */

type RouteContext = { params: Promise<{ storeId: string; orderNo: string }> };

export const dynamic = "force-dynamic";

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, orderNo } = await context.params;
	return forwardDelegated(request, () => ({
		method: "GET",
		path: `/admin/sellers/${numericSegment(storeId)}/orders/${orderNoSegment(orderNo)}`,
		timeout: "mutation",
	}));
}
