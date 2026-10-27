import { forwardDelegated, numericSegment } from "../../../../_forward";

/**
 * `POST /adm/api/posts/{storeId}/{postId}/unpublish` →
 * `POST /admin/sellers/{storeId}/posts/{postId}/unpublish`（`posts.publish`，§6.1.9）。
 */

type RouteContext = { params: Promise<{ storeId: string; postId: string }> };

export const dynamic = "force-dynamic";

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, postId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "POST",
		path: `/admin/sellers/${numericSegment(storeId)}/posts/${numericSegment(postId)}/unpublish`,
		timeout: "mutation",
	}));
}
