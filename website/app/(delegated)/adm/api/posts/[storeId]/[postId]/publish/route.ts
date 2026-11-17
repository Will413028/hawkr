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
	scheduleOf,
} from "../../../_post-wire";

/**
 * `POST /adm/api/posts/{storeId}/{postId}/publish`（`posts.publish`）：
 * body 帶 `publishMode: "scheduled"` 與 `scheduledPublishAt` 時打 `ScheduleStorePost`（排程），否則打
 * `PublishStorePost`（立即發布）；稽核動作由伺服器依呼叫的方法決定。
 */

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
				const publishTime = scheduleOf(body);
				if (publishTime !== undefined) {
					return async (client, options) => {
						const message = await client.scheduleStorePost(
							{ operation, postId: pathPostId, etag, publishTime },
							options,
						);
						return mutationDto(message.post, message.auditLogId);
					};
				}
				return async (client, options) => {
					const message = await client.publishStorePost(
						{ operation, postId: pathPostId, etag },
						options,
					);
					return mutationDto(message.post, message.auditLogId);
				};
			},
		};
	});
}
