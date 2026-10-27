import assert from "node:assert/strict";
import test from "node:test";
import { cropAnimatedGifFile, GifCropError } from "../lib/media/gif-crop";
import type {
	GifCropErrorCode,
	GifCropWorkerRequest,
	GifCropWorkerResponse,
} from "../lib/media/gif-crop-protocol";

const sourceBytes = new TextEncoder().encode("GIF89a-test-source");
const area = { x: 1, y: 2, width: 400, height: 300 };
const options = {
	containAspect: 4 / 3,
	maxOutput: 540,
	maxFps: 12,
	startTimeMs: 100,
	endTimeMs: 900,
	autoFit: true,
};
const file = () =>
	new File([sourceBytes], "animation.gif", {
		type: "image/gif",
		lastModified: 123,
	});

async function withWorkers(
	onRequest: (worker: StubWorker, index: number) => void,
	run: (workers: StubWorker[]) => Promise<void>,
) {
	const workers: StubWorker[] = [];
	const originals = ["Worker", "OffscreenCanvas"].map(
		(key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)] as const,
	);
	class Worker extends StubWorker {
		constructor() {
			super((worker) => onRequest(worker, workers.indexOf(worker)));
			assert.ok(workers.every((worker) => worker.terminated));
			workers.push(this);
		}
	}
	Object.defineProperty(globalThis, "Worker", {
		configurable: true,
		value: Worker,
	});
	Object.defineProperty(globalThis, "OffscreenCanvas", {
		configurable: true,
		value: class {},
	});
	try {
		await run(workers);
		assert.ok(workers.every((worker) => worker.terminated));
	} finally {
		for (const [key, descriptor] of originals) {
			if (descriptor) Object.defineProperty(globalThis, key, descriptor);
			else Reflect.deleteProperty(globalThis, key);
		}
	}
}

class StubWorker {
	onmessage: ((event: { data: GifCropWorkerResponse }) => void) | null = null;
	onerror: (() => void) | null = null;
	terminated = false;
	request!: GifCropWorkerRequest;
	constructor(private readonly onRequest: (worker: StubWorker) => void) {}
	postMessage(request: GifCropWorkerRequest, transfer: Transferable[]) {
		this.request = structuredClone(request, { transfer });
		assert.equal(request.buffer.byteLength, 0, "input ownership must transfer");
		assert.deepEqual(new Uint8Array(this.request.buffer), sourceBytes);
		this.onRequest(this);
	}
	terminate() {
		this.terminated = true;
	}
	send(data: GifCropWorkerResponse) {
		this.onmessage?.({ data });
	}
	fail(code: GifCropErrorCode) {
		this.send({ type: "error", code, message: code });
	}
	done() {
		this.send({ type: "done", buffer: sourceBytes.slice().buffer });
	}
}

test("automatic fitting retries only dimensions with fresh bytes and one active worker", async () => {
	const progress: number[] = [];
	await withWorkers(
		(worker, index) => {
			worker.send({ type: "progress", progress: 0.5 });
			if (index === 0) worker.fail("output-too-large");
			else worker.done();
		},
		async (workers) => {
			const original = file();
			const result = await cropAnimatedGifFile(original, area, {
				...options,
				onProgress: (value) => progress.push(value),
			});
			assert.deepEqual(
				workers.map((worker) => worker.request.maxOutput),
				[540, 360],
			);
			for (const worker of workers) {
				assert.deepEqual(worker.request.area, area);
				assert.equal(worker.request.maxFps, 12);
				assert.equal(worker.request.startTimeMs, 100);
				assert.equal(worker.request.endTimeMs, 900);
			}
			assert.equal(result.name, "animation-crop.gif");
			assert.equal(result.lastModified, original.lastModified);
			assert.equal(result.type, "image/gif");
			assert.deepEqual(
				new Uint8Array(await original.arrayBuffer()),
				sourceBytes,
			);
			assert.equal(progress.at(-1), 1);
			assert.deepEqual(
				progress,
				[...progress].sort((a, b) => a - b),
			);
		},
	);
});

test("automatic fitting has a finite retry budget and surfaces final output failure", async () => {
	await withWorkers(
		(worker) => worker.fail("output-resource-limit"),
		async (workers) => {
			await assert.rejects(cropAnimatedGifFile(file(), area, options), {
				code: "output-resource-limit",
			});
			assert.deepEqual(
				workers.map((worker) => worker.request.maxOutput),
				[540, 360, 240],
			);
		},
	);
});

test("automatic fitting skips repeated physical sizes for a smaller source crop", async () => {
	await withWorkers(
		(worker, index) =>
			index === 0 ? worker.fail("output-too-large") : worker.done(),
		async (workers) => {
			await cropAnimatedGifFile(
				file(),
				{ x: 0, y: 0, width: 300, height: 225 },
				options,
			);
			assert.deepEqual(
				workers.map((worker) => worker.request.maxOutput),
				[540, 240],
			);
		},
	);
});

test("manual dimensions and non-output failures never silently downgrade or retry", async () => {
	for (const code of [
		"output-too-large",
		"invalid-file",
		"resource-limit",
		"processing-failed",
	] as const) {
		await withWorkers(
			(worker) => worker.fail(code),
			async (workers) => {
				await assert.rejects(
					cropAnimatedGifFile(file(), area, {
						...options,
						autoFit: code !== "output-too-large",
					}),
					(error) => error instanceof GifCropError && error.code === code,
				);
				assert.equal(workers.length, 1);
			},
		);
	}
});

test("cancel during an automatic retry terminates it without starting another", async () => {
	const controller = new AbortController();
	await withWorkers(
		(worker, index) => {
			if (index === 0) worker.fail("output-too-large");
			else queueMicrotask(() => controller.abort());
		},
		async (workers) => {
			await assert.rejects(
				cropAnimatedGifFile(file(), area, {
					...options,
					signal: controller.signal,
				}),
				{ name: "AbortError" },
			);
			assert.equal(workers.length, 2);
		},
	);
});

test("cancellation between attempts prevents the next worker", async () => {
	const controller = new AbortController();
	await withWorkers(
		(worker) => {
			worker.fail("output-too-large");
			queueMicrotask(() => controller.abort());
		},
		async (workers) => {
			await assert.rejects(
				cropAnimatedGifFile(file(), area, {
					...options,
					signal: controller.signal,
				}),
				{ name: "AbortError" },
			);
			assert.equal(workers.length, 1);
		},
	);
});

test("posting failures clean up workers and presentation exceptions cannot strand output", async () => {
	await withWorkers(
		() => {
			throw new Error("post failed");
		},
		async (workers) => {
			await assert.rejects(cropAnimatedGifFile(file(), area, options), {
				code: "processing-failed",
			});
			assert.equal(workers.length, 1);
		},
	);
	await withWorkers(
		(worker) => worker.done(),
		async () => {
			const result = await cropAnimatedGifFile(file(), area, {
				...options,
				onProgress: () => {
					throw new Error("render failed");
				},
			});
			assert.equal(result.size, sourceBytes.length);
		},
	);
});
