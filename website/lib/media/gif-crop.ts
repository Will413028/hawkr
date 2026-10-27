import {
	type CropAreaPixels,
	calculateContainedOutputSize,
} from "@/lib/media/crop";
import type {
	GifCropErrorCode,
	GifCropWorkerRequest,
	GifCropWorkerResponse,
} from "@/lib/media/gif-crop-protocol";
import type { GifEditSettings } from "@/lib/media/gif-edit";

export {
	GIF_CROP_MAX_DECODED_PIXELS,
	GIF_CROP_MAX_DIMENSION,
	GIF_CROP_MAX_FRAMES,
	GIF_CROP_MAX_LOGICAL_PIXELS,
} from "@/lib/media/gif-crop-protocol";

import { isGifUploadFile, normalizeGifUploadFile } from "@/lib/media/gif";
import {
	GIF_CROP_MAX_BYTES as PROTOCOL_MAX_BYTES,
	GIF_CROP_MAX_EDGE as PROTOCOL_MAX_EDGE,
} from "@/lib/media/gif-crop-protocol";

// Keep these limits visible to upload UI/tests without making the worker import
// this browser-facing module (which would create a worker bundle cycle).
export const GIF_CROP_MAX_BYTES = PROTOCOL_MAX_BYTES;
export const GIF_CROP_MAX_EDGE = PROTOCOL_MAX_EDGE;

export class GifCropError extends Error {
	readonly code: GifCropErrorCode;

	constructor(code: GifCropErrorCode, message: string) {
		super(message);
		this.name = "GifCropError";
		this.code = code;
	}
}

export interface AnimatedGifCropOptions extends GifEditSettings {
	containAspect: number;
	maxOutput?: number;
	/** Retry smaller output sizes to fit upload limits; crop, time and FPS stay fixed. */
	autoFit?: boolean;
	signal?: AbortSignal;
	onProgress?: (progress: number) => void;
}

function abortError(): DOMException {
	return new DOMException("GIF 處理已取消", "AbortError");
}

function assertCropArea(area: CropAreaPixels): void {
	for (const value of [area.x, area.y, area.width, area.height]) {
		if (!Number.isFinite(value)) {
			throw new GifCropError("invalid-crop", "圖片取景範圍無效，請重試。");
		}
	}
	if (area.x < 0 || area.y < 0 || area.width <= 0 || area.height <= 0) {
		throw new GifCropError("invalid-crop", "圖片取景範圍無效，請重試。");
	}
}

/**
 * Rebuild an animated GIF using the seller-authored crop and transparent 4:3
 * padding. The worker never uploads anything; its one returned File continues
 * through the existing Media Service staged-upload lifecycle.
 */
export async function cropAnimatedGifFile(
	file: File,
	area: CropAreaPixels,
	options: AnimatedGifCropOptions,
): Promise<File> {
	if (!isGifUploadFile(file)) {
		throw new GifCropError("invalid-file", "選取的檔案不是 GIF 動畫。");
	}
	if (file.size > GIF_CROP_MAX_BYTES) {
		throw new GifCropError(
			"resource-limit",
			"GIF 超過 10 MB，請先縮小檔案後再試。",
		);
	}
	assertCropArea(area);
	if (!Number.isFinite(options.containAspect) || options.containAspect <= 0) {
		throw new GifCropError("invalid-crop", "圖片輸出比例無效，請重試。");
	}
	if (options.signal?.aborted) throw abortError();
	const requestedMaxOutput = options.maxOutput ?? GIF_CROP_MAX_EDGE;
	if (!Number.isFinite(requestedMaxOutput) || requestedMaxOutput < 4) {
		throw new GifCropError("invalid-crop", "圖片輸出尺寸無效，請重試。");
	}
	const maxOutput = Math.min(requestedMaxOutput, GIF_CROP_MAX_EDGE);
	const candidateSizes = options.autoFit
		? [maxOutput, ...[360, 240].filter((size) => size < maxOutput)]
		: [maxOutput];
	const seenDimensions = new Set<string>();
	const outputSizes = candidateSizes.filter((size) => {
		const { width, height } = calculateContainedOutputSize(
			area.width,
			area.height,
			options.containAspect,
			size,
		);
		const key = `${width}x${height}`;
		if (seenDimensions.has(key)) return false;
		seenDimensions.add(key);
		return true;
	});

	const normalized = normalizeGifUploadFile(file);
	let buffer = await normalized.arrayBuffer();
	if (options.signal?.aborted) throw abortError();
	if (buffer.byteLength < 6) {
		throw new GifCropError("invalid-file", "GIF 檔案已損壞或無法讀取。");
	}
	const signature = String.fromCharCode(...new Uint8Array(buffer, 0, 6));
	if (signature !== "GIF87a" && signature !== "GIF89a") {
		throw new GifCropError("invalid-file", "GIF 檔案已損壞或無法讀取。");
	}
	if (typeof Worker === "undefined" || typeof OffscreenCanvas === "undefined") {
		throw new GifCropError(
			"unsupported-browser",
			"目前的瀏覽器無法處理 GIF 動畫，請更新瀏覽器後再試。",
		);
	}

	for (let attempt = 0; attempt < outputSizes.length; attempt += 1) {
		if (options.signal?.aborted) throw abortError();
		if (attempt > 0) {
			// Each attempt transfers ownership and terminates its worker before retry.
			buffer = await normalized.arrayBuffer();
			if (options.signal?.aborted) throw abortError();
		}
		try {
			const result = await runWorkerAttempt(normalized, buffer, area, {
				...options,
				maxOutput: outputSizes[attempt],
				onProgress: (progress) =>
					options.onProgress?.((attempt + progress) / outputSizes.length),
			});
			if (options.signal?.aborted) throw abortError();
			try {
				options.onProgress?.(1);
			} catch {
				// Presentation callbacks must not discard a completed output.
			}
			return result;
		} catch (error) {
			if (options.signal?.aborted) throw abortError();
			if (
				!(error instanceof GifCropError) ||
				(error.code !== "output-too-large" &&
					error.code !== "output-resource-limit") ||
				attempt === outputSizes.length - 1
			) {
				throw error;
			}
		}
	}
	throw new GifCropError("processing-failed", "GIF 動畫處理失敗，請重試。");
}

function runWorkerAttempt(
	normalized: File,
	buffer: ArrayBuffer,
	area: CropAreaPixels,
	options: AnimatedGifCropOptions & { maxOutput: number },
): Promise<File> {
	return new Promise<File>((resolve, reject) => {
		const worker = new Worker(
			new URL("./gif-crop.worker.ts", import.meta.url),
			{ type: "module" },
		);
		let settled = false;
		let lastProgress = 0;

		const cleanup = () => {
			worker.onmessage = null;
			worker.onerror = null;
			worker.terminate();
			options.signal?.removeEventListener("abort", handleAbort);
		};
		const reportProgress = (progress: number) => {
			try {
				options.onProgress?.(progress);
			} catch {
				// A presentation callback must never strand the processing promise.
			}
		};
		const settleReject = (error: unknown) => {
			if (settled) return;
			settled = true;
			cleanup();
			reject(error);
		};
		const handleAbort = () => settleReject(abortError());

		worker.onmessage = (event: MessageEvent<GifCropWorkerResponse>) => {
			if (settled) return;
			const message = event.data;
			if (message.type === "progress") {
				const progress = Math.max(lastProgress, Math.min(1, message.progress));
				lastProgress = progress;
				reportProgress(progress);
				return;
			}
			if (message.type === "error") {
				settleReject(new GifCropError(message.code, message.message));
				return;
			}
			if (message.buffer.byteLength > GIF_CROP_MAX_BYTES) {
				settleReject(
					new GifCropError(
						"output-too-large",
						"調整後的 GIF 超過 10 MB，請縮短動畫或改用較小的 GIF。",
					),
				);
				return;
			}

			settled = true;
			cleanup();
			reportProgress(1);
			const base = normalized.name.replace(/\.gif$/i, "") || "post-animation";
			resolve(
				new File([message.buffer], `${base}-crop.gif`, {
					type: "image/gif",
					lastModified: normalized.lastModified,
				}),
			);
		};
		worker.onerror = () => {
			settleReject(
				new GifCropError(
					"processing-failed",
					"GIF 動畫處理失敗，原圖尚未上傳。請重試或更換圖片。",
				),
			);
		};
		options.signal?.addEventListener("abort", handleAbort, { once: true });

		if (options.signal?.aborted) {
			handleAbort();
			return;
		}
		const request: GifCropWorkerRequest = {
			type: "crop",
			buffer,
			area: { ...area },
			containAspect: options.containAspect,
			maxOutput: options.maxOutput,
			maxFps: options.maxFps,
			startTimeMs: options.startTimeMs,
			endTimeMs: options.endTimeMs,
		};
		try {
			worker.postMessage(request, [buffer]);
		} catch {
			settleReject(
				new GifCropError("processing-failed", "GIF 動畫處理失敗，請重試。"),
			);
		}
	});
}
