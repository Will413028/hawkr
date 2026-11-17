import {
	forwardDelegatedConnect,
	numericSegment,
	postIdSegment,
} from "../../../../_forward";
import {
	bodyRecord,
	etagOf,
	mutationDto,
	operationOf,
} from "../../../_post-wire";

/** `POST /adm/api/posts/{storeId}/{postId}/unpublish` → `SocialDelegatedService.UnpublishStorePost`（`posts.publish`）。 */

type RouteContext = { params: Promise<{ storeId: string; postId: string }> };

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, postId } = await context.params;
	return forwardDelegatedConnect(request, () => {
		const pathStoreId = numericSegment(storeId);
		const pathPostId = postIdSegment(postId);
		return {
			storeId: pathStoreId,
			method: "POST",
			timeout: "mutation",
			build: (input) => {
				const body = bodyRecord(input);
				const operation = operationOf(input, body);
				const etag = etagOf(body);
				return async (client, options) => {
					const message = await client.unpublishStorePost(
						{ operation, postId: pathPostId, etag },
						options,
					);
					return mutationDto(message.post, message.auditLogId);
				};
			},
		};
	});
}
