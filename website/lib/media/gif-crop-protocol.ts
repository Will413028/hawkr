import type { CropAreaPixels } from "@/lib/media/crop";

/** The Media Service upload contract caps every image at 10 MiB. */
export const GIF_CROP_MAX_BYTES = 10 * 1024 * 1024;
/** Keep authored post GIFs at the canonical card size to bound mobile work. */
export const GIF_CROP_MAX_EDGE = 720;
/** Reject decoder bombs before allocating a logical canvas. */
export const GIF_CROP_MAX_LOGICAL_PIXELS = 2_500_000;
/** Browser canvas implementations also impose a per-axis limit. */
export const GIF_CROP_MAX_DIMENSION = 4096;
/** Maximum emitted frames; longer sources can be trimmed or sampled first. */
export const GIF_CROP_MAX_FRAMES = 180;
/** Bounded metadata inspection, independent of the selected output frame count. */
export const GIF_CROP_MAX_SOURCE_FRAMES = 1_024;
/** Cumulative input patch work; frames are decoded/composited one at a time. */
export const GIF_CROP_MAX_DECODED_PIXELS = 240_000_000;
/** Separate bound for output canvas reads, palette quantization and encoding. */
export const GIF_CROP_MAX_OUTPUT_PIXELS = 80_000_000;
/** Limit parsed records before gifuct allocates one object per extension. */
export const GIF_CROP_MAX_RECORDS = 2_048;
/** A 10 MiB valid image can need ~41k 255-byte data sub-blocks. */
export const GIF_CROP_MAX_SUB_BLOCKS = 50_000;

export type GifCropErrorCode =
	| "invalid-file"
	| "invalid-crop"
	| "resource-limit"
	| "unsupported-browser"
	| "output-too-large"
	| "output-resource-limit"
	| "processing-failed";

export interface GifCropWorkerRequest {
	type: "crop";
	buffer: ArrayBuffer;
	area: CropAreaPixels;
	containAspect: number;
	maxOutput: number;
	maxFps?: number;
	startTimeMs?: number;
	endTimeMs?: number;
}

export type GifCropWorkerResponse =
	| { type: "progress"; progress: number }
	| { type: "done"; buffer: ArrayBuffer }
	| { type: "error"; code: GifCropErrorCode; message: string };
