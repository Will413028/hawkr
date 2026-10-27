import { forwardDelegated, numericSegment } from "../../../../_forward";

/**
 * `POST /adm/api/products/{storeId}/{productId}/unpublish` →
 * `POST /admin/sellers/{storeId}/products/{productId}/unpublish`
 * （`products.publish`，§6.1.8）。
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
		path: `/admin/sellers/${numericSegment(storeId)}/products/${numericSegment(productId)}/unpublish`,
		timeout: "mutation",
	}));
}
