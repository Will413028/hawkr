import { DelegatedRequestRejectedError } from "../../../../_csrf";
import { forwardDelegatedConnect, numericSegment } from "../../../../_forward";
import {
	bodyRecord,
	etagOf,
	mutationDto,
	operationOf,
} from "../../../_post-wire";

/** `POST /adm/api/posts/{storeId}/{postId}/unpublish` → `SocialDelegatedService.UnpublishStorePost`（`posts.publish`）。 */

type RouteContext = { params: Promise<{ storeId: string; postId: string }> };

const POST_ID =
	/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, postId } = await context.params;
	return forwardDelegatedConnect(request, () => {
		const pathStoreId = numericSegment(storeId);
		if (!POST_ID.test(postId)) {
			throw new DelegatedRequestRejectedError("INVALID_PATH_PARAM", 400);
		}
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
						{ operation, postId, etag },
						options,
					);
					return mutationDto(message.post, message.auditLogId);
				};
			},
		};
	});
}
