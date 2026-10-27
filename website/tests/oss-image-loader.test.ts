import assert from "node:assert/strict";
import { readdirSync } from "node:fs";
import test from "node:test";
import ossImageLoader from "../lib/media/ossImageLoader";

// Post OSS→R2 migration (design book 49 §4.4): per-width resizing is no longer
// expressible — the loader ignores `width`/`quality` and returns the ORIGINAL
// object through the configured media origin (tests/setup/hermetic.mjs pins
// NEXT_PUBLIC_MEDIA_BASE_URL) with NO `x-oss-process` query. next/image dedupes
// the identical URL across srcset widths so the browser fetches one image.
const ORIGINAL = "https://test-media.example.com/i/original";

test("returns the original object unchanged, ignoring width/quality", () => {
	const src = "https://test-media.example.com/public/p/a.jpg";
	const expected = `${ORIGINAL}/public/p/a.jpg`;
	assert.equal(ossImageLoader({ src, width: 640, quality: 75 }), expected);
	assert.equal(ossImageLoader({ src, width: 320 }), expected);
	assert.equal(ossImageLoader({ src, width: 400, quality: 85 }), expected);
});

test("output never contains an x-oss-process derivative query", () => {
	const out = ossImageLoader({
		src: "https://test-media.example.com/public/p/a.jpg",
		width: 640,
	});
	assert.equal(out.includes("x-oss-process"), false);
	assert.equal(out.includes("image/resize"), false);
});

test("leaves a foreign absolute URL untouched (absolute passthrough)", () => {
	const src = "https://images.example.com/public/p/a.jpg";
	assert.equal(ossImageLoader({ src, width: 200 }), src);
});

test("resolves a relative backend path against the configured media origin (no processing)", () => {
	assert.equal(
		ossImageLoader({ src: "public/p/a.jpg", width: 240 }),
		`${ORIGINAL}/public/p/a.jpg`,
	);
});

test("passes local /public assets through unchanged", () => {
	assert.equal(
		ossImageLoader({ src: "/images/hero.png", width: 800 }),
		"/images/hero.png",
	);
	assert.equal(
		ossImageLoader({ src: "/icons/leaf.svg", width: 24 }),
		"/icons/leaf.svg",
	);
	assert.equal(
		ossImageLoader({ src: "/brand/logo-mark.svg", width: 92 }),
		"/brand/logo-mark.svg",
	);
	assert.equal(
		ossImageLoader({ src: "/placeholders/avatar.svg", width: 980 }),
		"/placeholders/avatar.svg",
	);
	assert.equal(
		ossImageLoader({ src: "/brand/logo-mark.svg", width: 60 }),
		"/brand/logo-mark.svg",
	);
});

test("every subdirectory of public/ resolves as a local asset, including future ones", () => {
	const publicDir = new URL("../public", import.meta.url);
	const directories = readdirSync(publicDir, { withFileTypes: true })
		.filter((item) => item.isDirectory())
		.map((item) => item.name);

	// The loader classifies by structure (rooted path minus the reserved backend
	// namespaces), not by a hand-maintained directory list, so a brand-new folder
	// under public/ needs no code edit. The only way this can fail is a folder
	// named after a backend objectKey namespace (public / secure / tmp / i) —
	// which really would be broken at runtime and must be renamed.
	for (const dir of directories) {
		const src = `/${dir}/test-asset.png`;
		const resolved = ossImageLoader({ src, width: 200 });
		assert.equal(
			resolved,
			src,
			`public/${dir} was not served from the page origin (got: ${resolved}). Rename the folder: "${dir}" collides with a backend objectKey namespace.`,
		);
	}

	// Guard the rule itself: a directory that does not exist yet must already be
	// treated as local, otherwise the classification silently degraded back into
	// an allowlist and the next asset folder would 404 on the CDN.
	assert.equal(
		ossImageLoader({ src: "/folder-added-tomorrow/asset.png", width: 200 }),
		"/folder-added-tomorrow/asset.png",
	);
});

test("keeps backend objectKey namespaces remote even with a stray leading slash", () => {
	// Canonical backend keys are slash-less; a rooted variant must still resolve
	// to the CDN, never to this app's origin.
	assert.equal(
		ossImageLoader({ src: "/public/p/a.jpg", width: 240 }),
		`${ORIGINAL}/public/p/a.jpg`,
	);
	assert.equal(
		ossImageLoader({ src: "/secure/id-card/1.jpg", width: 240 }),
		`${ORIGINAL}/secure/id-card/1.jpg`,
	);
	assert.equal(
		ossImageLoader({ src: "/tmp/staged/1.jpg", width: 240 }),
		`${ORIGINAL}/tmp/staged/1.jpg`,
	);
	assert.equal(
		ossImageLoader({ src: "/i/thumb/public/p/a.jpg", width: 240 }),
		`${ORIGINAL}/i/thumb/public/p/a.jpg`,
	);
});

test("never treats a protocol-relative value as a local asset", () => {
	// "//host/x" and "/\host/x" would take the browser off our origin; they must
	// not be mistaken for a rooted public/ path.
	assert.equal(
		ossImageLoader({ src: "//evil.example/x.png", width: 200 }),
		`${ORIGINAL}//evil.example/x.png`,
	);
	assert.equal(
		ossImageLoader({ src: "/\\evil.example/x.png", width: 200 }),
		`${ORIGINAL}/\\evil.example/x.png`,
	);
});

test("leaves an already-absolute URL that carries a query untouched", () => {
	const src = "https://test-media.example.com/public/p/a.jpg?v=99";
	assert.equal(ossImageLoader({ src, width: 640 }), src);
});

test("leaves signed OSS URLs untouched", () => {
	const src =
		"https://test-media.example.com/public/p/a.jpg?Expires=123&OSSAccessKeyId=k&Signature=s";
	assert.equal(ossImageLoader({ src, width: 640 }), src);
});

test("serves an .svg on the media host as its original object", () => {
	const src = "https://test-media.example.com/brand/logo.svg";
	assert.equal(
		ossImageLoader({ src, width: 200 }),
		`${ORIGINAL}/brand/logo.svg`,
	);
});
