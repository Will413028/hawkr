import { forwardDelegated, numericSegment } from "../../_forward";

/**
 * 商品列表與建立（§6.1.8）：
 * - `GET`  → `GET /admin/sellers/{storeId}/products`（`products.read`）
 * - `POST` → `POST /admin/sellers/{storeId}/products`（`products.write`）
 *
 * ⚠️ 四張 KPI 卡（總商品數／上架中／庫存預警／草稿）**只能用本端點回應的 `total`**
 * 或不渲染。為它們接一支統計 RPC 是本案最容易被靜默鑿開的洞：不碰 `delegatedRoutes`
 * registry、不觸發 boot 斷言、不需要新 capability —— CI 全綠而攻擊面已經擴大。
 */

type RouteContext = { params: Promise<{ storeId: string }> };

/** Gateway `ListDelegatedProducts` 實際讀取的四個 key，多一個少一個都不對。 */
const LIST_QUERY = ["page", "page_size", "keyword", "status"] as const;

export const dynamic = "force-dynamic";

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "GET",
		path: `/admin/sellers/${numericSegment(storeId)}/products`,
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
		path: `/admin/sellers/${numericSegment(storeId)}/products`,
		timeout: "mutation",
	}));
}
