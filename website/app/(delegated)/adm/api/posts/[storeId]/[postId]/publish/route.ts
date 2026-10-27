import { forwardDelegated, numericSegment } from "../../../../_forward";

/**
 * `POST /adm/api/posts/{storeId}/{postId}/publish` →
 * `POST /admin/sellers/{storeId}/posts/{postId}/publish`（`posts.publish`，§6.1.9）。
 *
 * 排程發布（`scheduledPublishAt`）也走這一支；稽核 action 由後端依 body 決定
 * （`POST_PUBLISH`／`POST_SCHEDULE`），前端不區分端點。
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
		path: `/admin/sellers/${numericSegment(storeId)}/posts/${numericSegment(postId)}/publish`,
		timeout: "mutation",
	}));
}
