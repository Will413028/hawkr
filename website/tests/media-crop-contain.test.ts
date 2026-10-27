import assert from "node:assert/strict";
import test from "node:test";
import { cropImageFile } from "../lib/media/crop";

interface CapturedCanvas {
	width: number;
	height: number;
	drawCalls: unknown[][];
	fillCalls: unknown[][];
	encodedMime: string | undefined;
}

function installCanvasHarness(): {
	canvases: CapturedCanvas[];
	restore: () => void;
} {
	const imageDescriptor = Object.getOwnPropertyDescriptor(globalThis, "Image");
	const documentDescriptor = Object.getOwnPropertyDescriptor(
		globalThis,
		"document",
	);
	const canvases: CapturedCanvas[] = [];

	class TestImage {
		onload: (() => void) | null = null;
		onerror: (() => void) | null = null;

		set src(_value: string) {
			queueMicrotask(() => this.onload?.());
		}
	}

	Object.defineProperty(globalThis, "Image", {
		configurable: true,
		value: TestImage,
	});
	Object.defineProperty(globalThis, "document", {
		configurable: true,
		value: {
			createElement(tagName: string) {
				assert.equal(tagName, "canvas");
				const captured: CapturedCanvas = {
					width: 0,
					height: 0,
					drawCalls: [],
					fillCalls: [],
					encodedMime: undefined,
				};
				const canvas = {
					get width() {
						return captured.width;
					},
					set width(value: number) {
						captured.width = value;
					},
					get height() {
						return captured.height;
					},
					set height(value: number) {
						captured.height = value;
					},
					getContext(contextId: string) {
						assert.equal(contextId, "2d");
						return {
							drawImage: (...args: unknown[]) => captured.drawCalls.push(args),
							fillRect: (...args: unknown[]) => captured.fillCalls.push(args),
						};
					},
					toBlob(callback: (blob: Blob | null) => void, mime?: string) {
						captured.encodedMime = mime;
						callback(new Blob([], { type: mime }));
					},
				};
				canvases.push(captured);
				return canvas;
			},
		},
	});

	return {
		canvases,
		restore() {
			if (imageDescriptor) {
				Object.defineProperty(globalThis, "Image", imageDescriptor);
			} else {
				Reflect.deleteProperty(globalThis, "Image");
			}
			if (documentDescriptor) {
				Object.defineProperty(globalThis, "document", documentDescriptor);
			} else {
				Reflect.deleteProperty(globalThis, "document");
			}
		},
	};
}

test("non-4:3 post crops are contained inside transparent 4:3 output pixels", async () => {
	const harness = installCanvasHarness();
	try {
		const portrait = await cropImageFile(
			new File(["portrait"], "portrait.jpg", { type: "image/jpeg" }),
			{ x: 10, y: 20, width: 1600, height: 2000 },
			{ maxOutput: 1600, containAspect: 4 / 3 },
		);
		const wide = await cropImageFile(
			new File(["wide"], "wide.jpg", { type: "image/jpeg" }),
			{ x: 30, y: 40, width: 2400, height: 1350 },
			{ maxOutput: 1600, containAspect: 4 / 3 },
		);

		assert.equal(harness.canvases.length, 2);
		const [portraitCanvas, wideCanvas] = harness.canvases;

		assert.deepEqual(
			{ width: portraitCanvas.width, height: portraitCanvas.height },
			{ width: 1600, height: 1200 },
		);
		assert.deepEqual(
			portraitCanvas.drawCalls[0]?.slice(1),
			[10, 20, 1600, 2000, 320, 0, 960, 1200],
		);

		assert.deepEqual(
			{ width: wideCanvas.width, height: wideCanvas.height },
			{ width: 1600, height: 1200 },
		);
		assert.deepEqual(
			wideCanvas.drawCalls[0]?.slice(1),
			[30, 40, 2400, 1350, 0, 150, 1600, 900],
		);

		for (const canvas of harness.canvases) {
			assert.equal(canvas.width / canvas.height, 4 / 3);
			assert.deepEqual(canvas.fillCalls, []);
			assert.equal(canvas.encodedMime, "image/png");
		}
		assert.equal(portrait.type, "image/png");
		assert.equal(portrait.name, "portrait-crop.png");
		assert.equal(wide.type, "image/png");
		assert.equal(wide.name, "wide-crop.png");
	} finally {
		harness.restore();
	}
});

test("an exact 4:3 JPEG stays JPEG because no transparent padding is needed", async () => {
	const harness = installCanvasHarness();
	try {
		const result = await cropImageFile(
			new File(["landscape"], "landscape.jpg", { type: "image/jpeg" }),
			{ x: 0, y: 0, width: 2400, height: 1800 },
			{ maxOutput: 1600, containAspect: 4 / 3 },
		);

		const canvas = harness.canvases[0];
		assert.deepEqual(
			{ width: canvas.width, height: canvas.height },
			{ width: 1600, height: 1200 },
		);
		assert.deepEqual(
			canvas.drawCalls[0]?.slice(1),
			[0, 0, 2400, 1800, 0, 0, 1600, 1200],
		);
		assert.equal(canvas.encodedMime, "image/jpeg");
		assert.equal(result.type, "image/jpeg");
		assert.equal(result.name, "landscape-crop.jpg");
	} finally {
		harness.restore();
	}
});
