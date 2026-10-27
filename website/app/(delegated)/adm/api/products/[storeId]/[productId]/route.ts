import { forwardDelegated, numericSegment } from "../../../_forward";

/**
 * 單一商品（§6.1.8）：
 * - `GET`   → `GET /admin/sellers/{storeId}/products/{productId}`（`products.read`）
 * - `PATCH` → `PATCH /admin/sellers/{storeId}/products/{productId}`（`products.write`）
 *
 * ⚠️ `images` 與 `skus` 是**整份替換**：要保留既有圖就得把原圖的 `assetId` 一起回送
 * （新上傳的那些走 `stagedToken`）。漏送的後果是「只改一個價格」把商品既有圖片全部
 * 清空，而請求完全合法、`expectedVersion` 相符、回 `code: 0`。
 */

type RouteContext = { params: Promise<{ storeId: string; productId: string }> };

export const dynamic = "force-dynamic";

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, productId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "GET",
		path: `/admin/sellers/${numericSegment(storeId)}/products/${numericSegment(productId)}`,
		timeout: "mutation",
	}));
}

export async function PATCH(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId, productId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "PATCH",
		path: `/admin/sellers/${numericSegment(storeId)}/products/${numericSegment(productId)}`,
		timeout: "mutation",
	}));
}
