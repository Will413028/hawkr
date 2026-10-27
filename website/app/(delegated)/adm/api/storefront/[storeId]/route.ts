import { forwardDelegated, numericSegment } from "../../_forward";

/**
 * 店面（§6.1.7）：
 * - `GET`  → `GET /admin/sellers/{storeId}/storefront`（`storefront.read`）
 * - `PATCH` → `PATCH /admin/sellers/{storeId}/storefront`（`storefront.write`）
 *
 * 四條店面路由從 gateway 步驟 9 起固定回 503，到 A10 以 Connect 的代行契約恢復；BFF 照舊轉發。
 */

type RouteContext = { params: Promise<{ storeId: string }> };

export const dynamic = "force-dynamic";

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "GET",
		path: `/admin/sellers/${numericSegment(storeId)}/storefront`,
		timeout: "mutation",
	}));
}

export async function PATCH(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "PATCH",
		path: `/admin/sellers/${numericSegment(storeId)}/storefront`,
		timeout: "mutation",
	}));
}
