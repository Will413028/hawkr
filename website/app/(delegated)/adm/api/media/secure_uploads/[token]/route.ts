import {
	forwardDelegatedFile,
	secureUploadTokenSegment,
} from "../../../_forward";

/**
 * `GET /adm/api/media/secure_uploads/{token}` →
 * `GET /api/files/delegated/secure_uploads/{token}`：確認前的暫存預覽，只有開會話的那個代行 session 讀得到。
 */

type RouteContext = { params: Promise<{ token: string }> };

export async function GET(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { token } = await context.params;
	return forwardDelegatedFile(request, () => ({
		method: "GET",
		path: `/api/files/delegated/secure_uploads/${secureUploadTokenSegment(token)}`,
	}));
}
