import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const uiKit = readFileSync(
	new URL("../styles/components/ui-kit.css", import.meta.url),
	"utf8",
);
const storefrontPreview = readFileSync(
	new URL("../styles/features/storefront-product-preview.css", import.meta.url),
	"utf8",
);
const imageCropDialog = readFileSync(
	new URL("../components/ui/ImageCropDialog.tsx", import.meta.url),
	"utf8",
);

test("shared overlays own symmetric placement-aware motion", () => {
	assert.match(uiKit, /\[data-hk-overlay-root\]\s*\{[\s\S]*?opacity: 0;/);
	assert.match(
		uiKit,
		/\[data-hk-overlay-root\]\[data-hk-overlay-state="open"\][\s\S]*?opacity: 1;/,
	);
	assert.match(
		uiKit,
		/\[data-hk-overlay-root\]\[data-hk-overlay-state="closed"\][\s\S]*?--hk-overlay-closed-transform/,
	);
	for (const motion of [
		"dialog-center",
		"dialog-top",
		"dialog-bottom",
		"sheet-bottom",
		"sheet-right",
		"sheet-left",
	]) {
		assert.match(uiKit, new RegExp(`data-hk-overlay-motion="${motion}"`));
	}
	assert.match(uiKit, /--hk-overlay-duration: var\(--hkui-dur-slow\)/);
});

test("reduced motion keeps only a short opacity acknowledgement", () => {
	const overlayMotionStart = uiKit.indexOf("Shared Dialog / Sheet motion");
	const reducedMotionStart = uiKit.indexOf(
		"@media (prefers-reduced-motion: reduce)",
		overlayMotionStart,
	);
	const reducedMotion = uiKit.slice(
		reducedMotionStart,
		uiKit.indexOf(".hk-heroui-brand-scope", reducedMotionStart),
	);
	assert.ok(overlayMotionStart >= 0);
	assert.ok(reducedMotionStart > overlayMotionStart);
	assert.match(reducedMotion, /transition: opacity var\(--hkui-dur-fast\)/);
	assert.match(reducedMotion, /transform: none;/);
	assert.match(reducedMotion, /transition: none;/);
});

test("storefront preview no longer overrides shared enter and exit motion", () => {
	assert.doesNotMatch(
		storefrontPreview,
		/@keyframes storefront-product-preview/,
	);
	assert.doesNotMatch(
		storefrontPreview,
		/animation:\s*storefront-product-preview/,
	);
});

test("image crop keeps the shared Dialog mounted long enough to exit", () => {
	assert.match(imageCropDialog, /<Dialog\s+open=\{open\}/);
	assert.doesNotMatch(imageCropDialog, /if \(!open\) return null/);
	assert.match(
		imageCropDialog,
		/const renderedFiles = open \? files : displayFiles/,
	);
	assert.match(imageCropDialog, /IMAGE_CROP_EXIT_CONTENT_MS = 450/);
	assert.match(imageCropDialog, /if \(files\.length > 0\) setIndex\(0\)/);
});
