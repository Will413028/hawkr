/**
 * 公開圖片上傳：呼叫 `@hawkr/api/connect/upload` 的流程（BeginUpload、預簽 PUT、CompleteUpload、GetUpload 輪詢）。
 *
 * 賣家與買家走 `MediaAppService`（同源 session cookie）；代行營運走 `/adm/api/media/upload_sessions*` 的 BFF，
 * 由它帶代行憑證轉給 `MediaDelegatedService`。預簽 PUT 用 XHR 回報上傳進度。
 *
 * `previewUrl` 是選取檔案的本機 object URL：伺服器在實體確定前不給預覽網址，畫面照舊能先顯示縮圖。
 * `stagedToken` 是交給擁有者端點（商品圖庫等）的短期憑證，見 lib/media/resolve-staged-images.ts。
 * 被拒絕（REJECTED）丟出的錯誤訊息是 `@hawkr/ui-text` 的 RejectReason 文字。
 */

import { createClient } from "@connectrpc/connect";
import {
	type DelegatedPublicUploadClient,
	type PublicUploadClient,
	type PutTransport,
	uploadDelegatedPublicFile,
	uploadPublicFile,
} from "@hawkr/api/connect/upload";
import { MediaAppService } from "@hawkr/api/gen/hawkr/media/v1alpha1/media_pb";
import { createUiText, type Translate } from "@hawkr/ui-text/format";
import { commonUnknown, mediaRejectReason } from "@hawkr/ui-text/keys";
import { createTranslator } from "next-intl";
import type { RepoFetcher } from "@/lib/api/delegated";
import { authedTransport } from "@/lib/connect/transport";
import { activeLocale } from "@/lib/intl/active-locale";
import { messagesFor } from "@/lib/intl/config";

export interface UploadedAsset {
	/** 選取檔案的本機 object URL，供 <img> 顯示；不是伺服器網址，也不要送給後端。 */
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

/** 媒體上傳資料層的介面。 */
export interface MediaUploadRepo {
	uploadImageFile(
		file: File,
		onProgress?: (ratio: number) => void,
		options?: UploadImageOptions,
	): Promise<UploadedAsset>;
}

export interface MediaUploadRepoOptions {
	/** 代行 BFF 的資源根（`/media/upload_sessions`）；只在傳入 fetcher 時使用。 */
	root?: string;
}

type PublicUploader = (
	file: File,
	onProgress?: (ratio: number) => void,
) => ReturnType<typeof uploadPublicFile>;

/**
 * 不帶 fetcher 是賣家與買家路徑（`MediaAppService`）；帶 fetcher（代行的 `delegatedFetch`）時三步都走 BFF 路徑。
 */
export function createMediaUploadRepo(
	fetcher?: RepoFetcher,
	options: MediaUploadRepoOptions = {},
): MediaUploadRepo {
	const upload: PublicUploader = fetcher
		? (file, onProgress) =>
				uploadDelegatedPublicFile(
					delegatedUploadClient(
						fetcher,
						options.root ?? "/media/upload_sessions",
					),
					file,
					{ onProgress, put: xhrPut },
				)
		: (file, onProgress) =>
				uploadPublicFile(appUploadClient(), file, {
					onProgress,
					put: xhrPut,
				});
	return {
		uploadImageFile: (file, onProgress, uploadOptions) =>
			uploadImageFileWith(upload, file, onProgress, uploadOptions),
	};
}

/** 賣家路徑的預設實例；具名匯出 `uploadImageFile` 經由它。 */
export const mediaUploadRepo = createMediaUploadRepo();

export const uploadImageFile: MediaUploadRepo["uploadImageFile"] = (
	file,
	onProgress,
	options,
) => mediaUploadRepo.uploadImageFile(file, onProgress, options);

function appUploadClient(): PublicUploadClient {
	return createClient(MediaAppService, authedTransport());
}

interface BeginWire {
	stagedToken: string;
	uploadUrl: string;
	uploadHeaders: Record<string, string>;
}

interface GetWire {
	state: number;
	rejectReason: number | null;
}

function delegatedUploadClient(
	fetcher: RepoFetcher,
	root: string,
): DelegatedPublicUploadClient {
	const token = (stagedToken: string) => encodeURIComponent(stagedToken);
	const client = {
		beginUpload: async (req: {
			size: bigint;
			contentType: string;
			md5: string;
		}) => {
			const env = await fetcher<BeginWire>(root, {
				method: "POST",
				json: {
					size: String(req.size),
					contentType: req.contentType,
					md5: req.md5,
				},
			});
			if (!env.data?.stagedToken || !env.data.uploadUrl) {
				throw new Error("建立上傳工作階段失敗");
			}
			return env.data;
		},
		completeUpload: async (req: { stagedToken: string }) => {
			await fetcher(`${root}/${token(req.stagedToken)}/complete`, {
				method: "POST",
			});
			return {};
		},
		getUpload: async (req: { stagedToken: string }) => {
			const env = await fetcher<GetWire>(`${root}/${token(req.stagedToken)}`);
			if (!env.data) throw new Error("上傳狀態回應缺少內容");
			return {
				state: env.data.state,
				rejectReason: env.data.rejectReason ?? undefined,
			};
		},
	};
	return client as unknown as DelegatedPublicUploadClient;
}

const xhrPut: PutTransport = (req) =>
	new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open("PUT", req.url);
		for (const [k, v] of Object.entries(req.headers))
			xhr.setRequestHeader(k, v);
		xhr.upload.onprogress = (e) => {
			if (!req.onProgress || !e.lengthComputable) return;
			req.onProgress(e.loaded / e.total);
		};
		xhr.onload = () =>
			resolve({
				status: xhr.status,
				headers: { get: (name) => xhr.getResponseHeader(name) },
			});
		xhr.onerror = () => reject(new Error("上傳失敗（網路錯誤）"));
		xhr.onabort = () => reject(new Error("上傳已取消"));
		req.signal?.addEventListener("abort", () => xhr.abort());
		xhr.send(req.body);
	});

function rejectText(reason: number): string {
	const locale = activeLocale();
	const ut = createUiText(
		createTranslator({
			locale,
			messages: messagesFor(locale),
			namespace: "uitext",
		}) as unknown as Translate,
	);
	return ut(mediaRejectReason(reason) ?? commonUnknown());
}

async function uploadImageFileWith(
	upload: PublicUploader,
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
	const outcome = await upload(uploadFile, onProgress);
	if (outcome.state === "rejected") {
		throw new Error(rejectText(outcome.reason));
	}
	return {
		previewUrl: URL.createObjectURL(uploadFile),
		stagedToken: outcome.stagedToken,
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

function formatBytes(n: number): string {
	if (n < 1024) return `${n} B`;
	if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
	return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
