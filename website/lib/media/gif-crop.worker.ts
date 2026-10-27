import { applyPalette, GIFEncoder, quantize } from "gifenc";
import {
	decompressFrame,
	type ParsedFrame,
	type ParsedGif,
	parseGIF,
} from "gifuct-js";
import {
	calculateContainedOutputSize,
	calculateCropContainRect,
} from "@/lib/media/crop";
import {
	GifPreflightError,
	preflightAnimatedGif,
} from "@/lib/media/gif-crop-preflight";
import type {
	GifCropErrorCode,
	GifCropWorkerRequest,
	GifCropWorkerResponse,
} from "@/lib/media/gif-crop-protocol";
import {
	GIF_CROP_MAX_BYTES,
	GIF_CROP_MAX_DECODED_PIXELS,
	GIF_CROP_MAX_DIMENSION,
	GIF_CROP_MAX_FRAMES,
	GIF_CROP_MAX_LOGICAL_PIXELS,
	GIF_CROP_MAX_OUTPUT_PIXELS,
	GIF_CROP_MAX_SOURCE_FRAMES,
} from "@/lib/media/gif-crop-protocol";
import { planGifEdits } from "@/lib/media/gif-edit";
import { GIF_MIN_PLAYBACK_DELAY_MS } from "@/lib/media/gif-timing";

type GifRecord = ParsedGif["frames"][number];
type GifImageRecord = Extract<GifRecord, { image: unknown }>;

interface CropWorkerScope {
	onmessage: ((event: MessageEvent<GifCropWorkerRequest>) => void) | null;
	postMessage(message: GifCropWorkerResponse, transfer?: Transferable[]): void;
}

interface PreviousFrameState {
	disposalType: number;
	dims: { top: number; left: number; width: number; height: number };
	restorePatch: ImageData | null;
}

class WorkerCropError extends Error {
	constructor(
		readonly code: GifCropErrorCode,
		message: string,
	) {
		super(message);
	}
}

const workerScope = self as unknown as CropWorkerScope;

function isImageRecord(record: GifRecord): record is GifImageRecord {
	return "image" in record;
}

function assertCanvasContext(
	canvas: OffscreenCanvas,
): OffscreenCanvasRenderingContext2D {
	const context = canvas.getContext("2d", { willReadFrequently: true });
	if (!context) {
		throw new WorkerCropError(
			"unsupported-browser",
			"目前的瀏覽器無法處理 GIF 動畫，請更新瀏覽器後再試。",
		);
	}
	return context;
}

function validateRequest(
	request: GifCropWorkerRequest,
	width: number,
	height: number,
	frames: GifImageRecord[],
	decodeThroughIndex: number,
): void {
	if (
		!Number.isFinite(width) ||
		!Number.isFinite(height) ||
		width <= 0 ||
		height <= 0
	) {
		throw new WorkerCropError("invalid-file", "GIF 圖片尺寸無效，請更換檔案。");
	}
	const logicalPixels = width * height;
	if (
		width > GIF_CROP_MAX_DIMENSION ||
		height > GIF_CROP_MAX_DIMENSION ||
		logicalPixels > GIF_CROP_MAX_LOGICAL_PIXELS ||
		frames.length > GIF_CROP_MAX_SOURCE_FRAMES
	) {
		throw new WorkerCropError(
			"resource-limit",
			"這個 GIF 的尺寸或動畫幀數太大，請縮小或縮短動畫後再試。",
		);
	}
	if (frames.length < 1) {
		throw new WorkerCropError("invalid-file", "GIF 沒有可讀取的動畫畫面。");
	}
	let decodedPixels = 0;
	for (const [frameIndex, record] of frames.entries()) {
		const descriptor = record.image.descriptor;
		const values = [
			descriptor.left,
			descriptor.top,
			descriptor.width,
			descriptor.height,
		];
		if (
			values.some((value) => !Number.isInteger(value)) ||
			descriptor.left < 0 ||
			descriptor.top < 0 ||
			descriptor.width <= 0 ||
			descriptor.height <= 0 ||
			descriptor.width > GIF_CROP_MAX_DIMENSION ||
			descriptor.height > GIF_CROP_MAX_DIMENSION ||
			descriptor.left + descriptor.width > width ||
			descriptor.top + descriptor.height > height
		) {
			throw new WorkerCropError(
				"invalid-file",
				"GIF 動畫畫面尺寸無效，請更換檔案。",
			);
		}
		const patchPixels = descriptor.width * descriptor.height;
		if (
			patchPixels > GIF_CROP_MAX_LOGICAL_PIXELS ||
			(frameIndex <= decodeThroughIndex &&
				decodedPixels > GIF_CROP_MAX_DECODED_PIXELS - patchPixels)
		) {
			throw new WorkerCropError(
				"resource-limit",
				"這個 GIF 的尺寸或動畫幀數太大，請縮小或縮短動畫後再試。",
			);
		}
		if (frameIndex <= decodeThroughIndex) decodedPixels += patchPixels;
	}
	const { x, y, width: cropWidth, height: cropHeight } = request.area;
	if (
		x < 0 ||
		y < 0 ||
		cropWidth <= 0 ||
		cropHeight <= 0 ||
		x + cropWidth > width + 1 ||
		y + cropHeight > height + 1
	) {
		throw new WorkerCropError("invalid-crop", "圖片取景範圍無效，請重新調整。");
	}
}

// gifenc's installed pnnquant2.js nearest-neighbour work grows with occupied
// histogram bins squared. Smooth resizing can turn a 256-colour GIF into tens
// of thousands of colours, so keep both quantization paths within 4,096 bins.
const MAX_QUANTIZATION_BINS = 4_096;

export function makeIndexedFrame(rgba: Uint8ClampedArray): {
	indexed: Uint8Array;
	palette: number[][];
} {
	let opaquePixels = 0;
	for (let offset = 3; offset < rgba.length; offset += 4) {
		if (rgba[offset] > 127) opaquePixels += 1;
	}

	const opaqueRgba = new Uint8Array(Math.max(1, opaquePixels) * 4);
	const seenColors =
		opaquePixels > MAX_QUANTIZATION_BINS ? new Uint8Array(65_536) : null;
	let occupiedBins = 0;
	let quantizationFormat: "rgb565" | "rgb444" = "rgb565";
	let opaqueOffset = 0;
	for (let offset = 0; offset < rgba.length; offset += 4) {
		if (rgba[offset + 3] <= 127) continue;
		if (seenColors && quantizationFormat === "rgb565") {
			// Match gifenc/src/rgb-packing.js exactly; invisible RGB never creates bins.
			const bin =
				((rgba[offset] << 8) & 0xf800) |
				((rgba[offset + 1] << 2) & 0x03e0) |
				(rgba[offset + 2] >> 3);
			if (seenColors[bin] === 0) {
				seenColors[bin] = 1;
				occupiedBins += 1;
				if (occupiedBins > MAX_QUANTIZATION_BINS) quantizationFormat = "rgb444";
			}
		}
		opaqueRgba[opaqueOffset] = rgba[offset];
		opaqueRgba[opaqueOffset + 1] = rgba[offset + 1];
		opaqueRgba[opaqueOffset + 2] = rgba[offset + 2];
		opaqueRgba[opaqueOffset + 3] = 255;
		opaqueOffset += 4;
	}
	if (opaquePixels === 0) opaqueRgba[3] = 255;

	const opaquePalette = quantize(opaqueRgba, 255, {
		format: quantizationFormat,
	});
	const palette: number[][] = [[0, 0, 0], ...opaquePalette];
	// quantize returns ordinary RGB triples for either histogram format. Keep the
	// existing finer RGB565 nearest-colour mapping rather than coarsening it too.
	const opaqueIndexes = applyPalette(rgba, opaquePalette, "rgb565");
	const indexed = new Uint8Array(opaqueIndexes.length);
	for (let index = 0; index < opaqueIndexes.length; index += 1) {
		indexed[index] = rgba[index * 4 + 3] <= 127 ? 0 : opaqueIndexes[index] + 1;
	}
	return { indexed, palette };
}

function applyPreviousDisposal(
	context: OffscreenCanvasRenderingContext2D,
	previous: PreviousFrameState | null,
): void {
	if (!previous) return;
	if (previous.disposalType === 2) {
		context.clearRect(
			previous.dims.left,
			previous.dims.top,
			previous.dims.width,
			previous.dims.height,
		);
	} else if (
		(previous.disposalType === 3 || previous.disposalType === 4) &&
		previous.restorePatch
	) {
		context.putImageData(
			previous.restorePatch,
			previous.dims.left,
			previous.dims.top,
		);
	}
}

function processGif(request: GifCropWorkerRequest): ArrayBuffer {
	if (request.buffer.byteLength > GIF_CROP_MAX_BYTES) {
		throw new WorkerCropError(
			"resource-limit",
			"GIF 超過 10 MB，請先縮小檔案後再試。",
		);
	}
	let preflight: ReturnType<typeof preflightAnimatedGif>;
	try {
		preflight = preflightAnimatedGif(request.buffer);
	} catch (error) {
		if (error instanceof GifPreflightError) {
			throw new WorkerCropError(error.code, error.message);
		}
		throw error;
	}
	let editPlan: ReturnType<typeof planGifEdits>;
	try {
		editPlan = planGifEdits(preflight.frameDelaysMs, request);
	} catch (error) {
		throw new WorkerCropError(
			"invalid-crop",
			error instanceof Error ? error.message : "GIF 編輯設定無效，請重新調整。",
		);
	}
	// A retained frame depends on every preceding delta/disposal, including the
	// trimmed-away lead-in. Reject excessive selected work before gifuct allocates
	// its parsed tree or the first decompressed patch.
	const selectedDecodedPixels =
		preflight.decodedPixelPrefixes[editPlan.decodeThroughIndex];
	if (
		!Number.isFinite(selectedDecodedPixels) ||
		selectedDecodedPixels > GIF_CROP_MAX_DECODED_PIXELS
	) {
		throw new WorkerCropError(
			"resource-limit",
			"這段 GIF 的解碼處理量過大，請提早結束時間，或改用尺寸較小的原始 GIF。",
		);
	}
	let parsed: ParsedGif;
	try {
		parsed = parseGIF(request.buffer);
	} catch {
		throw new WorkerCropError("invalid-file", "GIF 檔案已損壞或無法讀取。");
	}
	const width = parsed.lsd.width;
	const height = parsed.lsd.height;
	const frames = parsed.frames.filter(isImageRecord);
	if (
		width !== preflight.width ||
		height !== preflight.height ||
		frames.length !== preflight.frameCount
	) {
		throw new WorkerCropError(
			"invalid-file",
			"GIF 檔案結構不一致，請更換檔案。",
		);
	}
	validateRequest(request, width, height, frames, editPlan.decodeThroughIndex);
	const loopCount = preflight.loopCount;
	const frameDelaysMs = frames.map((record) =>
		"gce" in record ? (record.gce.delay ?? 0) * 10 : 0,
	);
	if (
		frameDelaysMs.some(
			(delay, index) => delay !== preflight.frameDelaysMs[index],
		)
	) {
		throw new WorkerCropError(
			"invalid-file",
			"GIF 動畫時間結構不一致，請更換檔案。",
		);
	}
	const maxOutput = Math.max(4, Math.floor(request.maxOutput));
	const output = calculateContainedOutputSize(
		request.area.width,
		request.area.height,
		request.containAspect,
		maxOutput,
	);
	const destination = calculateCropContainRect(
		request.area.width,
		request.area.height,
		output.width,
		output.height,
	);
	// A GIF delay is an unsigned 16-bit centisecond value. Sampling may combine
	// long delays, so split such holds instead of allowing the encoder to wrap.
	const maxFrameDelayMs = 655_350;
	const encodedFrameCount = editPlan.frames.reduce(
		(sum, frame) =>
			sum + Math.max(1, Math.ceil(frame.delayMs / maxFrameDelayMs)),
		0,
	);
	if (encodedFrameCount > GIF_CROP_MAX_FRAMES) {
		throw new WorkerCropError(
			"resource-limit",
			"調整後的 GIF 畫面超過 180 幀，請降低流暢度或縮短動畫後再試。",
		);
	}
	if (
		output.width * output.height * encodedFrameCount >
		GIF_CROP_MAX_OUTPUT_PIXELS
	) {
		throw new WorkerCropError(
			"output-resource-limit",
			"GIF 處理量較大，請降低輸出尺寸、流暢度，或縮短動畫後再試。",
		);
	}
	const logicalCanvas = new OffscreenCanvas(width, height);
	const logicalContext = assertCanvasContext(logicalCanvas);
	const patchCanvas = new OffscreenCanvas(1, 1);
	const outputCanvas = new OffscreenCanvas(output.width, output.height);
	const outputContext = assertCanvasContext(outputCanvas);
	outputContext.imageSmoothingEnabled = true;
	outputContext.imageSmoothingQuality = "high";
	const encoder = GIFEncoder();
	let previous: PreviousFrameState | null = null;
	let plannedFrameIndex = 0;
	let emittedFrames = 0;

	for (let frameIndex = 0; frameIndex < frames.length; frameIndex += 1) {
		if (frameIndex > editPlan.decodeThroughIndex) break;
		const record = frames[frameIndex];
		applyPreviousDisposal(logicalContext, previous);
		let frame: ParsedFrame | undefined;
		try {
			frame = decompressFrame(record, parsed.gct, true);
		} catch {
			throw new WorkerCropError("invalid-file", "GIF 動畫畫面無法解碼。");
		}
		if (!frame?.patch) {
			throw new WorkerCropError("invalid-file", "GIF 動畫畫面內容不完整。");
		}
		const dims = frame.dims;
		const restorePatch =
			frame.disposalType === 3 || frame.disposalType === 4
				? logicalContext.getImageData(
						dims.left,
						dims.top,
						dims.width,
						dims.height,
					)
				: null;

		patchCanvas.width = dims.width;
		patchCanvas.height = dims.height;
		const patchContext = assertCanvasContext(patchCanvas);
		const patchPixels: Uint8ClampedArray<ArrayBuffer> = new Uint8ClampedArray(
			frame.patch.length,
		);
		patchPixels.set(frame.patch);
		patchContext.putImageData(
			new ImageData(patchPixels, dims.width, dims.height),
			0,
			0,
		);
		logicalContext.drawImage(patchCanvas, dims.left, dims.top);
		// Always update source disposal, even for trimmed or sampled-out patches.
		previous = {
			disposalType: frame.disposalType ?? 0,
			dims,
			restorePatch,
		};
		workerScope.postMessage({
			type: "progress",
			progress: ((frameIndex + 1) / (editPlan.decodeThroughIndex + 1)) * 0.98,
		});
		const plannedFrame = editPlan.frames[plannedFrameIndex];
		if (plannedFrame?.sourceFrameIndex !== frameIndex) continue;
		plannedFrameIndex += 1;

		outputContext.clearRect(0, 0, output.width, output.height);
		outputContext.drawImage(
			logicalCanvas,
			request.area.x,
			request.area.y,
			request.area.width,
			request.area.height,
			destination.x,
			destination.y,
			destination.width,
			destination.height,
		);
		const rgba = outputContext.getImageData(
			0,
			0,
			output.width,
			output.height,
		).data;
		const { indexed, palette } = makeIndexedFrame(rgba);
		let remainingDelayMs = plannedFrame.delayMs;
		do {
			let delayMs = Math.min(remainingDelayMs, maxFrameDelayMs);
			const remainderMs = remainingDelayMs - delayMs;
			if (remainderMs > 0 && remainderMs < GIF_MIN_PLAYBACK_DELAY_MS) {
				delayMs -= GIF_MIN_PLAYBACK_DELAY_MS - remainderMs;
			}
			encoder.writeFrame(indexed, output.width, output.height, {
				palette,
				transparent: true,
				transparentIndex: 0,
				delay: delayMs,
				repeat: emittedFrames === 0 ? loopCount : undefined,
				dispose: 2,
			});
			emittedFrames += 1;
			remainingDelayMs -= delayMs;
			if (encoder.bytesView().byteLength > GIF_CROP_MAX_BYTES) {
				throw new WorkerCropError(
					"output-too-large",
					"調整後的 GIF 超過 10 MB，請降低輸出尺寸或流暢度後再試。",
				);
			}
		} while (remainingDelayMs > 0);
	}

	encoder.finish();
	const bytes = encoder.bytes();
	if (bytes.byteLength > GIF_CROP_MAX_BYTES) {
		throw new WorkerCropError(
			"output-too-large",
			"調整後的 GIF 超過 10 MB，請縮短動畫或改用較小的 GIF。",
		);
	}
	const outputBuffer = new Uint8Array(bytes.byteLength);
	outputBuffer.set(bytes);
	return outputBuffer.buffer;
}

workerScope.onmessage = (event) => {
	if (event.data.type !== "crop") return;
	try {
		const buffer = processGif(event.data);
		workerScope.postMessage({ type: "done", buffer }, [buffer]);
	} catch (error) {
		const code =
			error instanceof WorkerCropError ? error.code : "processing-failed";
		const message =
			error instanceof WorkerCropError
				? error.message
				: "GIF 動畫處理失敗，原圖尚未上傳。請重試或更換圖片。";
		workerScope.postMessage({ type: "error", code, message });
	}
};
