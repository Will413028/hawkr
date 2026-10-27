import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test, { mock } from "node:test";
import { decompressFrame, parseGIF } from "gifuct-js";
import { preflightAnimatedGif } from "../lib/media/gif-crop-preflight";
import type {
	GifCropWorkerRequest,
	GifCropWorkerResponse,
} from "../lib/media/gif-crop-protocol";
import { inspectGifFile } from "../lib/media/gif-edit";

const codecs = createRequire(import.meta.url)(
	"gifenc",
) as typeof import("gifenc");

/** Pixel-exact canvas subset for tiny unscaled fixtures; codecs remain real. */
class TinyImageData {
	constructor(
		readonly data: Uint8ClampedArray,
		readonly width: number,
		readonly height: number,
	) {}
}

class TinyCanvas {
	private pixelWidth: number;
	private pixelHeight: number;
	data: Uint8ClampedArray;
	constructor(width: number, height: number) {
		this.pixelWidth = width;
		this.pixelHeight = height;
		this.data = new Uint8ClampedArray(width * height * 4);
	}
	get width() {
		return this.pixelWidth;
	}
	set width(value: number) {
		this.pixelWidth = value;
		this.data = new Uint8ClampedArray(this.width * this.height * 4);
	}
	get height() {
		return this.pixelHeight;
	}
	set height(value: number) {
		this.pixelHeight = value;
		this.data = new Uint8ClampedArray(this.width * this.height * 4);
	}
	getContext() {
		const canvas = this;
		return {
			clearRect(left: number, top: number, width: number, height: number) {
				for (let y = top; y < top + height; y += 1) {
					for (let x = left; x < left + width; x += 1) {
						canvas.data.fill(
							0,
							(y * canvas.width + x) * 4,
							(y * canvas.width + x + 1) * 4,
						);
					}
				}
			},
			getImageData(left: number, top: number, width: number, height: number) {
				const data = new Uint8ClampedArray(width * height * 4);
				for (let y = 0; y < height; y += 1) {
					for (let x = 0; x < width; x += 1) {
						const source = ((top + y) * canvas.width + left + x) * 4;
						data.set(
							canvas.data.subarray(source, source + 4),
							(y * width + x) * 4,
						);
					}
				}
				return new TinyImageData(data, width, height);
			},
			putImageData(image: TinyImageData, left: number, top: number) {
				for (let y = 0; y < image.height; y += 1) {
					for (let x = 0; x < image.width; x += 1) {
						const source = (y * image.width + x) * 4;
						canvas.data.set(
							image.data.subarray(source, source + 4),
							((top + y) * canvas.width + left + x) * 4,
						);
					}
				}
			},
			drawImage(source: TinyCanvas, ...args: number[]) {
				const [sx, sy, sw, sh, dx, dy, dw, dh] =
					args.length === 2
						? [
								0,
								0,
								source.width,
								source.height,
								args[0],
								args[1],
								source.width,
								source.height,
							]
						: args;
				for (let y = 0; y < dh; y += 1) {
					for (let x = 0; x < dw; x += 1) {
						const offset =
							((sy + Math.floor((y * sh) / dh)) * source.width +
								sx +
								Math.floor((x * sw) / dw)) *
							4;
						if (source.data[offset + 3] === 0) continue;
						canvas.data.set(
							source.data.subarray(offset, offset + 4),
							((dy + y) * canvas.width + dx + x) * 4,
						);
					}
				}
			},
		};
	}
}

function patchAnimation(disposal: number): ArrayBuffer {
	const frameSpecs = [
		{ width: 4, height: 3, left: 0, color: [255, 0, 0], disposal: 1 },
		{ width: 1, height: 1, left: 0, color: [0, 0, 255], disposal },
		{ width: 1, height: 1, left: 1, color: [0, 255, 0], disposal: 1 },
	];
	let header: number[] = [];
	const blocks = frameSpecs.flatMap((frame, index) => {
		const encoder = codecs.GIFEncoder();
		encoder.writeFrame(
			new Uint8Array(frame.width * frame.height),
			frame.width,
			frame.height,
			{
				palette: [frame.color],
				delay: 40,
				repeat: -1,
				dispose: frame.disposal,
			},
		);
		encoder.finish();
		const bytes = Array.from(encoder.bytes());
		const tableBytes = 3 * (1 << ((bytes[10] & 7) + 1));
		if (index === 0) header = bytes.slice(0, 13 + tableBytes);
		const palette = bytes.slice(13, 13 + tableBytes);
		const body = bytes.slice(13 + tableBytes, -1);
		assert.equal(body[8], 0x2c); // GCE followed by image descriptor
		body[9] = frame.left;
		body[17] = 0x80 | (bytes[10] & 7); // give each patch its own palette
		return [...body.slice(0, 18), ...palette, ...body.slice(18)];
	});
	const loop = [
		0x21,
		0xff,
		11,
		...Array.from("NETSCAPE2.0", (char) => char.charCodeAt(0)),
		3,
		1,
		3,
		0,
		0,
	];
	return new Uint8Array([...header, ...loop, ...blocks, 0x3b]).buffer;
}

function repeatedAnimation(
	frameCount: number,
	delaysMs?: readonly number[],
): ArrayBuffer {
	const encoder = codecs.GIFEncoder();
	for (let index = 0; index < frameCount; index += 1) {
		encoder.writeFrame(new Uint8Array(12), 4, 3, {
			palette: [[255, 0, 0]],
			delay: delaysMs?.[index] ?? 40,
			repeat: -1,
		});
	}
	encoder.finish();
	return new Uint8Array(encoder.bytes()).buffer;
}

/** A small first patch followed by valid, highly compressible large patches. */
function largeDescriptorAnimation(frameCount: number): ArrayBuffer {
	const encoder = codecs.GIFEncoder();
	encoder.writeFrame(new Uint8Array(1600 * 1500), 1600, 1500, {
		palette: [[255, 0, 0]],
		delay: 40,
		repeat: -1,
	});
	encoder.finish();
	const base = encoder.bytes();
	const tableBytes = 3 * (1 << ((base[10] & 7) + 1));
	const header = Array.from(base.slice(0, 13 + tableBytes));
	const frame = Array.from(base.slice(13 + tableBytes, -1));
	const smallFrame = Array.from(
		new Uint8Array(repeatedAnimation(1)).slice(13 + tableBytes, -1),
	);
	return new Uint8Array([
		...header,
		...smallFrame,
		...Array.from({ length: frameCount - 1 }, () => frame).flat(),
		0x3b,
	]).buffer;
}

test("worker composites edits and budgets selected work before decoding", async (t) => {
	const responses: GifCropWorkerResponse[] = [];
	const scope: {
		onmessage: ((event: { data: GifCropWorkerRequest }) => void) | null;
		postMessage(message: GifCropWorkerResponse): void;
	} = {
		onmessage: null,
		postMessage: (message) => responses.push(message),
	};
	const originals = ["self", "OffscreenCanvas", "ImageData"].map(
		(name) =>
			[name, Object.getOwnPropertyDescriptor(globalThis, name)] as const,
	);
	const quantizationFormats: Array<string | undefined> = [];
	const codecMock = mock.module("gifenc", {
		namedExports: {
			GIFEncoder: codecs.GIFEncoder,
			quantize: (...args: Parameters<typeof codecs.quantize>) => {
				quantizationFormats.push(args[2]?.format);
				return codecs.quantize(...args);
			},
			applyPalette: codecs.applyPalette,
		},
	});
	let parseCalls = 0;
	let decodeCalls = 0;
	const decoderMock = mock.module("gifuct-js", {
		namedExports: {
			parseGIF: (buffer: ArrayBuffer) => {
				parseCalls += 1;
				return parseGIF(buffer);
			},
			decompressFrame: (...args: Parameters<typeof decompressFrame>) => {
				decodeCalls += 1;
				return decompressFrame(...args);
			},
		},
	});
	Object.defineProperty(globalThis, "self", {
		configurable: true,
		value: scope,
	});
	Object.defineProperty(globalThis, "OffscreenCanvas", {
		configurable: true,
		value: TinyCanvas,
	});
	Object.defineProperty(globalThis, "ImageData", {
		configurable: true,
		value: TinyImageData,
	});
	try {
		const { makeIndexedFrame } = await import("../lib/media/gif-crop.worker");
		await t.test(
			"high-colour canvas readback uses bounded bins and retains valid colour and transparent padding",
			() => {
				// Represents the high-colour RGBA produced by smooth browser downscaling.
				const width = 128;
				const height = 96;
				const rgba = new Uint8ClampedArray(width * height * 4);
				for (let index = 0; index < 8192; index += 1) {
					const color = (index * 4051) & 0x7fff;
					rgba.set(
						[
							((color >> 10) & 31) * 8,
							((color >> 5) & 31) * 8,
							(color & 31) * 8,
							255,
						],
						(width + index) * 4,
					);
				}
				quantizationFormats.length = 0;
				const { indexed, palette } = makeIndexedFrame(rgba);
				assert.deepEqual(quantizationFormats, ["rgb444"]);
				assert.ok(palette.length <= 256);
				assert.ok(
					palette.every(
						(color) =>
							color.length === 3 &&
							color.every(
								(channel) =>
									Number.isInteger(channel) && channel >= 0 && channel <= 255,
							),
					),
				);
				const encoder = codecs.GIFEncoder();
				encoder.writeFrame(indexed, width, height, {
					palette,
					transparent: true,
					transparentIndex: 0,
					delay: 80,
				});
				encoder.finish();
				const buffer = new Uint8Array(encoder.bytes()).buffer;
				const metadata = preflightAnimatedGif(buffer);
				assert.deepEqual(
					[metadata.width, metadata.height, metadata.durationMs],
					[128, 96, 80],
				);
				const parsed = parseGIF(buffer);
				const record = parsed.frames.find((frame) => "image" in frame);
				assert.ok(record && "image" in record);
				const output = decompressFrame(record, parsed.gct, true);
				assert.ok(output?.patch);
				let colorError = 0;
				for (let index = 0; index < width * height; index += 1) {
					const opaque = rgba[index * 4 + 3] > 127;
					assert.equal(output.patch[index * 4 + 3], opaque ? 255 : 0);
					assert.equal(indexed[index] === 0, !opaque);
					if (opaque) {
						for (let channel = 0; channel < 3; channel += 1) {
							colorError += Math.abs(
								output.patch[index * 4 + channel] - rgba[index * 4 + channel],
							);
						}
					}
				}
				assert.ok(
					colorError / (8192 * 3) < 25,
					"palette must retain usable RGB colour fidelity",
				);
				quantizationFormats.length = 0;
				makeIndexedFrame(rgba.slice(width * 4, (width + 4096) * 4));
				assert.deepEqual(quantizationFormats, ["rgb565"]);
				for (let offset = 3; offset < rgba.length; offset += 4)
					rgba[offset] = 0;
				rgba.set([200, 80, 16, 255], 0);
				quantizationFormats.length = 0;
				const hiddenColors = makeIndexedFrame(rgba);
				assert.deepEqual(
					quantizationFormats,
					["rgb565"],
					"invisible RGB must not inflate quantization complexity",
				);
				assert.deepEqual(
					hiddenColors.palette[hiddenColors.indexed[0]],
					[200, 80, 16],
				);
				assert.ok(hiddenColors.indexed.slice(1).every((index) => index === 0));
			},
		);
		for (const disposal of [1, 2, 3]) {
			for (const trimmed of [false, true]) {
				responses.length = 0;
				scope.onmessage?.({
					data: {
						type: "crop",
						buffer: patchAnimation(disposal),
						area: { x: 0, y: 0, width: 4, height: 3 },
						containAspect: 4 / 3,
						maxOutput: 4,
						...(trimmed ? { startTimeMs: 80, endTimeMs: 120 } : { maxFps: 10 }),
					},
				});
				const result = responses.find((message) => message.type === "done");
				assert.ok(result && result.type === "done", JSON.stringify(responses));
				const metadata = preflightAnimatedGif(result.buffer);
				assert.equal(metadata.loopCount, 3);
				assert.equal(metadata.durationMs, trimmed ? 40 : 120);
				assert.equal(metadata.frameCount, trimmed ? 1 : 2);
				assert.deepEqual([metadata.width, metadata.height], [4, 3]);
				const parsed = parseGIF(result.buffer);
				const records = parsed.frames.filter((record) => "image" in record);
				const finalFrame = decompressFrame(
					records[records.length - 1],
					parsed.gct,
					true,
				);
				assert.ok(finalFrame?.patch);
				const pixel = (index: number) =>
					Array.from(finalFrame.patch.subarray(index * 4, index * 4 + 4));
				assert.deepEqual(pixel(1), [0, 255, 0, 255]);
				assert.deepEqual(pixel(2), [255, 0, 0, 255]);
				assert.deepEqual(
					pixel(0),
					disposal === 1
						? [0, 0, 255, 255]
						: disposal === 2
							? [0, 0, 0, 0]
							: [255, 0, 0, 255],
				);
			}
		}
		await t.test(
			"181-frame source can be inspected, trimmed or sampled without raising output limits",
			async () => {
				const buffer = repeatedAnimation(181);
				const metadata = await inspectGifFile(
					new File([buffer], "long.gif", { type: "image/gif" }),
				);
				assert.equal(metadata.frameCount, 181);
				assert.equal(metadata.durationMs, 7240);
				for (const settings of [
					{ startTimeMs: 7000, endTimeMs: 7200 },
					{ maxFps: 12 },
				]) {
					responses.length = 0;
					decodeCalls = 0;
					scope.onmessage?.({
						data: {
							type: "crop",
							buffer,
							area: { x: 0, y: 0, width: 4, height: 3 },
							containAspect: 4 / 3,
							maxOutput: 4,
							...settings,
						},
					});
					const result = responses.find((message) => message.type === "done");
					assert.ok(
						result && result.type === "done",
						JSON.stringify(responses),
					);
					const output = preflightAnimatedGif(result.buffer);
					assert.equal(output.durationMs, "maxFps" in settings ? 7240 : 200);
					assert.equal(output.frameCount, "maxFps" in settings ? 87 : 5);
					assert.equal(decodeCalls, 180); // includes the trimmed/sampled leading patches
				}
				responses.length = 0;
				decodeCalls = 0;
				scope.onmessage?.({
					data: {
						type: "crop",
						buffer,
						area: { x: 0, y: 0, width: 4, height: 3 },
						containAspect: 4 / 3,
						maxOutput: 4,
					},
				});
				assert.ok(
					responses.some(
						(message) =>
							message.type === "error" &&
							message.code === "resource-limit" &&
							message.message.includes("180"),
					),
				);
				assert.equal(decodeCalls, 0);
			},
		);
		await t.test(
			"long combined holds split without a browser-clamped 10ms tail",
			() => {
				responses.length = 0;
				scope.onmessage?.({
					data: {
						type: "crop",
						buffer: repeatedAnimation(2, [655_350, 40]),
						area: { x: 0, y: 0, width: 4, height: 3 },
						containAspect: 4 / 3,
						maxOutput: 4,
						endTimeMs: 655_360,
					},
				});
				const result = responses.find((message) => message.type === "done");
				assert.ok(result && result.type === "done", JSON.stringify(responses));
				const output = preflightAnimatedGif(result.buffer);
				assert.equal(output.durationMs, 655_360);
				assert.deepEqual(output.frameDelaysMs, [655_340, 20]);
			},
		);
		await t.test(
			"oversized selected prefix rejects before parseGIF even when output is trimmed to one frame",
			() => {
				const buffer = largeDescriptorAnimation(101);
				assert.equal(preflightAnimatedGif(buffer).decodedPixels, 240_000_012);
				responses.length = 0;
				parseCalls = 0;
				decodeCalls = 0;
				scope.onmessage?.({
					data: {
						type: "crop",
						buffer,
						area: { x: 0, y: 0, width: 4, height: 3 },
						containAspect: 4 / 3,
						maxOutput: 4,
						startTimeMs: 4000,
						endTimeMs: 4040,
					},
				});
				assert.ok(
					responses.some(
						(message) =>
							message.type === "error" &&
							message.code === "resource-limit" &&
							message.message.includes("解碼"),
					),
				);
				assert.equal(parseCalls, 0);
				assert.equal(decodeCalls, 0);
				// The same source is editable when the chosen early prefix is small.
				responses.length = 0;
				scope.onmessage?.({
					data: {
						type: "crop",
						buffer,
						area: { x: 0, y: 0, width: 4, height: 3 },
						containAspect: 4 / 3,
						maxOutput: 4,
						endTimeMs: 40,
					},
				});
				const result = responses.find((message) => message.type === "done");
				assert.ok(result && result.type === "done", JSON.stringify(responses));
				assert.equal(preflightAnimatedGif(result.buffer).durationMs, 40);
				assert.equal(parseCalls, 1);
				assert.equal(decodeCalls, 1);
			},
		);
	} finally {
		codecMock.restore();
		decoderMock.restore();
		for (const [name, descriptor] of originals) {
			if (descriptor) Object.defineProperty(globalThis, name, descriptor);
			else Reflect.deleteProperty(globalThis, name);
		}
	}
});
