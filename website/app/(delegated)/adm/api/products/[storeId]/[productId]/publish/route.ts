import { forwardDelegated, numericSegment } from "../../../../_forward";

/**
 * `POST /adm/api/products/{storeId}/{productId}/publish` →
 * `POST /admin/sellers/{storeId}/products/{productId}/publish`
 * （`products.publish`，§6.1.8）。
 *
 * ⚠️ 代行**沒有**賣家側那種「傳 status 值」的形狀：上架與下架是兩支命令端點，body
 * 只收 `expectedVersion`／`operationReason`／`notifySeller`。
 */

type RouteContext = { params: Promise<{ storeId: string; productId: string }> };

export const dynamic = "force-dynamic";

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, productId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "POST",
		path: `/admin/sellers/${numericSegment(storeId)}/products/${numericSegment(productId)}/publish`,
		timeout: "mutation",
	}));
}
