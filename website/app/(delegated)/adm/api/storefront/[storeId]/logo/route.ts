import { forwardDelegated, numericSegment } from "../../../_forward";

/**
 * `POST /adm/api/storefront/{storeId}/logo` →
 * `POST /admin/sellers/{storeId}/storefront/logo`（`storefront.write`，§6.1.7）。
 *
 * body 收的是 `stagedToken`（媒體三步的產物），不是檔案位元組 —— 位元組在步 ②
 * 由瀏覽器直送 R2，一次都不經過這裡。
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
		path: `/admin/sellers/${numericSegment(storeId)}/storefront/logo`,
		timeout: "mutation",
	}));
}
