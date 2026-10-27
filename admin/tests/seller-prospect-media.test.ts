import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { UpdateProspectRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import {
	normalizeSourceUrl,
	PROSPECT_IMAGE_MAX_BYTES,
	prospectImageUpdate,
	validateProspectImage,
} from "../src/lib/seller-prospect-media";

test("image input accepts exactly supported raster types and rejects empty, oversized and SVG files", () => {
	for (const type of ["image/jpeg", "image/png", "image/webp"]) {
		assert.equal(
			validateProspectImage({ type, size: PROSPECT_IMAGE_MAX_BYTES }),
			null,
		);
	}
	for (const file of [
		{ type: "image/svg+xml", size: 100 },
		{ type: "image/gif", size: 100 },
		{ type: "image/png", size: 0 },
		{ type: "image/png", size: PROSPECT_IMAGE_MAX_BYTES + 1 },
	])
		assert.ok(validateProspectImage(file));
});

test("來源網址規則同伺服器：去空白後是 https、有主機、最長 2048 bytes；不另擋內網位址", () => {
	for (const value of [
		"",
		"   ",
		"javascript:alert(1)",
		"data:image/png,x",
		"http://example.org",
		"assets/logo.png",
		"https://",
		`https://example.org/${"a".repeat(2048)}`,
	]) {
		assert.equal(normalizeSourceUrl(value), null, value);
	}
	assert.equal(
		normalizeSourceUrl(" https://example.org/brand "),
		"https://example.org/brand",
	);
	assert.equal(
		normalizeSourceUrl("https://localhost/image"),
		"https://localhost/image",
	);
	const longest = `https://example.org/${"a".repeat(2048 - "https://example.org/".length)}`;
	assert.equal(normalizeSourceUrl(longest), longest);
});

test('換圖請求：mask 只列這一張圖、帶共用的 etag，不送 "*"', () => {
	for (const [kind, path, field] of [
		["logo", "logo_upload", "logoUpload"],
		["cover", "cover_upload", "coverUpload"],
	] as const) {
		const request = create(
			UpdateProspectRequestSchema,
			prospectImageUpdate({
				id: "7",
				etag: "e1",
				kind,
				stagedToken: "tok",
				sourceUrl: "",
			}),
		);
		assert.deepEqual(request.updateMask?.paths, [path]);
		assert.equal(request.prospect?.id, "7");
		assert.equal(request.prospect?.etag, "e1");
		assert.equal(request.prospect?.[field]?.stagedToken, "tok");
		assert.equal(request.prospect?.[field]?.sourceUrl, "");
	}
});

test("prospect 上傳走共用的 uploadImageToMedia，沒有專用的 gateway 路徑與瀏覽器憑證選項", async () => {
	const source = await readFile(
		"src/services/seller-prospect-media.ts",
		"utf8",
	);
	assert.match(source, /uploadImageToMedia/u);
	assert.match(source, /validateProspectImage/u);
	assert.doesNotMatch(source, /seller_prospects|localStorage|fetch\(/u);
});
