import { forwardDelegated, mediaSessionSegment } from "../../../../_forward";

/**
 * `POST /adm/api/media/upload_sessions/{sessionId}/complete` →
 * `POST /admin/delegated-operations/media/upload_sessions/{sessionId}/complete`
 * （§6.1.13 步 ③）。
 *
 * 回的是 `stagedToken` —— **JWT 形狀的短期憑證**：不進 log、不進稽核快照、不進
 * `changed_fields`，只由呼叫端當場交給 storefront／products／posts 的寫入端點。
 *
 * ⚠️ 這一支不能靠上一列的 `upload_sessions` 兜住（禁 catch-all），漏建就是工作區的
 * 上傳按鈕拿得到 uploadUrl、傳得上 R2，然後在最後一步 404。
 */

type RouteContext = { params: Promise<{ sessionId: string }> };

export const dynamic = "force-dynamic";

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { sessionId } = await context.params;
	return forwardDelegated(request, () => ({
		method: "POST",
		path: `/admin/delegated-operations/media/upload_sessions/${mediaSessionSegment(sessionId)}/complete`,
		timeout: "mutation",
	}));
}
