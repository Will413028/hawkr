import {
	chunkIndexSegment,
	forwardDelegatedFile,
	secureUploadTokenSegment,
} from "../../../../../_forward";

/**
 * `PUT /adm/api/media/secure_uploads/{token}/chunks/{index}` →
 * `PUT /api/files/delegated/secure_uploads/{token}/chunks/{index}`（單體檔案端點，B1-D16／D17）。
 *
 * body 是原始位元組（一段，上限 4 MiB），原樣轉送；授權只有 operation credential（cookie → `X-HK-Operation-Session`），
 * 上傳者與店家由單體從 session 取得。
 */

type RouteContext = { params: Promise<{ token: string; index: string }> };

export async function PUT(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { token, index } = await context.params;
	return forwardDelegatedFile(request, () => ({
		method: "PUT",
		path: `/api/files/delegated/secure_uploads/${secureUploadTokenSegment(token)}/chunks/${chunkIndexSegment(index)}`,
	}));
}
