import { MediaDelegatedService } from "@hawkr/api/gen/hawkr/media/v1alpha1/media_pb";
import { DelegatedRequestRejectedError } from "../../_csrf";
import { forwardDelegatedConnect } from "../../_forward";

/**
 * `POST /adm/api/media/upload_sessions` → `MediaDelegatedService.BeginUpload`：公開圖片上傳的第一步。
 * 回 `{ stagedToken, uploadUrl, uploadHeaders }`；第二步 `PUT <uploadUrl>` 由瀏覽器直送物件儲存，不經任何 BFF。
 *
 * 店家由代行 session 決定，路徑沒有 storeId，所以不做綁定店比對；不掛 capability（真正要授權的是把圖掛上去的
 * 寫入端點）。逾時走單筆 mutation 檔：經過這裡的只有 JSON metadata。
 */

interface BeginBody {
	size: unknown;
	contentType: unknown;
	md5: unknown;
}

export async function POST(request: Request): Promise<Response> {
	return forwardDelegatedConnect(request, {
		service: MediaDelegatedService,
		method: "POST",
		timeout: "mutation",
		build: ({ json }) => {
			const body = (json ?? {}) as BeginBody;
			if (
				typeof body.size !== "string" ||
				!/^[1-9][0-9]{0,15}$/.test(body.size) ||
				typeof body.contentType !== "string" ||
				typeof body.md5 !== "string"
			) {
				throw new DelegatedRequestRejectedError("INVALID_BODY", 400);
			}
			const size = BigInt(body.size);
			const { contentType, md5 } = body;
			return async (client, options) => {
				const res = await client.beginUpload(
					{ size, contentType, md5 },
					options,
				);
				return {
					stagedToken: res.stagedToken,
					uploadUrl: res.uploadUrl,
					uploadHeaders: res.uploadHeaders,
				};
			};
		},
	});
}
