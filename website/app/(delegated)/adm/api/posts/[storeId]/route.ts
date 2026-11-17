import { forwardDelegatedConnect, numericSegment } from "../../_forward";
import {
	bodyRecord,
	createFields,
	listDto,
	mutationDto,
	operationOf,
	pageSizeOf,
	statusFilter,
} from "../_post-wire";

/**
 * 貼文列表與建立：
 * - `GET`  → `SocialDelegatedService.ListStorePosts`（`posts.read`）
 * - `POST` → `SocialDelegatedService.CreateStorePost`（`posts.write`，建立一律是草稿）
 *
 * `:storeId` 先過值域檢查，轉送點在送出前再驗它就是 session 綁定的店（不符 403／9262，不打 Connect）；Connect 請求不帶 store_id。
 * 列表是 page_token 分頁（`page_size`、`page_token`、`status`），沒有頁碼與總數。
 */

type RouteContext = { params: Promise<{ storeId: string }> };

const LIST_QUERY = ["page_size", "page_token", "status"] as const;

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegatedConnect(request, () => {
		const pathStoreId = numericSegment(storeId);
		return {
			storeId: pathStoreId,
			method: "GET",
			timeout: "mutation",
			query: LIST_QUERY,
			build: ({ query }) => {
				const request = {
					pageSize: pageSizeOf(query),
					pageToken: query.get("page_token") ?? "",
					status: statusFilter(query),
				};
				return async (client, options) => {
					const message = await client.listStorePosts(request, options);
					return listDto(message.posts, message.nextPageToken);
				};
			},
		};
	});
}

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegatedConnect(request, () => {
		const pathStoreId = numericSegment(storeId);
		return {
			storeId: pathStoreId,
			method: "POST",
			timeout: "mutation",
			build: (input) => {
				const body = bodyRecord(input);
				const operation = operationOf(input, body);
				const fields = createFields(body);
				return async (client, options) => {
					const message = await client.createStorePost(
						{ operation, ...fields },
						options,
					);
					return mutationDto(message.post, message.auditLogId);
				};
			},
		};
	});
}
