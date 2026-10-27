import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const LAYOUT = "features/product/ProductDetailLayout.tsx";

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

/**
 * The description-image fallback — the branch taken whenever no dimensions are
 * known for a block, which since the `imageInfo` probe was deleted (#339) is
 * every block — must reserve its box before the bytes arrive.
 *
 * It does that with a provisional ratio plus explicit `width`/`height`. The
 * ratio must include `auto`: once the image metadata arrives, the browser swaps
 * to the asset's intrinsic ratio so portrait seller artwork can fill the
 * description width without being letterboxed inside a 4/3 frame.
 *
 * If a real per-image aspect ratio ever becomes available (preset SSOT in
 * `workers/image-gateway`), it replaces the provisional 4/3 ratio. Rebuilding
 * the render-time OSS probe is not that: it cost N× 2.5s timeouts with no
 * cross-request cache, which is why it was removed.
 */
function descriptionFallbackTag(): string {
	const text = source(LAYOUT);
	const marker = text.indexOf("dims 無法解析");
	assert.notEqual(
		marker,
		-1,
		`${LAYOUT} must keep the description-image fallback branch`,
	);
	const start = text.indexOf("<img", marker);
	const end = text.indexOf("/>", start);
	assert.ok(
		start > marker && end > start,
		`${LAYOUT} must render the fallback through an <img> tag`,
	);
	return text.slice(start, end + 2);
}

test("the description-image fallback reserves a provisional box before the image loads", () => {
	const tag = descriptionFallbackTag();

	assert.match(
		tag,
		/aspectRatio:\s*"auto\s+[^";]+"/,
		"the fallback must reserve a provisional ratio and then yield to the " +
			"image's intrinsic ratio after its metadata loads",
	);
	assert.match(
		tag,
		/width=\{\d+\}/,
		"an intrinsic width must stay declared so the ratio has a base to work from",
	);
	assert.match(
		tag,
		/height=\{\d+\}/,
		"an intrinsic height must stay declared alongside the width",
	);
});

test("the fallback fills the description width without cropping or letterboxing", () => {
	const tag = descriptionFallbackTag();

	assert.match(
		tag,
		/width:\s*"100%"/,
		"description artwork must consume the available content width",
	);
	assert.match(
		tag,
		/maxWidth:\s*"100%"/,
		"description artwork must never overflow its content surface",
	);
	assert.match(
		tag,
		/height:\s*"auto"/,
		"the rendered height must follow the image's intrinsic ratio",
	);
	assert.doesNotMatch(
		tag,
		/objectFit:/,
		"object-fit would keep the image inside the provisional 4/3 box instead " +
			"of letting the box adopt the seller artwork's natural ratio",
	);
});

test("the render-time dimension probe stays deleted", () => {
	assert.throws(
		() => source("lib/media/imageInfo.ts"),
		"lib/media/imageInfo.ts must not come back — it probed OSS per image at " +
			"render time with no cross-request cache, so a description with N " +
			"images cost N× the timeout on every cold render (#339)",
	);
});
