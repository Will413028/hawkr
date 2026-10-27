import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
	CROP_PRESETS,
	closestAspectOption,
	POST_ASPECT_OPTIONS,
	POST_FEED_ASPECT,
	parseCustomAspectInput,
} from "../components/ui/ImageCropDialog";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("seller cover upload enables the adjustable fill editor", () => {
	const preset = CROP_PRESETS.sellerCover;

	assert.equal(preset.aspect, 4);
	assert.equal(preset.sellerCoverEditor, true);
	assert.equal(preset.confirmLabel, "確認上傳");
	assert.ok(preset.surfaces?.length);
	assert.ok(preset.surfaces?.every((surface) => surface.fit === "cover"));
	assert.ok(preset.surfaces?.every((surface) => surface.aspect === 4));
});

test("avatar upload uses the canonical logo editor and real display sizes", () => {
	const preset = CROP_PRESETS.avatar;

	assert.equal(preset.aspect, 1);
	assert.equal(preset.avatarEditor, true);
	assert.equal(preset.maxOutput, 640);
	assert.deepEqual(
		preset.surfaces?.map(({ size, circle }) => ({ size, circle })),
		[
			{ size: 84, circle: true },
			{ size: 44, circle: true },
			{ size: 30, circle: true },
		],
	);
});

test("complete-logo preview wires real pointer and keyboard positioning into output", () => {
	const dialog = source("../components/ui/ImageCropDialog.tsx");

	assert.match(dialog, /data-avatar-logo-control="true"/);
	assert.match(dialog, /setPointerCapture\(event\.pointerId\)/);
	assert.match(dialog, /onPointerCancel=\{finishAvatarLogoDrag\}/);
	assert.match(dialog, /onLostPointerCapture=\{finishAvatarLogoDrag\}/);
	assert.match(dialog, /onKeyDown=\{handleAvatarLogoKeyDown\}/);
	assert.match(dialog, /logoTransform: avatarLogoTransform/);
});

test("avatar fit switch uses a plain beige base and white glass selection lens", () => {
	const dialog = source("../components/ui/ImageCropDialog.tsx");
	const modeAttribute = dialog.indexOf("data-avatar-fit-mode={avatarFitMode}");
	const switchStart = dialog.lastIndexOf("<div", modeAttribute);
	const switchEnd = dialog.indexOf("</ToggleButtonGroup>", modeAttribute);
	const switchMarkup = dialog.slice(switchStart, switchEnd);

	assert.notEqual(modeAttribute, -1);
	assert.notEqual(switchStart, -1);
	assert.notEqual(switchEnd, -1);
	assert.match(switchMarkup, /bg-hk-surface-muted/);
	assert.match(switchMarkup, /bg-\[rgba\(255,255,255,0\.72\)\]/);
	assert.match(switchMarkup, /data-avatar-fit-indicator="true"/);
	assert.match(switchMarkup, /backdrop-filter:blur\(10px\)_saturate\(115%\)/);
	assert.match(
		switchMarkup,
		/avatarFitMode === "logo"[\s\S]*?translate-x-full/,
	);
	assert.match(switchMarkup, /motion-reduce:transition-none/);
	assert.doesNotMatch(switchMarkup, /gradient/);
});

test("dialog lets crop gestures inside the panel reach document listeners", () => {
	const dialog = source("../components/ui/Dialog.tsx");

	assert.match(dialog, /panelRef\.current\?\.contains\(target\)/);
	assert.doesNotMatch(
		dialog,
		/function stopNativeOverlayMotionEvent\(event: Event\)[\s\S]*?event\.stopPropagation\(\)/,
	);
});

test("product uploads preserve the full image by default while posts keep crop controls", () => {
	assert.equal(CROP_PRESETS.product.aspect, 4 / 3);
	assert.equal(CROP_PRESETS.product.productEditor, true);
	assert.equal(CROP_PRESETS.product.confirmLabel, "確認上傳");
	assert.equal(CROP_PRESETS.product.sellerCoverEditor, undefined);
	assert.equal(CROP_PRESETS.product.avatarEditor, undefined);
	assert.equal(CROP_PRESETS.post.aspect, 4 / 3);
	assert.equal(CROP_PRESETS.post.postEditor, true);
	assert.equal(CROP_PRESETS.post.productEditor, undefined);
	assert.equal(CROP_PRESETS.post.sellerCoverEditor, undefined);
	assert.equal(CROP_PRESETS.post.avatarEditor, undefined);
});

test("only post uploads opt into the image-first post editor", () => {
	assert.equal(CROP_PRESETS.product.postEditor, undefined);
	assert.equal(CROP_PRESETS.avatar.postEditor, undefined);
	assert.equal(CROP_PRESETS.sellerCover.postEditor, undefined);
	assert.equal(CROP_PRESETS.review.postEditor, undefined);
	assert.equal(CROP_PRESETS.photo.postEditor, undefined);
});

test("product editor maps full-image zoom to 100% and only crops after interaction", () => {
	const dialog = source("../components/ui/ImageCropDialog.tsx");

	assert.match(dialog, /calculateProductImageContainZoom\(nextNaturalAspect\)/);
	assert.match(dialog, /minZoom=\{productContainZoom\}/);
	assert.match(dialog, /objectFit="cover"/);
	assert.match(dialog, /zoomWithScroll/);
	assert.match(dialog, /event\.button === 1/);
	assert.match(dialog, /onTouchRequest=\{\(\) => !processing\}/);
	assert.match(dialog, /transform: productTransform/);
	assert.match(dialog, /createProductImageFile\(current, imageRef\.current/);
	assert.doesNotMatch(
		dialog,
		/<ToggleButton id="cover">填滿畫面<\/ToggleButton>/,
	);
});

test("AI-enhanced product images enter the same full-image-first editor", () => {
	const uploader = source(
		"../features/seller/components/ProductImageUploader.tsx",
	);
	const handlerStart = uploader.indexOf("const handleAiApply");
	const handlerEnd = uploader.indexOf("\n\treturn (", handlerStart);
	const handler = uploader.slice(handlerStart, handlerEnd);

	assert.notEqual(handlerStart, -1);
	assert.notEqual(handlerEnd, -1);
	assert.match(handler, /setAiCropQueue\(\[file\]\)/);
	assert.doesNotMatch(handler, /startUploads\(\[file\]\)/);
	assert.match(
		uploader,
		/<ImageCropDialog[\s\S]*?files=\{aiCropQueue\}[\s\S]*?crop="product"[\s\S]*?onCropped=\{\(file\) => startUploads\(\[file\]\)\}/,
	);
});

test("post uploads offer the feed ratio band, product uploads do not", () => {
	// Posts land on a uniform 4:3 board but the uploader chooses the frame.
	assert.equal(CROP_PRESETS.post.aspectOptions, POST_ASPECT_OPTIONS);
	// Product surfaces are all 4:3 — a picker there would promise a shape the
	// PDP / market card / cart thumb cannot honour.
	assert.equal(CROP_PRESETS.product.aspectOptions, undefined);
	assert.equal(CROP_PRESETS.avatar.aspectOptions, undefined);
	assert.equal(CROP_PRESETS.sellerCover.aspectOptions, undefined);
});

test("post crop viewport and feed-card preview share the canonical 4:3 contract", () => {
	const feedCard = CROP_PRESETS.post.surfaces?.find(
		(surface) => surface.label === "動態牆卡片",
	);
	const dialog = source("../components/ui/ImageCropDialog.tsx");

	assert.equal(POST_FEED_ASPECT, 4 / 3);
	assert.equal(CROP_PRESETS.post.aspect, POST_FEED_ASPECT);
	assert.equal(feedCard?.aspect, POST_FEED_ASPECT);
	assert.equal(feedCard?.fit, "contain");
	assert.match(dialog, /data-post-crop-viewport="true"/);
	assert.match(dialog, /aspectRatio: POST_FEED_ASPECT/);
	assert.match(
		dialog,
		/containAspect: isPostEditor\s*\? POST_FEED_ASPECT\s*:\s*undefined/,
	);
});

test("post ratio band spans the supported feed range, tall to wide", () => {
	const ratios = POST_ASPECT_OPTIONS.map((o) => o.aspect);
	assert.deepEqual(ratios, [4 / 5, 1, 4 / 3, 1.91]);
	// Ordered so the control reads like a slider.
	for (let i = 1; i < ratios.length; i += 1) {
		assert.ok(ratios[i] > ratios[i - 1]);
	}
});

test("closestAspectOption picks the nearest shape in log space", () => {
	const pick = (natural: number) =>
		closestAspectOption(POST_ASPECT_OPTIONS, natural)?.aspect;

	// Exact matches.
	assert.equal(pick(4 / 5), 4 / 5);
	assert.equal(pick(1), 1);
	assert.equal(pick(1.91), 1.91);

	// A 3:4 portrait (0.75) is nearer 4:5 (0.8) than square.
	assert.equal(pick(3 / 4), 4 / 5);
	// 16:9 (1.78) is nearer 1.91 than 4:3.
	assert.equal(pick(16 / 9), 1.91);
	// 3:2 (1.5) sits between 4:3 (1.33) and 1.91 — log distance favours 4:3.
	assert.equal(pick(3 / 2), 4 / 3);
	// Extremes clamp to the ends of the band rather than falling through.
	assert.equal(pick(9 / 16), 4 / 5);
	assert.equal(pick(3), 1.91);

	// A 5:4 (1.25) and its reciprocal 4:5 (0.8) must not both collapse onto the
	// wider option — that is the linear-distance bug this helper avoids.
	assert.equal(pick(0.8), 4 / 5);
	assert.equal(pick(1.25), 4 / 3);
});

test("closestAspectOption rejects unusable input instead of guessing", () => {
	assert.equal(closestAspectOption(POST_ASPECT_OPTIONS, 0), null);
	assert.equal(closestAspectOption(POST_ASPECT_OPTIONS, -1), null);
	assert.equal(closestAspectOption(POST_ASPECT_OPTIONS, Number.NaN), null);
	assert.equal(
		closestAspectOption(POST_ASPECT_OPTIONS, Number.POSITIVE_INFINITY),
		null,
	);
	assert.equal(closestAspectOption([], 1), null);
});

test("custom post ratios accept positive decimal dimensions inside the feed band", () => {
	assert.equal(parseCustomAspectInput("4", "5"), 4 / 5);
	assert.equal(parseCustomAspectInput("1", "1"), 1);
	assert.equal(parseCustomAspectInput("3", "2"), 3 / 2);
	assert.equal(parseCustomAspectInput("1.91", "1"), 1.91);
	// Whitespace is harmless and the bounds are inclusive.
	assert.equal(parseCustomAspectInput(" 4 ", " 5 "), 4 / 5);
});

test("custom post ratios reject missing, non-positive, non-finite, and out-of-band values", () => {
	assert.equal(parseCustomAspectInput("", "1"), null);
	assert.equal(parseCustomAspectInput("1", ""), null);
	assert.equal(parseCustomAspectInput("0", "1"), null);
	assert.equal(parseCustomAspectInput("-1", "1"), null);
	assert.equal(parseCustomAspectInput("hello", "1"), null);
	assert.equal(parseCustomAspectInput("Infinity", "1"), null);
	assert.equal(parseCustomAspectInput("3", "4"), null);
	assert.equal(parseCustomAspectInput("2", "1"), null);
});

test("post ratio choices keep original and custom dimensions behind one compact trigger", () => {
	const dialog = source("../components/ui/ImageCropDialog.tsx");

	assert.match(dialog, /data-post-image-editor="true"/);
	assert.match(dialog, /data-post-ratio-trigger="true"/);
	assert.match(dialog, /aria-expanded=\{postRatioPanelOpen\}/);
	assert.match(dialog, />原圖</);
	assert.match(dialog, />自訂</);
	assert.match(dialog, /postCustomAspectOpen\s*&&/);
	assert.match(dialog, /inputMode="decimal"/);
});

test("changing post ratio invalidates stale crop geometry until Cropper reports again", () => {
	const dialog = source("../components/ui/ImageCropDialog.tsx");
	const pickerStart = dialog.indexOf("const handlePickAspect");
	const pickerEnd = dialog.indexOf("const drawPreviews", pickerStart);
	const picker = dialog.slice(pickerStart, pickerEnd);
	const completeStart = dialog.indexOf("const handleCropComplete");
	const completeEnd = dialog.indexOf("const advance", completeStart);
	const complete = dialog.slice(completeStart, completeEnd);

	assert.notEqual(pickerStart, -1);
	assert.notEqual(pickerEnd, -1);
	assert.match(picker, /areaRef\.current = null/);
	assert.match(picker, /setCropReady\(false\)/);
	assert.notEqual(completeStart, -1);
	assert.notEqual(completeEnd, -1);
	assert.match(complete, /setCropReady\(true\)/);
});

test("post crop failures stay recoverable instead of silently uploading the original", () => {
	const dialog = source("../components/ui/ImageCropDialog.tsx");
	const confirmStart = dialog.indexOf("const handleConfirm");
	const skipStart = dialog.indexOf("const handleSkip", confirmStart);
	const confirm = dialog.slice(confirmStart, skipStart);

	assert.notEqual(confirmStart, -1);
	assert.notEqual(skipStart, -1);
	assert.match(confirm, /if \(isPostEditor\)/);
	assert.match(confirm, /setProcessingError\(/);
	assert.match(confirm, /return;/);
	assert.match(confirm, /catch/);
	// Other legacy crop presets may still fall back after their explicit branch,
	// but a post failure must first stop inside the recoverable error branch.
	assert.match(
		confirm,
		/catch[\s\S]*?if \(isPostEditor\)[\s\S]*?setProcessingError\([\s\S]*?return;[\s\S]*?onCropped\(current, index\)/,
	);
});
