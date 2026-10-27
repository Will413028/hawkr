import assert from "node:assert/strict";
import test from "node:test";
import {
	GifPreflightError,
	preflightAnimatedGif,
} from "../lib/media/gif-crop-preflight";
import {
	GIF_CROP_MAX_BYTES,
	GIF_CROP_MAX_DECODED_PIXELS,
	GIF_CROP_MAX_OUTPUT_PIXELS,
	GIF_CROP_MAX_RECORDS,
	GIF_CROP_MAX_SOURCE_FRAMES,
} from "../lib/media/gif-crop-protocol";

const ascii = (value: string) =>
	Array.from(value, (char) => char.charCodeAt(0));

function gifBytes(options?: {
	beforeImage?: number[];
	frameWidth?: number;
	frameHeight?: number;
	trailer?: boolean;
}): Uint8Array {
	const frameWidth = options?.frameWidth ?? 1;
	const frameHeight = options?.frameHeight ?? 1;
	return new Uint8Array([
		...ascii("GIF89a"),
		1,
		0,
		1,
		0,
		0x80,
		0,
		0,
		0,
		0,
		0,
		255,
		255,
		255,
		...(options?.beforeImage ?? []),
		0x2c,
		0,
		0,
		0,
		0,
		frameWidth & 0xff,
		(frameWidth >> 8) & 0xff,
		frameHeight & 0xff,
		(frameHeight >> 8) & 0xff,
		0,
		2,
		2,
		0x44,
		0x01,
		0,
		...(options?.trailer === false ? [] : [0x3b]),
	]);
}

test("GIF preflight reads loop metadata only from application extensions", () => {
	const realLoop = [0x21, 0xff, 11, ...ascii("NETSCAPE2.0"), 3, 1, 5, 0, 0];
	assert.equal(
		preflightAnimatedGif(gifBytes({ beforeImage: realLoop }).buffer).loopCount,
		5,
	);

	const fakeLoopPayload = [...ascii("NETSCAPE2.0"), 3, 1, 7, 0];
	const comment = [0x21, 0xfe, fakeLoopPayload.length, ...fakeLoopPayload, 0];
	assert.equal(
		preflightAnimatedGif(gifBytes({ beforeImage: comment }).buffer).loopCount,
		-1,
	);
});

test("GIF preflight rejects out-of-bounds frame descriptors before decode", () => {
	assert.throws(
		() =>
			preflightAnimatedGif(
				gifBytes({ frameWidth: 4096, frameHeight: 4096 }).buffer,
			),
		(error: unknown) =>
			error instanceof GifPreflightError && error.code === "invalid-file",
	);
});

test("GIF preflight bounds extension-object allocation and truncated data", () => {
	const emptyComments = Array.from({ length: GIF_CROP_MAX_RECORDS + 1 }, () => [
		0x21, 0xfe, 0,
	]).flat();
	assert.throws(
		() => preflightAnimatedGif(gifBytes({ beforeImage: emptyComments }).buffer),
		(error: unknown) =>
			error instanceof GifPreflightError && error.code === "resource-limit",
	);
	assert.throws(
		() => preflightAnimatedGif(gifBytes({ trailer: false }).buffer),
		(error: unknown) =>
			error instanceof GifPreflightError && error.code === "invalid-file",
	);
});

/** Descriptor-only fixtures exercise preflight, never the actual LZW decoder. */
function structuralAnimation(
	width: number,
	height: number,
	delaysMs: number[],
): ArrayBuffer {
	const frame = Array.from(gifBytes()).slice(19, -1);
	frame[5] = width & 0xff;
	frame[6] = (width >> 8) & 0xff;
	frame[7] = height & 0xff;
	frame[8] = (height >> 8) & 0xff;
	const header = Array.from(gifBytes()).slice(0, 19);
	header[6] = width & 0xff;
	header[7] = (width >> 8) & 0xff;
	header[8] = height & 0xff;
	header[9] = (height >> 8) & 0xff;
	return new Uint8Array([
		...header,
		...delaysMs.flatMap((delayMs) => [
			0x21,
			0xf9,
			4,
			0,
			(delayMs / 10) & 0xff,
			((delayMs / 10) >> 8) & 0xff,
			0,
			0,
			...frame,
		]),
		0x3b,
	]).buffer;
}

test("preflight reads raw zero delays and cumulative duration", () => {
	const metadata = preflightAnimatedGif(structuralAnimation(4, 3, [0, 40, 90]));
	assert.deepEqual(metadata.frameDelaysMs, [0, 40, 90]);
	assert.equal(metadata.durationMs, 130);
	assert.equal(metadata.decodedPixels, 36);
});

test("121-frame 1280x720 input fits bounded sequential decode work independently of output", () => {
	const metadata = preflightAnimatedGif(
		structuralAnimation(
			1280,
			720,
			Array.from({ length: 121 }, () => 40),
		),
	);
	assert.equal(metadata.decodedPixels, 111_513_600);
	assert.ok(metadata.decodedPixels > GIF_CROP_MAX_OUTPUT_PIXELS);
	assert.ok(metadata.decodedPixels < GIF_CROP_MAX_DECODED_PIXELS);
	assert.equal(metadata.frameCount, 121);
});

test("metadata permits longer sources and exposes selected-prefix work before decoding", () => {
	const largerWork = preflightAnimatedGif(
		structuralAnimation(
			1600,
			1500,
			Array.from({ length: 101 }, () => 40),
		),
	);
	assert.equal(largerWork.decodedPixels, 242_400_000);
	assert.equal(
		largerWork.decodedPixelPrefixes[99],
		GIF_CROP_MAX_DECODED_PIXELS,
	);
	assert.equal(largerWork.decodedPixelPrefixes[100], 242_400_000);
	const longer = preflightAnimatedGif(
		structuralAnimation(
			4,
			3,
			Array.from({ length: 181 }, () => 40),
		),
	);
	assert.equal(longer.frameCount, 181);
	assert.equal(longer.durationMs, 7_240);
	assert.equal(longer.decodedPixelPrefixes.length, 181);
	assert.equal(longer.decodedPixelPrefixes[180], 181 * 12);
});

test("metadata keeps hard source-frame and compressed-byte budgets", () => {
	assert.equal(
		preflightAnimatedGif(
			structuralAnimation(
				4,
				3,
				Array.from({ length: GIF_CROP_MAX_SOURCE_FRAMES }, () => 40),
			),
		).frameCount,
		GIF_CROP_MAX_SOURCE_FRAMES,
	);
	for (const buffer of [
		structuralAnimation(
			4,
			3,
			Array.from({ length: GIF_CROP_MAX_SOURCE_FRAMES + 1 }, () => 40),
		),
		new ArrayBuffer(GIF_CROP_MAX_BYTES + 1),
	]) {
		assert.throws(
			() => preflightAnimatedGif(buffer),
			(error: unknown) =>
				error instanceof GifPreflightError && error.code === "resource-limit",
		);
	}
});
