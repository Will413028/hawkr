/**
 * hawkr-website#430 D4-M1 — a display URL must never reach the field the
 * backend persists as an imageKey (10 檔 §25 / 49 §6.1).
 *
 * This is not a dead-fallback cleanup. It is a live write-path bug, proven on
 * dev on 2026-08-05:
 *
 *   PUT /app/products/866542657374814537 with
 *     description.blocks[] = [{ type:"image",
 *       imageKey:"https://test-media.example.com/i/original/public/product/…png" }]
 *   → GET the same product returns the block verbatim:
 *       { imageKey: "https://test-media.example.com/i/original/public/…png",
 *         url:       "https://test-media.example.com/i/original/public/…png" }
 *
 * Gateway does NOT normalize this input — `convertDescriptionInputToProto`
 * (gateway/internal/handler/seller_product.go) copies `ObjectKey` straight
 * through, unlike banner which runs `resolveInputObjectKey`
 * (content.go + media_input.go). So the frontend is the only thing standing
 * between the editor and a polluted row.
 *
 * And the editor DOES produce that shape: the Editor.js uploader stores
 * `{ file: { url: <full CDN URL>, stagedToken } }` (DescriptionEditor.tsx) —
 * there is no `file.imageKey` on a fresh upload — and the old
 * `imageKey: d.file?.imageKey ?? displayUrl` therefore sent the URL.
 */

import assert from "node:assert/strict";
import test from "node:test";
import { toObjectKey } from "../lib/media/cdn";
import {
	fromEditorData,
	toEditorData,
} from "../lib/products/descriptionEditorJs";

const KEY = "public/product/2685914682/20260718/f8d18047.png";
const DISPLAY_URL = `https://test-media.example.com/i/original/${KEY}`;

test("toObjectKey peels our media URLs and refuses everything it cannot own", () => {
	assert.equal(toObjectKey(KEY), KEY);
	assert.equal(toObjectKey(DISPLAY_URL), KEY);
	assert.equal(
		toObjectKey(`https://test-media.example.com/i/detail/${KEY}`),
		KEY,
	);
	assert.equal(toObjectKey(` ${DISPLAY_URL} `), KEY);
	assert.equal(toObjectKey(`/${KEY}`), KEY);

	// Not ours / not an object → no imageKey to send.
	assert.equal(toObjectKey("https://lh3.googleusercontent.com/a/x=s96-c"), "");
	assert.equal(toObjectKey("blob:https://web-dev.example.com/uuid"), "");
	assert.equal(toObjectKey("data:image/png;base64,AAAA"), "");
	assert.equal(toObjectKey("/placeholders/post.svg"), "");
	assert.equal(toObjectKey(undefined), "");
	assert.equal(toObjectKey(""), "");
	// An opaque suffix may be a signature; rewriting its path is not safe, and
	// the value is not a key we may store either.
	assert.equal(toObjectKey(`${DISPLAY_URL}?sig=abc`), "");
});

test("a freshly uploaded description image sends a bare imageKey, never the URL", () => {
	// Exactly what DescriptionEditor's uploader hands Editor.js: a display URL
	// plus a stagedToken, and NO imageKey.
	const blocks = fromEditorData({
		blocks: [
			{
				id: "blk-new",
				type: "image",
				data: {
					file: { url: DISPLAY_URL, stagedToken: "staged-abc" },
					caption: "新上傳",
				},
			},
		],
	} as never).blocks;

	assert.equal(blocks.length, 1);
	assert.equal(blocks[0].imageKey, KEY);
	assert.equal(blocks[0].stagedToken, "staged-abc");
	assert.ok(
		!blocks[0].imageKey?.startsWith("http"),
		"a full URL in imageKey is what pollutes the DB — Gateway stores it verbatim",
	);
});

test("an already-polluted row is repaired on the next save instead of round-tripping", () => {
	// A row written before this fix: imageKey holds the URL. `toEditorData`
	// feeds it back into the editor, and the next save must write the key back.
	const editorData = toEditorData({
		version: "1.0",
		blocks: [
			{
				type: "image",
				id: "blk-dirty",
				imageKey: DISPLAY_URL,
				imageUrl: DISPLAY_URL,
			},
		],
	});
	const back = fromEditorData(editorData);
	assert.equal(back.blocks[0].imageKey, KEY);
});

test("a foreign image is dropped rather than written into imageKey", () => {
	const blocks = fromEditorData({
		blocks: [
			{
				id: "blk-foreign",
				type: "image",
				data: { file: { url: "https://evil.example.com/x.png" }, caption: "" },
			},
		],
	} as never).blocks;

	assert.equal(blocks.length, 1);
	assert.equal(blocks[0].imageKey, undefined);
});

test("an unchanged saved block round-trips byte-for-byte", () => {
	const original = {
		version: "1.0",
		blocks: [
			{
				type: "image" as const,
				id: "blk-0",
				assetId: "872530103878698664",
				imageKey: KEY,
				imageUrl: DISPLAY_URL,
				alt: "說明",
			},
		],
	};
	const back = fromEditorData(toEditorData(original));
	assert.equal(back.blocks[0].imageKey, KEY);
	assert.equal(back.blocks[0].assetId, "872530103878698664");
	assert.equal(back.blocks[0].alt, "說明");
});

/* ============================================================
 * 契約守門（backend#2235）：description.blocks[] 的圖片欄名收斂成
 * `imageKey` / `imageUrl`（原 `objectKey` / `url`，gateway
 * `seller_product.go:200`、`product.go:434`）。
 *
 * 這條測試刻意**只餵舊欄名**：如果哪天有人把 `?? block.objectKey` 之類的
 * 退路加回來，它會從「整塊被丟掉」變成「讀得到值」而轉紅。沒有這道，
 * 死退路可以無聲地活很久——它平常不會被走到，直到後端某次改名讓它突然
 * 變成唯一被讀到的分支。
 * ============================================================ */
test("只給舊欄名（objectKey / url）的 image block 讀不出任何東西", () => {
	const back = fromEditorData(
		toEditorData({
			version: "1.0",
			blocks: [
				{
					type: "image",
					id: "blk-legacy",
					// 舊 wire 形狀，逐字重現
					objectKey: KEY,
					url: DISPLAY_URL,
				} as never,
			],
		}),
	);
	// `toEditorData` 讀 `imageUrl`，舊形狀沒有 → isRenderableImage 為 false →
	// 整塊在進編輯器前就被丟掉，不會有半個 block 帶著舊值活下來。
	assert.equal(back.blocks.length, 0);
});
