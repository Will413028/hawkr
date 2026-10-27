import assert from "node:assert/strict";
import test from "node:test";
import {
	calculateProductContainRect,
	calculateProductImageContainZoom,
	calculateProductImageDestinationRect,
	calculateProductImageOutputSize,
	drawProductImage,
	PRODUCT_IMAGE_ASPECT,
	productImageTransformWillCrop,
} from "../lib/media/product-image";

test("product output keeps the existing 4:3 surface contract without upscaling", () => {
	assert.equal(PRODUCT_IMAGE_ASPECT, 4 / 3);
	assert.deepEqual(calculateProductImageOutputSize(2400, 1800), {
		width: 1600,
		height: 1200,
	});
	assert.deepEqual(calculateProductImageOutputSize(640, 960), {
		width: 960,
		height: 720,
	});
	assert.deepEqual(calculateProductImageOutputSize(400, 300), {
		width: 400,
		height: 300,
	});
});

test("complete-image mode keeps every edge of square and portrait uploads", () => {
	assert.deepEqual(calculateProductContainRect(1000, 1000, 1000, 750), {
		x: 125,
		y: 0,
		width: 750,
		height: 750,
	});
	assert.deepEqual(calculateProductContainRect(900, 1600, 1600, 1200), {
		x: 462.5,
		y: 0,
		width: 675,
		height: 1200,
	});
});

test("the cropper's displayed 100% is the full-image zoom", () => {
	assert.equal(calculateProductImageContainZoom(4 / 3), 1);
	assert.equal(calculateProductImageContainZoom(600 / 1000), 0.45);
	assert.equal(calculateProductImageContainZoom(16 / 9), 0.75);
});

test("only a seller-adjusted transform can move pixels outside the canvas", () => {
	assert.deepEqual(calculateProductImageDestinationRect(600, 1000, 1000, 750), {
		x: 275,
		y: 0,
		width: 450,
		height: 750,
	});
	assert.equal(
		productImageTransformWillCrop(600, 1000, 1000, 750, {
			scale: 1,
			offsetX: 0,
			offsetY: 0,
		}),
		false,
	);
	assert.equal(
		productImageTransformWillCrop(600, 1000, 1000, 750, {
			scale: 2,
			offsetX: 0,
			offsetY: 0,
		}),
		true,
	);
});

test("complete-image canvas draws the full source as its foreground", () => {
	const drawCalls: unknown[][] = [];
	const context = {
		clearRect() {},
		drawImage(...args: unknown[]) {
			drawCalls.push(args);
		},
		fillRect() {},
		fillStyle: "",
		filter: "none",
		imageSmoothingEnabled: false,
		imageSmoothingQuality: "low",
		restore() {},
		save() {},
	} as unknown as CanvasRenderingContext2D;
	const image = {} as CanvasImageSource;

	drawProductImage(context, image, 900, 1600, {
		targetWidth: 1600,
		targetHeight: 1200,
	});

	assert.equal(drawCalls.length, 1);
	assert.deepEqual(drawCalls.at(-1)?.slice(1, 5), [0, 0, 900, 1600]);
});

test("custom zoom and pan are encoded as destination geometry", () => {
	const drawCalls: unknown[][] = [];
	const context = {
		clearRect() {},
		drawImage(...args: unknown[]) {
			drawCalls.push(args);
		},
		fillRect() {},
		fillStyle: "",
		imageSmoothingEnabled: false,
		imageSmoothingQuality: "low",
	} as unknown as CanvasRenderingContext2D;
	const image = {} as CanvasImageSource;

	drawProductImage(context, image, 600, 1000, {
		transform: { scale: 2, offsetX: 0.05, offsetY: 0.1 },
		targetWidth: 1000,
		targetHeight: 750,
	});

	assert.deepEqual(drawCalls.at(-1)?.slice(5), [100, -300, 900, 1500]);
});

test("invalid source sizes fail before a broken canvas is produced", () => {
	assert.throws(() => calculateProductImageOutputSize(0, 1200));
	assert.throws(() => calculateProductImageOutputSize(1200, Number.NaN));
});
