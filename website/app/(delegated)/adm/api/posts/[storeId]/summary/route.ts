import { forwardDelegatedConnect, numericSegment } from "../../../_forward";
import { summaryDto } from "../../_post-wire";

/**
 * `GET /adm/api/posts/{storeId}/summary` → `SocialDelegatedService.GetStoreSocialSummary`（`posts.read`）：
 * 代行工作區的動態數，由伺服器以資料庫計數（每個狀態一項，不含已刪除）。
 */

type RouteContext = { params: Promise<{ storeId: string }> };

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
			build: () => async (client, options) =>
				summaryDto(await client.getStoreSocialSummary({}, options)),
		};
	});
}
