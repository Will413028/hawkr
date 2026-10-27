/**
 * Three-step signed-URL image upload, mirroring the backend's
 * /app/media/upload_sessions flow used by mobile.
 *
 *   1. POST /app/media/upload_sessions
 *      → { sessionId, uploadUrl }   (presigned PUT, no auth header on it)
 *   2. PUT <uploadUrl>  body: file binary  Content-Type: <file.type>
 *   3. POST /app/media/upload_sessions/{sessionId}/complete
 *      → { previewUrl, stagedToken }
 *
 * `previewUrl` is the permanent CDN URL to render in <img> for display.
 * `stagedToken` is a short-lived token accepted by endpoints that take staged
 * uploads — the product gallery sends `stagedToken` (not previewUrl) for each
 * image; see lib/media/resolve-staged-images.ts.
 *
 * Step 2 uses XHR (not fetch) so we can stream upload progress back to the
 * UI — fetch() does not expose upload progress events in the browser.
 */

import { authedFetch } from "@/lib/api/client";
import type { RepoFetcher } from "@/lib/api/delegated";
import { buildImageUrl } from "@/lib/media/cdn";

interface CreateUploadSessionOutput {
	sessionId: string;
	uploadUrl: string;
	expiresIn: string;
	isSecure: boolean;
}

interface CompleteUploadSessionOutput {
	previewUrl: string;
	stagedToken: string;
}

export interface UploadedAsset {
	/** Preview/display URL for <img>. NOTE: product gallery sends `stagedToken`,
	 *  not this URL — see lib/media/resolve-staged-images.ts. */
	previewUrl: string;
	/** Short-lived token for endpoints that accept staged uploads (product gallery). */
	stagedToken: string;
}

/** Default max: 10MB. Backend likely caps lower; surface server error if so. */
const DEFAULT_MAX_BYTES = 10 * 1024 * 1024;
const IMAGE_COMPRESSION_TRIGGER_BYTES = 2 * 1024 * 1024;
const IMAGE_COMPRESSION_MAX_EDGE = 1600;
const IMAGE_COMPRESSION_QUALITY = 0.82;
const IMAGE_COMPRESSION_TYPE = "image/webp";

export interface UploadImageOptions {
	/** When false, skip the client-side resize/re-encode and upload the original
	 *  bytes (still capped at DEFAULT_MAX_BYTES). Product *description* images opt
	 *  out so high-detail story art / embedded text stays crisp; the gallery keeps
	 *  the default downscale to bound thumbnail weight. */
	compress?: boolean;
}

/**
 * 媒體上傳資料層的介面（56 號 §4.5 L1053；設計書 L1053 漏列了這一支）。
 *
 * 三步中只有 ①③ 打後端（代行走 `/adm/api/media/upload_sessions*`，§6.1.13），
 * ② 的 `PUT <uploadUrl>` 是直送物件儲存的預簽網址，兩條路徑一模一樣。
 */
export interface MediaUploadRepo {
	uploadImageFile(
		file: File,
		onProgress?: (ratio: number) => void,
		options?: UploadImageOptions,
	): Promise<UploadedAsset>;
}

export interface MediaUploadRepoOptions {
	/** 資源根。賣家 `/app/media/upload_sessions`。 */
	root?: string;
}

export function createMediaUploadRepo(
	fetcher: RepoFetcher,
	options: MediaUploadRepoOptions = {},
): MediaUploadRepo {
	const root = options.root ?? "/app/media/upload_sessions";
	return {
		uploadImageFile: (file, onProgress, uploadOptions) =>
			uploadImageFileWith(fetcher, root, file, onProgress, uploadOptions),
	};
}

/** 賣家路徑的預設實例；具名匯出 `uploadImageFile` 經由它。 */
export const mediaUploadRepo = createMediaUploadRepo(authedFetch);

export const uploadImageFile: MediaUploadRepo["uploadImageFile"] = (
	file,
	onProgress,
	options,
) => mediaUploadRepo.uploadImageFile(file, onProgress, options);

async function uploadImageFileWith(
	fetcher: RepoFetcher,
	root: string,
	file: File,
	onProgress?: (ratio: number) => void,
	options?: UploadImageOptions,
): Promise<UploadedAsset> {
	if (!file.type.startsWith("image/")) {
		throw new Error("僅支援圖片檔案 (JPEG / PNG / WebP)");
	}
	const uploadFile =
		options?.compress === false ? file : await prepareImageForUpload(file);
	if (uploadFile.size > DEFAULT_MAX_BYTES) {
		throw new Error(
			`檔案太大（${formatBytes(uploadFile.size)}），請壓縮到 10 MB 以下再試`,
		);
	}

	const sessionEnv = await fetcher<CreateUploadSessionOutput>(root, {
		method: "POST",
		json: {
			contentType: uploadFile.type,
			fileSize: String(uploadFile.size),
			filename: uploadFile.name,
		},
	});
	const session = sessionEnv.data;
	if (!session?.sessionId || !session.uploadUrl) {
		throw new Error("建立上傳工作階段失敗");
	}
	await putWithProgress(
		session.uploadUrl,
		uploadFile,
		uploadFile.type,
		onProgress,
	);

	const completeEnv = await fetcher<CompleteUploadSessionOutput>(
		`${root}/${encodeURIComponent(session.sessionId)}/complete`,
		{ method: "POST" },
	);
	const completed = completeEnv.data;
	// 同時要求 stagedToken：缺它時 fail-fast，避免下游（商品 gallery）把缺 token
	// 的新上傳圖誤判成既有圖而下載重傳。後端契約本就保證兩者皆回。
	if (!completed?.previewUrl || !completed.stagedToken) {
		throw new Error("上傳完成回應缺少 previewUrl 或 stagedToken");
	}
	return {
		// Several uploaders render this value before the owning entity is
		// finalized/refetched. Normalize at the API boundary so the preview is
		// always built on the configured media origin.
		previewUrl: buildImageUrl(completed.previewUrl, "original"),
		stagedToken: completed.stagedToken,
	};
}

type DecodedImage = ImageBitmap | HTMLImageElement;

async function prepareImageForUpload(file: File): Promise<File> {
	if (!canCompressImage(file)) return file;

	const decoded = await decodeImage(file).catch(() => null);
	if (!decoded) return file;

	try {
		const sourceWidth = getDecodedWidth(decoded);
		const sourceHeight = getDecodedHeight(decoded);
		const longestEdge = Math.max(sourceWidth, sourceHeight);
		const shouldResize = longestEdge > IMAGE_COMPRESSION_MAX_EDGE;
		const shouldCompress =
			shouldResize ||
			file.size > IMAGE_COMPRESSION_TRIGGER_BYTES ||
			file.size > DEFAULT_MAX_BYTES;

		if (!sourceWidth || !sourceHeight || !shouldCompress) return file;

		const scale = shouldResize ? IMAGE_COMPRESSION_MAX_EDGE / longestEdge : 1;
		const width = Math.max(1, Math.round(sourceWidth * scale));
		const height = Math.max(1, Math.round(sourceHeight * scale));
		const canvas = document.createElement("canvas");
		canvas.width = width;
		canvas.height = height;

		const ctx = canvas.getContext("2d");
		if (!ctx) return file;
		ctx.imageSmoothingEnabled = true;
		ctx.imageSmoothingQuality = "high";
		ctx.drawImage(decoded, 0, 0, width, height);

		const blob = await canvasToBlob(
			canvas,
			IMAGE_COMPRESSION_TYPE,
			IMAGE_COMPRESSION_QUALITY,
		);
		if (!blob) return file;
		if (blob.size >= file.size) return file;

		return new File([blob], withImageExtension(file.name, blob.type), {
			type: blob.type || IMAGE_COMPRESSION_TYPE,
			lastModified: file.lastModified,
		});
	} finally {
		closeDecodedImage(decoded);
	}
}

function canCompressImage(file: File): boolean {
	if (typeof document === "undefined") return false;
	if (!file.type.startsWith("image/")) return false;
	return file.type !== "image/gif" && file.type !== "image/svg+xml";
}

async function decodeImage(file: File): Promise<DecodedImage> {
	if (typeof createImageBitmap === "function") {
		try {
			return await createImageBitmap(file);
		} catch {
			// Fall through to the HTMLImageElement path for browsers/codecs that
			// do not support createImageBitmap for a given file.
		}
	}
	return loadHtmlImage(file);
}

function loadHtmlImage(file: File): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const url = URL.createObjectURL(file);
		const image = new Image();
		image.onload = () => {
			URL.revokeObjectURL(url);
			resolve(image);
		};
		image.onerror = () => {
			URL.revokeObjectURL(url);
			reject(new Error("圖片讀取失敗"));
		};
		image.src = url;
	});
}

function getDecodedWidth(image: DecodedImage): number {
	return "naturalWidth" in image ? image.naturalWidth : image.width;
}

function getDecodedHeight(image: DecodedImage): number {
	return "naturalHeight" in image ? image.naturalHeight : image.height;
}

function closeDecodedImage(image: DecodedImage): void {
	if ("close" in image && typeof image.close === "function") {
		image.close();
	}
}

function canvasToBlob(
	canvas: HTMLCanvasElement,
	type: string,
	quality: number,
): Promise<Blob | null> {
	return new Promise((resolve) => {
		canvas.toBlob((blob) => resolve(blob?.size ? blob : null), type, quality);
	});
}

function withImageExtension(filename: string, contentType: string): string {
	const baseName = filename.trim() || "upload";
	const extension =
		contentType === "image/png"
			? ".png"
			: contentType === "image/jpeg"
				? ".jpg"
				: ".webp";
	return /\.[^./\\]+$/.test(baseName)
		? baseName.replace(/\.[^./\\]+$/, extension)
		: `${baseName}${extension}`;
}

function putWithProgress(
	url: string,
	file: File,
	contentType: string,
	onProgress?: (ratio: number) => void,
): Promise<void> {
	return new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open("PUT", url);
		if (contentType) xhr.setRequestHeader("Content-Type", contentType);
		xhr.upload.onprogress = (e) => {
			if (!onProgress || !e.lengthComputable) return;
			onProgress(e.loaded / e.total);
		};
		xhr.onload = () => {
			if (xhr.status >= 200 && xhr.status < 300) {
				resolve();
			} else {
				reject(new Error(`上傳失敗（HTTP ${xhr.status}）`));
			}
		};
		xhr.onerror = () => reject(new Error("上傳失敗（網路錯誤）"));
		xhr.onabort = () => reject(new Error("上傳已取消"));
		xhr.send(file);
	});
}

function formatBytes(n: number): string {
	if (n < 1024) return `${n} B`;
	if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
	return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
