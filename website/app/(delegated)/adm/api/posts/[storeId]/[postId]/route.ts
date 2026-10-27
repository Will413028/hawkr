import { forwardDelegated, numericSegment } from "../../../_forward";

/**
 * 單一貼文（§6.1.9）：
 * - `GET`   → `GET /admin/sellers/{storeId}/posts/{postId}`（`posts.read`）
 * - `PATCH` → `PATCH /admin/sellers/{storeId}/posts/{postId}`（`posts.write`）
 */

type RouteContext = { params: Promise<{ storeId: string; postId: string }> };

export const dynamic = "force-dynamic";

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, postId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "GET",
		path: `/admin/sellers/${numericSegment(storeId)}/posts/${numericSegment(postId)}`,
		timeout: "mutation",
	}));
}

export async function PATCH(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, postId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "PATCH",
		path: `/admin/sellers/${numericSegment(storeId)}/posts/${numericSegment(postId)}`,
		timeout: "mutation",
	}));
}
