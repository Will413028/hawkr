import {
	GIF_CROP_MAX_BYTES,
	GIF_CROP_MAX_DIMENSION,
	GIF_CROP_MAX_LOGICAL_PIXELS,
	GIF_CROP_MAX_RECORDS,
	GIF_CROP_MAX_SOURCE_FRAMES,
	GIF_CROP_MAX_SUB_BLOCKS,
} from "@/lib/media/gif-crop-protocol";

export class GifPreflightError extends Error {
	constructor(
		readonly code: "invalid-file" | "resource-limit",
		message: string,
	) {
		super(message);
		this.name = "GifPreflightError";
	}
}

export interface GifPreflightResult {
	width: number;
	height: number;
	frameCount: number;
	decodedPixels: number;
	/** Inclusive cumulative patch pixels, used to budget the selected decode prefix. */
	decodedPixelPrefixes: number[];
	/** Raw GIF delays, in milliseconds; zero-delay frames stay zero. */
	frameDelaysMs: number[];
	durationMs: number;
	/** -1 omits the loop extension; 0 means infinite looping. */
	loopCount: number;
}

const GIF_HEADER_BYTES = 6;
const LOGICAL_SCREEN_DESCRIPTOR_BYTES = 7;

/**
 * Structural scan with only a bounded delay list before gifuct builds its tree.
 * It rejects descriptor/extension bombs and reads loop metadata only from a
 * real application extension, never from matching bytes inside image payloads.
 */
export function preflightAnimatedGif(buffer: ArrayBuffer): GifPreflightResult {
	if (buffer.byteLength > GIF_CROP_MAX_BYTES) {
		throw new GifPreflightError(
			"resource-limit",
			"GIF 超過 10 MB，請先縮小檔案後再試。",
		);
	}
	const bytes = new Uint8Array(buffer);
	let cursor = 0;
	let recordCount = 0;
	let subBlockCount = 0;
	let frameCount = 0;
	let decodedPixels = 0;
	let loopCount = -1;
	let pendingDelayMs = 0;
	let durationMs = 0;
	const frameDelaysMs: number[] = [];
	const decodedPixelPrefixes: number[] = [];

	const invalid = (message = "GIF 檔案已損壞或無法讀取。"): never => {
		throw new GifPreflightError("invalid-file", message);
	};
	const resourceLimit = (): never => {
		throw new GifPreflightError(
			"resource-limit",
			"這個 GIF 的尺寸或動畫幀數太大，請縮小或縮短動畫後再試。",
		);
	};
	const ensure = (length: number): void => {
		if (length < 0 || cursor + length > bytes.length) invalid();
	};
	const readByte = (): number => {
		ensure(1);
		return bytes[cursor++];
	};
	const readUint16 = (): number => {
		ensure(2);
		const value = bytes[cursor] | (bytes[cursor + 1] << 8);
		cursor += 2;
		return value;
	};
	const readAscii = (length: number): string => {
		ensure(length);
		let value = "";
		for (let index = 0; index < length; index += 1) {
			value += String.fromCharCode(bytes[cursor + index]);
		}
		cursor += length;
		return value;
	};
	const skip = (length: number): void => {
		ensure(length);
		cursor += length;
	};
	const countRecord = (): void => {
		recordCount += 1;
		if (recordCount > GIF_CROP_MAX_RECORDS) resourceLimit();
	};
	const readSubBlocks = (
		onFirst?: (start: number, length: number) => void,
	): void => {
		let first = true;
		while (true) {
			const length = readByte();
			if (length === 0) return;
			subBlockCount += 1;
			if (subBlockCount > GIF_CROP_MAX_SUB_BLOCKS) resourceLimit();
			ensure(length);
			if (first) onFirst?.(cursor, length);
			first = false;
			cursor += length;
		}
	};
	const skipColorTable = (packed: number): boolean => {
		const exists = (packed & 0x80) !== 0;
		if (!exists) return false;
		const entries = 1 << ((packed & 0x07) + 1);
		skip(entries * 3);
		return true;
	};

	if (bytes.length < GIF_HEADER_BYTES + LOGICAL_SCREEN_DESCRIPTOR_BYTES) {
		invalid();
	}
	const signature = readAscii(GIF_HEADER_BYTES);
	if (signature !== "GIF87a" && signature !== "GIF89a") invalid();
	const width = readUint16();
	const height = readUint16();
	const logicalPixels = width * height;
	if (width <= 0 || height <= 0) invalid("GIF 圖片尺寸無效，請更換檔案。");
	if (
		width > GIF_CROP_MAX_DIMENSION ||
		height > GIF_CROP_MAX_DIMENSION ||
		logicalPixels > GIF_CROP_MAX_LOGICAL_PIXELS
	) {
		resourceLimit();
	}
	const logicalPacked = readByte();
	skip(2); // background colour index + pixel aspect ratio
	const hasGlobalColorTable = skipColorTable(logicalPacked);

	let sawTrailer = false;
	while (cursor < bytes.length) {
		const marker = readByte();
		if (marker === 0x3b) {
			sawTrailer = true;
			break;
		}
		countRecord();

		if (marker === 0x21) {
			const label = readByte();
			const firstBlockLength = readByte();
			subBlockCount += 1;
			if (subBlockCount > GIF_CROP_MAX_SUB_BLOCKS) resourceLimit();
			ensure(firstBlockLength);

			if (label === 0xf9) {
				if (firstBlockLength !== 4) invalid();
				skip(1); // packed disposal/transparency flags
				pendingDelayMs = readUint16() * 10;
				skip(1); // transparent colour index
				if (readByte() !== 0) invalid();
				continue;
			}

			let applicationId: string | null = null;
			if (label === 0xff) {
				if (firstBlockLength !== 11) invalid();
				applicationId = readAscii(firstBlockLength);
			} else {
				skip(firstBlockLength);
				// A plain-text rendering block consumes the preceding GCE too.
				if (label === 0x01) pendingDelayMs = 0;
				// Comment/unknown extensions are themselves a sub-block chain.
				// A zero-sized first block already terminated that chain.
				if (firstBlockLength === 0) continue;
			}

			const isLoopExtension =
				applicationId === "NETSCAPE2.0" || applicationId === "ANIMEXTS1.0";
			let sawLoopPayload = false;
			readSubBlocks((start, length) => {
				if (!isLoopExtension) return;
				if (length !== 3 || bytes[start] !== 1) invalid();
				loopCount = bytes[start + 1] | (bytes[start + 2] << 8);
				sawLoopPayload = true;
			});
			if (isLoopExtension && !sawLoopPayload) invalid();
			continue;
		}

		if (marker === 0x2c) {
			const left = readUint16();
			const top = readUint16();
			const frameWidth = readUint16();
			const frameHeight = readUint16();
			const imagePacked = readByte();
			const hasLocalColorTable = skipColorTable(imagePacked);
			if (!hasGlobalColorTable && !hasLocalColorTable) invalid();
			if (
				frameWidth <= 0 ||
				frameHeight <= 0 ||
				frameWidth > GIF_CROP_MAX_DIMENSION ||
				frameHeight > GIF_CROP_MAX_DIMENSION ||
				left + frameWidth > width ||
				top + frameHeight > height
			) {
				invalid("GIF 動畫畫面尺寸無效，請更換檔案。");
			}
			const framePixels = frameWidth * frameHeight;
			if (framePixels > GIF_CROP_MAX_LOGICAL_PIXELS) {
				resourceLimit();
			}
			decodedPixels += framePixels;
			frameCount += 1;
			if (frameCount > GIF_CROP_MAX_SOURCE_FRAMES) resourceLimit();
			decodedPixelPrefixes.push(decodedPixels);
			frameDelaysMs.push(pendingDelayMs);
			durationMs += pendingDelayMs;
			pendingDelayMs = 0;

			const minimumCodeSize = readByte();
			if (minimumCodeSize < 2 || minimumCodeSize > 8) invalid();
			let hasImageData = false;
			readSubBlocks((_start, length) => {
				hasImageData = length > 0;
			});
			if (!hasImageData) invalid();
			continue;
		}

		invalid();
	}

	if (!sawTrailer || frameCount < 1) invalid();
	return {
		width,
		height,
		frameCount,
		decodedPixels,
		decodedPixelPrefixes,
		loopCount,
		frameDelaysMs,
		durationMs,
	};
}
