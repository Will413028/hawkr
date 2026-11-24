"use client";

import { createClient } from "@connectrpc/connect";
import {
	type RejectReason,
	uploadPublicFile,
	uploadSecureFile,
} from "@hawkr/api/connect/upload";
import { MediaAppService } from "@hawkr/api/gen/hawkr/media/v1alpha1/media_pb";
import { mediaRejectReason } from "@hawkr/ui-text/keys";
import { adminTransport } from "@/lib/connect/transport";
import { staffTextOrUnknown } from "@/lib/ui-text";

/** 媒體服務拒收檔案；message 是共用文案目錄的原因句，可直接顯示給員工。 */
export class MediaRejectedError extends Error {
	readonly reason: RejectReason;
	constructor(reason: RejectReason) {
		super(staffTextOrUnknown(mediaRejectReason(reason)));
		this.name = "MediaRejectedError";
		this.reason = reason;
	}
}

/** 被拒收時顯示原因句，其他上傳錯誤（內部字串）用呼叫端的固定句。 */
export function uploadFailureText(caught: unknown, fallback: string): string {
	return caught instanceof MediaRejectedError ? caught.message : fallback;
}

/** 上傳圖片並回暫存 token（給建立或更新的 RPC）；機敏檔走分段上傳。 */
export async function uploadImageToMedia(
	file: File,
	options: { isSecure?: boolean } = {},
): Promise<string> {
	const client = createClient(MediaAppService, adminTransport());
	if (options.isSecure) {
		const secure = await uploadSecureFile(client, file, {
			chunkPath: "/api/bff/api/files/secure_uploads",
		});
		return secure.stagedToken;
	}
	const outcome = await uploadPublicFile(client, file);
	if (outcome.state === "rejected")
		throw new MediaRejectedError(outcome.reason);
	return outcome.stagedToken;
}
