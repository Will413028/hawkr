import assert from "node:assert/strict";
import test from "node:test";
import {
	AVATAR_BACKGROUND_PRESETS,
	AVATAR_DEFAULT_BACKGROUND,
	AVATAR_LOGO_MIN_SCALE,
	AVATAR_OUTPUT,
	analyzeAlphaPixels,
	calculateAvatarLogoDestination,
	calculateAvatarLogoMaxScale,
	calculateAvatarLogoRect,
	constrainAvatarLogoTransform,
	shouldUseAvatarLogoFit,
} from "../lib/media/avatar-logo";

function alphaFixture(
	width: number,
	height: number,
	visible: (x: number, y: number) => boolean,
): Uint8ClampedArray {
	const pixels = new Uint8ClampedArray(width * height * 4);
	for (let y = 0; y < height; y += 1) {
		for (let x = 0; x < width; x += 1) {
			pixels[(y * width + x) * 4 + 3] = visible(x, y) ? 255 : 0;
		}
	}
	return pixels;
}

test("transparent logo analysis trims to the non-transparent bounds", () => {
	const analysis = analyzeAlphaPixels(
		alphaFixture(6, 6, (x, y) => x >= 2 && x <= 3 && y >= 1 && y <= 4),
		6,
		6,
	);

	assert.equal(analysis.hasTransparency, true);
	assert.deepEqual(analysis.visibleBounds, {
		x: 2,
		y: 1,
		width: 2,
		height: 4,
	});
	assert.equal(analysis.visiblePixelRatio, 8 / 36);
	assert.equal(analysis.visibleBoundsAreaRatio, 8 / 36);
	assert.equal(shouldUseAvatarLogoFit("image/png", analysis), true);
});

test("opaque portrait stays in fill mode", () => {
	const analysis = analyzeAlphaPixels(
		alphaFixture(4, 4, () => true),
		4,
		4,
	);

	assert.equal(analysis.hasTransparency, false);
	assert.deepEqual(analysis.visibleBounds, {
		x: 0,
		y: 0,
		width: 4,
		height: 4,
	});
	assert.equal(shouldUseAvatarLogoFit("image/png", analysis), false);
	assert.equal(shouldUseAvatarLogoFit("image/jpeg", analysis), false);
});

test("complete-logo mode reserves a circular-mask-safe 15 percent inset", () => {
	const rect = calculateAvatarLogoRect(400, 200);

	assert.deepEqual(rect, {
		x: 96,
		y: 208,
		width: 448,
		height: 224,
	});
	assert.equal(AVATAR_OUTPUT.width, 640);
	assert.equal(AVATAR_OUTPUT.height, 640);
});

test("complete-logo default transform preserves the original centered output", () => {
	assert.deepEqual(
		calculateAvatarLogoDestination(400, 200),
		calculateAvatarLogoRect(400, 200),
	);
});

test("complete-logo drag is clamped so every visible corner stays in the circle", () => {
	const transform = constrainAvatarLogoTransform(400, 200, {
		offsetX: 500,
		offsetY: 500,
		scale: 1,
	});
	const destination = calculateAvatarLogoDestination(400, 200, transform);
	const radius = AVATAR_OUTPUT.width / 2;
	const corners = [
		[destination.x, destination.y],
		[destination.x + destination.width, destination.y],
		[destination.x, destination.y + destination.height],
		[destination.x + destination.width, destination.y + destination.height],
	];

	assert.ok(Math.hypot(transform.offsetX, transform.offsetY) > 0);
	for (const [x, y] of corners) {
		assert.ok(Math.hypot(x - radius, y - radius) <= radius + 0.0001);
	}
});

test("complete-logo scale stays inside its safe dynamic limits", () => {
	const maxScale = calculateAvatarLogoMaxScale(400, 200);
	const tooLarge = constrainAvatarLogoTransform(400, 200, {
		offsetX: 100,
		offsetY: 0,
		scale: 99,
	});
	const tooSmall = constrainAvatarLogoTransform(400, 200, {
		offsetX: 0,
		offsetY: 0,
		scale: 0.1,
	});

	assert.equal(tooLarge.scale, maxScale);
	assert.ok(Math.hypot(tooLarge.offsetX, tooLarge.offsetY) < 0.0001);
	assert.equal(tooSmall.scale, AVATAR_LOGO_MIN_SCALE);
});

test("avatar backgrounds include the required consistent and transparent choices", () => {
	assert.deepEqual(
		AVATAR_BACKGROUND_PRESETS.map(({ value }) => value),
		["#FFFDF7", "#FFFFFF", "#B8441F", "transparent"],
	);
	assert.equal(AVATAR_DEFAULT_BACKGROUND, "#FFFDF7");
});
