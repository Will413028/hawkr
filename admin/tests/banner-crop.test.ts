import assert from "node:assert/strict";
import test from "node:test";
import {
	BANNER_ASPECT_RATIO,
	BANNER_CANVAS_HEIGHT,
	BANNER_CANVAS_WIDTH,
	calculateCenterCrop,
} from "../src/components/banner/crop";

test("BANNER constants 定義為標準 16:9 規格 (1600x900)", () => {
	assert.equal(BANNER_CANVAS_WIDTH, 1600);
	assert.equal(BANNER_CANVAS_HEIGHT, 900);
	assert.equal(BANNER_ASPECT_RATIO, 16 / 9);
	assert.equal(BANNER_CANVAS_WIDTH / BANNER_CANVAS_HEIGHT, 16 / 9);
});

test("calculateCenterCrop：已經是 16:9 的圖片原樣保留", () => {
	const crop = calculateCenterCrop(1600, 900);
	assert.deepEqual(crop, { sx: 0, sy: 0, sWidth: 1600, sHeight: 900 });

	const cropSmall = calculateCenterCrop(800, 450);
	assert.deepEqual(cropSmall, { sx: 0, sy: 0, sWidth: 800, sHeight: 450 });
});

test("calculateCenterCrop：較寬的圖片（例如 2:1 / 1200x600）置中裁切左右兩側", () => {
	const crop = calculateCenterCrop(1200, 600);
	// 600 * (16 / 9) = 1066.6666666666667
	const expectedWidth = (600 * 16) / 9;
	const expectedSx = (1200 - expectedWidth) / 2;

	assert.equal(crop.sy, 0);
	assert.equal(crop.sHeight, 600);
	assert.ok(Math.abs(crop.sWidth - expectedWidth) < 0.001);
	assert.ok(Math.abs(crop.sx - expectedSx) < 0.001);
	assert.ok(Math.abs(crop.sWidth / crop.sHeight - 16 / 9) < 0.001);
});

test("calculateCenterCrop：較高的圖片（例如 4:3 / 1200x900）置中裁切上下兩側", () => {
	const crop = calculateCenterCrop(1200, 900);
	// 1200 / (16 / 9) = 675
	const expectedHeight = 675;
	const expectedSy = (900 - expectedHeight) / 2;

	assert.equal(crop.sx, 0);
	assert.equal(crop.sWidth, 1200);
	assert.equal(crop.sHeight, 675);
	assert.equal(crop.sy, expectedSy);
	assert.ok(Math.abs(crop.sWidth / crop.sHeight - 16 / 9) < 0.001);
});

test("calculateCenterCrop：非正數保護（邊界處理）", () => {
	const zero = calculateCenterCrop(0, 0);
	assert.deepEqual(zero, { sx: 0, sy: 0, sWidth: 0, sHeight: 0 });
});
