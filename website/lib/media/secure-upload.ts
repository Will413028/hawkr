/**
 * Two-step secure upload for sensitive documents (KYC ID card, bank
 * statements, business registration certs, etc.).
 *
 * Mirrors `hawkr_seller/lib/features/media/data/media_repository.dart`
 * (`uploadPrivateDocument`):
 *
 *   1. POST /app/media/upload_sessions  body: { isSecure: true, ... }
 *      → { sessionId, expiresIn, ... }
 *   2. PUT  /app/media/secure_upload/{sessionId}  body: file bytes
 *      → { stagedToken }
 *
 * Differs from the public flow (`lib/media/upload.ts`):
 *   - The PUT goes directly to the gateway with the same-origin session cookie
 *     (`__Host-hk_session`), NOT a presigned OSS URL — the gateway
 *     server-side encrypts before persisting.
 *   - There's no "complete" call — the secure PUT itself returns the
 *     stagedToken in its response body.
 *   - No public `previewUrl` is exposed; the caller only gets a token to
 *     pass into the certification submit body.
 *
 * Caller passes the token into `submitCertification({ idCardFrontStagedToken,
 * bankBookStagedToken, … })` etc. Backend resolves the token to the
 * encrypted asset at submit time.
 */

import { authedFetch } from "@/lib/api/client";
import { env } from "@/lib/config/env";

interface CreateSecureSessionOutput {
	sessionId: string;
	expiresIn?: string;
	objectKey?: string;
	isSecure?: boolean;
}

interface SecurePutOutput {
	stagedToken: string;
}

export interface SecureUploadResult {
	/** Short-lived token to pass into submit/save payloads (`*_staged_token`). */
	stagedToken: string;
	/** Server-side reference (sessionId). Useful for retries / debugging. */
	referenceId: string;
}

/** Default max: 10 MB. KYC docs rarely exceed this; backend may cap lower. */
const DEFAULT_MAX_BYTES = 10 * 1024 * 1024;

export async function uploadSecureDocument(
	file: File,
	docType: string,
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

	const sessionEnv = await authedFetch<CreateSecureSessionOutput>(
		"/app/media/upload_sessions",
		{
			method: "POST",
			json: {
				isSecure: true,
				docType,
				contentType: file.type,
				fileSize: String(file.size),
				filename: file.name,
			},
		},
	);
	const session = sessionEnv.data;
	if (!session?.sessionId) {
		throw new Error("建立安全上傳工作階段失敗");
	}

	// Step 2: PUT raw bytes directly to the gateway (NOT a presigned URL —
	// the server needs to see the body to encrypt at rest).
	const stagedToken = await putSecure(
		`/app/media/secure_upload/${encodeURIComponent(session.sessionId)}`,
		file,
		onProgress,
	);

	return { stagedToken, referenceId: session.sessionId };
}

/**
 * PUT raw bytes directly to the gateway with the same-origin session cookie
 * (XHR for upload progress, which fetch cannot report).
 */
async function putSecure(
	gatewayPath: string,
	file: File,
	onProgress?: (ratio: number) => void,
): Promise<string> {
	return new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open("PUT", `${env.apiBaseUrl}${gatewayPath}`);
		xhr.withCredentials = true;
		xhr.setRequestHeader("Content-Type", file.type);
		xhr.upload.onprogress = (e) => {
			if (!onProgress || !e.lengthComputable) return;
			onProgress(e.loaded / e.total);
		};
		xhr.onload = () => {
			if (xhr.status >= 200 && xhr.status < 300) {
				try {
					const body = JSON.parse(xhr.responseText) as {
						data?: SecurePutOutput;
					};
					const token = body.data?.stagedToken;
					if (!token) {
						reject(new Error("安全上傳完成但未取得 stagedToken"));
						return;
					}
					resolve(token);
				} catch {
					reject(new Error("安全上傳回應解析失敗"));
				}
			} else {
				reject(new Error(`安全上傳失敗（HTTP ${xhr.status}）`));
			}
		};
		xhr.onerror = () => reject(new Error("安全上傳失敗（網路錯誤）"));
		xhr.onabort = () => reject(new Error("安全上傳已取消"));
		xhr.send(file);
	});
}

function formatBytes(n: number): string {
	if (n < 1024) return `${n} B`;
	if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
	return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
