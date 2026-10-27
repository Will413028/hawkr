import { forwardDelegated, numericSegment } from "../../../_forward";

/**
 * `POST /adm/api/products/{storeId}/batch-unpublish` →
 * `POST /admin/sellers/{storeId}/products/batch-unpublish`（`products.publish`，§6.1.8）。
 *
 * **唯一走 60s 檔的端點**（§7.3.1 ②）：那一檔的存在理由是「批次 N=50 在服務端的單
 * 交易工作量」，不是網路慢。
 *
 * ⚠️ capability 是 `products.publish`，不是任何「批次」專用值 —— 批次與逐筆做的是
 * 同一件事，只有跳數不同。整批共用**一個** `operationReason`，不逐項。
 *
 * ⚠️ 這條路徑是靜態段，與同層的 `[productId]` 併存：Next 的靜態段優先，而商品 id
 * 恆為數字 ⇒ 不會有一個叫 `batch-unpublish` 的商品把它遮掉。
 */

type RouteContext = { params: Promise<{ storeId: string }> };

export const dynamic = "force-dynamic";

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { storeId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "POST",
		path: `/admin/sellers/${numericSegment(storeId)}/products/batch-unpublish`,
		timeout: "batch",
	}));
}
