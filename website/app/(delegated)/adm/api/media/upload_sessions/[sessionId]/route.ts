import { MediaDelegatedService } from "@hawkr/api/gen/hawkr/media/v1alpha1/media_pb";
import { forwardDelegatedConnect, stagedTokenSegment } from "../../../_forward";

/**
 * `GET /adm/api/media/upload_sessions/{stagedToken}` → `MediaDelegatedService.GetUpload`：
 * 回 `{ state, rejectReason }`（proto enum 的數字值），瀏覽器輪詢到 READY 或 REJECTED。
 */

type RouteContext = { params: Promise<{ sessionId: string }> };

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { sessionId } = await context.params;
	return forwardDelegatedConnect(request, () => {
		const stagedToken = stagedTokenSegment(sessionId);
		return {
			service: MediaDelegatedService,
			method: "GET",
			timeout: "mutation",
			build: () => async (client, options) => {
				const res = await client.getUpload({ stagedToken }, options);
				return { state: res.state, rejectReason: res.rejectReason ?? null };
			},
		};
	});
}
