import { DelegatedRequestRejectedError } from "../../../_csrf";
import { forwardDelegatedConnect, numericSegment } from "../../../_forward";
import {
	bodyRecord,
	etagOf,
	mutationDto,
	operationOf,
	postDto,
	updateFields,
} from "../../_post-wire";

/**
 * 單一貼文：
 * - `GET`   → `SocialDelegatedService.GetStorePost`（`posts.read`）
 * - `PATCH` → `SocialDelegatedService.UpdateStorePost`（`posts.write`；etag 取自 body 的 `expectedVersion`）
 *
 * 貼文 ID 是 UUID（社群模組）；`:storeId` 只做值域檢查，目標店家來自代行 session。
 */

type RouteContext = { params: Promise<{ storeId: string; postId: string }> };

const POST_ID =
	/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function postIdSegment(value: string): string {
	if (!POST_ID.test(value)) {
		throw new DelegatedRequestRejectedError("INVALID_PATH_PARAM", 400);
	}
	return value;
}

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, postId } = await context.params;
	return forwardDelegatedConnect(request, () => {
		numericSegment(storeId);
		const id = postIdSegment(postId);
		return {
			method: "GET",
			timeout: "mutation",
			build: () => async (client, options) => {
				const message = await client.getStorePost({ postId: id }, options);
				return message.post ? postDto(message.post) : null;
			},
		};
	});
}

export async function PATCH(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, postId } = await context.params;
	return forwardDelegatedConnect(request, () => {
		numericSegment(storeId);
		const id = postIdSegment(postId);
		return {
			method: "PATCH",
			timeout: "mutation",
			build: (input) => {
				const body = bodyRecord(input);
				const operation = operationOf(input, body);
				const { post, updateMask } = updateFields(id, etagOf(body), body);
				return async (client, options) => {
					const message = await client.updateStorePost(
						{ operation, post, updateMask },
						options,
					);
					return mutationDto(message.post, message.auditLogId);
				};
			},
		};
	});
}
