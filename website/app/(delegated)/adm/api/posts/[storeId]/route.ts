import { forwardDelegated, numericSegment } from "../../_forward";

/**
 * 貼文列表與建立（§6.1.9）：
 * - `GET`  → `GET /admin/sellers/{storeId}/posts`（`posts.read`）
 * - `POST` → `POST /admin/sellers/{storeId}/posts`（`posts.write`）
 *
 * ⚠️ 列表走 admin 私有讀取，**不得**用公開貼文端點兜底：草稿與排程中的貼文在
 * public 路徑上根本查不到，而那正是代行要處理的那一批。
 */

type RouteContext = { params: Promise<{ storeId: string }> };

/** Gateway `ListDelegatedPosts` 實際讀取的三個 key（沒有 `keyword`）。 */
const LIST_QUERY = ["page", "page_size", "status"] as const;

export const dynamic = "force-dynamic";

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "GET",
		path: `/admin/sellers/${numericSegment(storeId)}/posts`,
		timeout: "mutation",
		query: LIST_QUERY,
	}));
}

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "POST",
		path: `/admin/sellers/${numericSegment(storeId)}/posts`,
		timeout: "mutation",
	}));
}
