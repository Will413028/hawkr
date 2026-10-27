import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
// media.ts reads NEXT_PUBLIC_MEDIA_BASE_URL lazily inside buildImageUrl(), so we
// can toggle the gate per test via process.env; the value is read fresh on each
// call.
import { buildImageUrl, isMediaGatewayEnabled } from "../src/utils/media";

const MEDIA_BASE = "https://test-media.example.com";

const original = process.env.NEXT_PUBLIC_MEDIA_BASE_URL;

function setGate(value: string): void {
	if (value) process.env.NEXT_PUBLIC_MEDIA_BASE_URL = value;
	else delete process.env.NEXT_PUBLIC_MEDIA_BASE_URL;
}

afterEach(() => {
	setGate(original ?? "");
});

// ---- gate OFF (legacy CDN, no processing) ----------------------------------
beforeEach(() => setGate(""));

test("gate OFF: reports disabled", () => {
	setGate("");
	assert.equal(isMediaGatewayEnabled(), false);
});

test("gate OFF: builds no URL for a bare relative key (no built-in host)", () => {
	setGate("");
	assert.equal(buildImageUrl("public/product/1/a.jpg", "card"), "");
	assert.equal(buildImageUrl("/public/x.jpg", "card"), "");
});

test("gate OFF: passes through absolute / data / blob unchanged", () => {
	setGate("");
	assert.equal(
		buildImageUrl("https://test-media.example.com/a.jpg", "card"),
		"https://test-media.example.com/a.jpg",
	);
	assert.equal(
		buildImageUrl("data:image/png;base64,AAA", "card"),
		"data:image/png;base64,AAA",
	);
	assert.equal(buildImageUrl("blob:abc", "card"), "blob:abc");
});

test("gate OFF: does not peel a gateway URL (legacy passthrough)", () => {
	setGate("");
	assert.equal(
		buildImageUrl(`${MEDIA_BASE}/i/original/public/x.jpg`, "card"),
		`${MEDIA_BASE}/i/original/public/x.jpg`,
	);
});

test("gate OFF: empty / whitespace / nullish → empty string", () => {
	setGate("");
	assert.equal(buildImageUrl("", "card"), "");
	assert.equal(buildImageUrl("   ", "card"), "");
	assert.equal(buildImageUrl(null, "card"), "");
	assert.equal(buildImageUrl(undefined, "card"), "");
});

// ---- gate ON (image gateway presets) ---------------------------------------
test("gate ON: reports enabled", () => {
	setGate(MEDIA_BASE);
	assert.equal(isMediaGatewayEnabled(), true);
});

test("gate ON: builds /i/{preset}/{key} for an objectKey", () => {
	setGate(MEDIA_BASE);
	assert.equal(
		buildImageUrl("public/product/1/a.jpg", "card"),
		`${MEDIA_BASE}/i/card/public/product/1/a.jpg`,
	);
});

test("gate ON: defaults to the original preset", () => {
	setGate(MEDIA_BASE);
	assert.equal(
		buildImageUrl("public/x.jpg"),
		`${MEDIA_BASE}/i/original/public/x.jpg`,
	);
});

test("gate ON: strips a single leading slash from the key", () => {
	setGate(MEDIA_BASE);
	assert.equal(
		buildImageUrl("/public/x.jpg", "thumb"),
		`${MEDIA_BASE}/i/thumb/public/x.jpg`,
	);
});

// ---- gate ON: URL → objectKey restoration ----------------------------------
test("gate ON: peels a new-format gateway URL and re-applies the preset", () => {
	setGate(MEDIA_BASE);
	assert.equal(
		buildImageUrl(`${MEDIA_BASE}/i/original/public/product/1/a.jpg`, "card"),
		`${MEDIA_BASE}/i/card/public/product/1/a.jpg`,
	);
});

test("gate ON: peels a media-host URL that has no /i/ segment", () => {
	setGate(MEDIA_BASE);
	assert.equal(
		buildImageUrl("https://test-media.example.com/public/x.jpg", "thumb"),
		`${MEDIA_BASE}/i/thumb/public/x.jpg`,
	);
});

test("gate ON: peels a configured-host URL that already carries a preset", () => {
	setGate(MEDIA_BASE);
	assert.equal(
		buildImageUrl(`${MEDIA_BASE}/i/thumb/public/x.jpg`, "detail"),
		`${MEDIA_BASE}/i/detail/public/x.jpg`,
	);
});

test("gate ON: only the configured host is ours; another media-looking host is foreign", () => {
	setGate(MEDIA_BASE);
	for (const other of [
		"https://media.example.com/i/thumb/public/x.jpg",
		"https://test-media.example.com.evil.test/public/x.jpg",
	]) {
		assert.equal(buildImageUrl(other, "detail"), other);
	}
});

test("gate ON: follows whichever origin is configured", () => {
	setGate("https://cdn.example.org/");
	assert.equal(
		buildImageUrl("public/x.jpg", "card"),
		"https://cdn.example.org/i/card/public/x.jpg",
	);
	assert.equal(
		buildImageUrl("https://cdn.example.org/i/thumb/public/x.jpg", "detail"),
		"https://cdn.example.org/i/detail/public/x.jpg",
	);
	assert.equal(
		buildImageUrl(`${MEDIA_BASE}/public/x.jpg`, "detail"),
		`${MEDIA_BASE}/public/x.jpg`,
	);
});

test("gate ON: passes through foreign-host absolute URLs (Google avatar) untouched", () => {
	setGate(MEDIA_BASE);
	assert.equal(
		buildImageUrl("https://lh3.googleusercontent.com/a/AAcHTt=s96", "avatar"),
		"https://lh3.googleusercontent.com/a/AAcHTt=s96",
	);
});

test("gate ON: empty input → empty string", () => {
	setGate(MEDIA_BASE);
	assert.equal(buildImageUrl("", "card"), "");
	assert.equal(buildImageUrl(null, "card"), "");
});
