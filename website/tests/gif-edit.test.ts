import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { preflightAnimatedGif } from "../lib/media/gif-crop-preflight";
import { inspectGifFile, planGifEdits } from "../lib/media/gif-edit";
import { getGifPlaybackDelayMs } from "../lib/media/gif-timing";

const { GIFEncoder } = createRequire(import.meta.url)(
	"gifenc",
) as typeof import("gifenc");

test("original frames retain browser playback durations including zero and final zero", () => {
	const plan = planGifEdits([0, 40, 70, 0]);
	assert.deepEqual(plan.frames, [
		{ sourceFrameIndex: 0, delayMs: 100 },
		{ sourceFrameIndex: 1, delayMs: 40 },
		{ sourceFrameIndex: 2, delayMs: 70 },
		{ sourceFrameIndex: 3, delayMs: 100 },
	]);
	assert.equal(plan.durationMs, 310);
	assert.equal(plan.decodeThroughIndex, 3);
});

test("sampling reduces output frames while preserving original playback duration", () => {
	const plan = planGifEdits(
		Array.from({ length: 10 }, () => 40),
		{ maxFps: 12 },
	);
	assert.deepEqual(plan.frames, [
		{ sourceFrameIndex: 0, delayMs: 90 },
		{ sourceFrameIndex: 2, delayMs: 80 },
		{ sourceFrameIndex: 4, delayMs: 80 },
		{ sourceFrameIndex: 6, delayMs: 90 },
		{ sourceFrameIndex: 8, delayMs: 60 },
	]);
	assert.equal(
		plan.frames.reduce((sum, frame) => sum + frame.delayMs, 0),
		400,
	);
	assert.equal(plan.decodeThroughIndex, 8);
});

test("trimming clips boundary frame delays and keeps their source indexes", () => {
	const plan = planGifEdits([100, 100, 100, 100], {
		startTimeMs: 154,
		endTimeMs: 346,
	});
	assert.deepEqual(plan.frames, [
		{ sourceFrameIndex: 1, delayMs: 50 },
		{ sourceFrameIndex: 2, delayMs: 100 },
		{ sourceFrameIndex: 3, delayMs: 50 },
	]);
	assert.equal(plan.durationMs, 200);
	assert.equal(plan.decodeThroughIndex, 3);
});

test("variable-delay sampling retains slow holds and sums omitted delays", () => {
	const plan = planGifEdits([0, 40, 40, 400, 40, 40, 40], { maxFps: 10 });
	assert.deepEqual(plan.frames, [
		{ sourceFrameIndex: 0, delayMs: 100 },
		{ sourceFrameIndex: 1, delayMs: 100 },
		{ sourceFrameIndex: 3, delayMs: 400 },
		{ sourceFrameIndex: 4, delayMs: 100 },
	]);
	assert.equal(plan.durationMs, 700);
	// The final dropped deltas do not affect any later retained output frame.
	assert.equal(plan.decodeThroughIndex, 4);
});

test("a long pose beginning between sampling ticks remains visible", () => {
	assert.deepEqual(planGifEdits([40, 40, 400], { maxFps: 10 }).frames, [
		{ sourceFrameIndex: 0, delayMs: 100 },
		{ sourceFrameIndex: 2, delayMs: 380 },
	]);
});

test("trim and sampling preserve selected duration without decoding irrelevant tail", () => {
	const plan = planGifEdits([40, 40, 40, 40, 40, 40, 40], {
		startTimeMs: 50,
		endTimeMs: 220,
		maxFps: 10,
	});
	assert.deepEqual(plan.frames, [
		{ sourceFrameIndex: 1, delayMs: 100 },
		{ sourceFrameIndex: 3, delayMs: 70 },
	]);
	assert.equal(plan.durationMs, 170);
	assert.equal(plan.decodeThroughIndex, 3);
});

test("all-zero-delay animations have a usable browser playback timeline", () => {
	assert.deepEqual(planGifEdits([0, 0]).frames, [
		{ sourceFrameIndex: 0, delayMs: 100 },
		{ sourceFrameIndex: 1, delayMs: 100 },
	]);
	const sampled = planGifEdits([0, 0], { maxFps: 12 });
	assert.equal(sampled.durationMs, 200);
	assert.equal(sampled.frames.length, 2);
	const trimmed = planGifEdits([0, 0], { startTimeMs: 50, endTimeMs: 150 });
	assert.deepEqual(trimmed.frames, [
		{ sourceFrameIndex: 0, delayMs: 50 },
		{ sourceFrameIndex: 1, delayMs: 50 },
	]);
});

test("invalid controls and selections shorter than 20 ms fail closed", () => {
	for (const settings of [
		{ maxFps: 0 },
		{ maxFps: Number.NaN },
		{ maxFps: 101 },
		{ startTimeMs: -10 },
		{ startTimeMs: -1 },
		{ startTimeMs: 50, endTimeMs: 50 },
		{ startTimeMs: 1, endTimeMs: 4 },
		{ startTimeMs: 0, endTimeMs: 10 },
		{ endTimeMs: 110 },
		{ endTimeMs: Number.POSITIVE_INFINITY },
	]) {
		assert.throws(() => planGifEdits([50, 50], settings));
	}
	assert.throws(() => planGifEdits([100], { endTimeMs: 10 }), /0\.02 秒/);
	assert.equal(planGifEdits([100], { endTimeMs: 20 }).durationMs, 20);
});

test("100 frames with raw 10 ms delays still play for 10 seconds after sampling", () => {
	for (const maxFps of [undefined, 12, 8]) {
		const plan = planGifEdits(
			Array.from({ length: 100 }, () => 10),
			{ maxFps },
		);
		assert.equal(plan.durationMs, 10_000);
		assert.equal(
			plan.frames.reduce(
				(sum, frame) => sum + getGifPlaybackDelayMs(frame.delayMs),
				0,
			),
			10_000,
		);
		assert.ok(plan.frames.every((frame) => frame.delayMs >= 20));
	}
});

test("sampling an ordinary 100 ms animation cannot create a browser-clamped final hold", () => {
	const plan = planGifEdits([40, 40, 20], { maxFps: 12 });
	assert.deepEqual(plan.frames, [
		{ sourceFrameIndex: 0, delayMs: 80 },
		{ sourceFrameIndex: 2, delayMs: 20 },
	]);
	assert.equal(plan.durationMs, 100);
	assert.equal(plan.decodeThroughIndex, 2);
});

test("a 0.93-second trim stays 0.93 seconds while preserving its short sampled tail", () => {
	const plan = planGifEdits(
		Array.from({ length: 25 }, () => 40),
		{
			maxFps: 12,
			endTimeMs: 930,
		},
	);
	assert.equal(plan.durationMs, 930);
	assert.equal(
		plan.frames.reduce((sum, frame) => sum + frame.delayMs, 0),
		930,
	);
	assert.ok(plan.frames.every((frame) => frame.delayMs >= 20));
	assert.deepEqual(plan.frames.slice(-2), [
		{ sourceFrameIndex: 21, delayMs: 70 },
		{ sourceFrameIndex: 23, delayMs: 20 },
	]);
});

test("original-rate trimming borrows for short start and end slices without stretching time", () => {
	assert.deepEqual(
		planGifEdits([100, 100], {
			startTimeMs: 90,
			endTimeMs: 150,
		}).frames,
		[
			{ sourceFrameIndex: 0, delayMs: 20 },
			{ sourceFrameIndex: 1, delayMs: 40 },
		],
	);
	assert.deepEqual(
		planGifEdits([100, 100], {
			startTimeMs: 50,
			endTimeMs: 110,
		}).frames,
		[
			{ sourceFrameIndex: 0, delayMs: 40 },
			{ sourceFrameIndex: 1, delayMs: 20 },
		],
	);
});

test("short slices merge only when neither neighbor can donate while staying playable", () => {
	assert.deepEqual(
		planGifEdits([100, 100], {
			startTimeMs: 90,
			endTimeMs: 110,
		}).frames,
		[{ sourceFrameIndex: 1, delayMs: 20 }],
	);
});

test("variable source delays and trims always encode the exact browser playback duration", () => {
	let randomState = 20260908;
	const next = () => {
		randomState = (Math.imul(randomState, 1664525) + 1013904223) >>> 0;
		return randomState / 2 ** 32;
	};
	for (let sample = 0; sample < 500; sample += 1) {
		const delays = Array.from(
			{ length: 1 + Math.floor(next() * 40) },
			() => Math.floor(next() * 50) * 10,
		);
		const total = delays.reduce(
			(sum, delay) => sum + getGifPlaybackDelayMs(delay),
			0,
		);
		const startTimeMs = Math.floor((next() * (total - 10)) / 10) * 10;
		const endTimeMs =
			startTimeMs +
			20 +
			Math.floor((next() * (total - startTimeMs - 10)) / 10) * 10;
		const maxFps = [undefined, 8, 12, 30, 60, 100][Math.floor(next() * 6)];
		const plan = planGifEdits(delays, { startTimeMs, endTimeMs, maxFps });
		assert.ok(
			plan.frames.every(
				(frame) => frame.delayMs >= 20 && frame.delayMs % 10 === 0,
			),
		);
		assert.equal(
			plan.frames.reduce(
				(sum, frame) => sum + getGifPlaybackDelayMs(frame.delayMs),
				0,
			),
			endTimeMs - startTimeMs,
		);
		assert.equal(plan.durationMs, endTimeMs - startTimeMs);
		assert.equal(plan.decodeThroughIndex, plan.frames.at(-1)?.sourceFrameIndex);
		assert.ok(
			plan.frames.every(
				(frame, index) =>
					index === 0 ||
					frame.sourceFrameIndex > plan.frames[index - 1].sourceFrameIndex,
			),
		);
	}
});

test("metadata inspection reads GIF timing without needing browser canvas APIs", async () => {
	const encoder = GIFEncoder();
	for (const delay of [0, 40, 90]) {
		encoder.writeFrame(new Uint8Array(12), 4, 3, {
			palette: [[20, 120, 20]],
			delay,
			repeat: 2,
		});
	}
	encoder.finish();
	const file = new File([new Uint8Array(encoder.bytes())], "test.gif", {
		type: "image/gif",
	});
	assert.deepEqual(await inspectGifFile(file), {
		width: 4,
		height: 3,
		frameCount: 3,
		durationMs: 230,
	});
	// The structural scanner still reports the GIF's raw timing metadata.
	assert.equal(preflightAnimatedGif(await file.arrayBuffer()).durationMs, 130);
	const controller = new AbortController();
	controller.abort();
	await assert.rejects(inspectGifFile(file, controller.signal), {
		name: "AbortError",
	});
	await assert.rejects(
		inspectGifFile(new File(["bad"], "bad.gif", { type: "image/gif" })),
	);
});

test("metadata exposes the playable duration of all-zero animations for trimming", async () => {
	const encoder = GIFEncoder();
	for (let frame = 0; frame < 3; frame += 1) {
		encoder.writeFrame(new Uint8Array(12), 4, 3, {
			palette: [[20, 120, 20]],
			delay: 0,
			repeat: 0,
		});
	}
	encoder.finish();
	const file = new File([new Uint8Array(encoder.bytes())], "zero.gif", {
		type: "image/gif",
	});
	assert.equal((await inspectGifFile(file)).durationMs, 300);
});
