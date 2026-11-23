/**
 * 機敏文件（KYC 證件、佐證檔等）的分段上傳：`@hawkr/api/connect/upload` 的 BeginSecureUpload，之後依伺服器給的
 * 段大小依序 PUT 到單體的 `/api/files/secure_uploads/{token}/chunks/{index}`（同源 session cookie，伺服器端加密後才落地）。
 * 沒有公開預覽網址；呼叫端只拿到 token，在送出認證或申覆時交給擁有者的 API。
 */

import { createClient } from "@connectrpc/connect";
import { uploadSecureFile } from "@hawkr/api/connect/upload";
import { MediaAppService } from "@hawkr/api/gen/hawkr/media/v1alpha1/media_pb";
import { authedTransport } from "@/lib/connect/transport";

export interface SecureUploadResult {
	/** Short-lived token to pass into submit/save payloads (`*_staged_token`). */
	stagedToken: string;
	/** 與 stagedToken 相同，除錯時對照用。 */
	referenceId: string;
}

/** Default max: 10 MB. KYC docs rarely exceed this; backend may cap lower. */
const DEFAULT_MAX_BYTES = 10 * 1024 * 1024;

export async function uploadSecureDocument(
	file: File,
	_docType: string,
	onProgress?: (ratio: number) => void,
): Promise<SecureUploadResult> {
	if (!file.type.startsWith("image/") && file.type !== "application/pdf") {
		throw new Error("僅支援圖片或 PDF 檔案");
	}
	if (file.size > DEFAULT_MAX_BYTES) {
		throw new Error(
			`檔案太大（${formatBytes(file.size)}），請壓縮到 10 MB 以下再試`,
		);
	}
	const client = createClient(MediaAppService, authedTransport());
	const { stagedToken } = await uploadSecureFile(client, file, {
		onProgress,
	});
	return { stagedToken, referenceId: stagedToken };
}

function formatBytes(n: number): string {
	if (n < 1024) return `${n} B`;
	if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
	return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
