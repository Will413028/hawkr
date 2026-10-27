import { forwardDelegated, numericSegment } from "../../../_forward";

/**
 * `POST /adm/api/storefront/{storeId}/cover` →
 * `POST /admin/sellers/{storeId}/storefront/cover`（`storefront.write`，§6.1.7）。
 */

type RouteContext = { params: Promise<{ storeId: string }> };

export const dynamic = "force-dynamic";

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "POST",
		path: `/admin/sellers/${numericSegment(storeId)}/storefront/cover`,
		timeout: "mutation",
	}));
}
