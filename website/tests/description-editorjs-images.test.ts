import assert from "node:assert/strict";
import test, { afterEach, beforeEach } from "node:test";
import type { OutputData } from "@editorjs/editorjs";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import {
	fromEditorData,
	toEditorData,
} from "../lib/products/descriptionEditorJs";
import {
	type ProductDescription,
	toDescriptionWireInput,
} from "../lib/products/seller-repo";

const originalEnv = getEnv();
const MEDIA_ORIGIN = "https://test-media.example.com";
const EXISTING_KEY = "public/product/editor-test/existing.png";
const STAGED_KEY = "public/product/editor-test/staged.png";
const imageUrl = (key: string) => `${MEDIA_ORIGIN}/i/original/${key}`;

beforeEach(() => {
	__setEnvForTest({
		...originalEnv,
		flavor: "dev",
		mediaBaseUrl: MEDIA_ORIGIN,
	});
});
afterEach(() => __setEnvForTest(originalEnv));

const existingDescription = (): ProductDescription => ({
	version: "1.0",
	blocks: [
		{ type: "text", id: "intro", runs: [{ text: "商品介紹" }] },
		{
			type: "image",
			id: "existing-image",
			assetId: "existing-asset",
			imageKey: EXISTING_KEY,
			imageUrl: imageUrl(EXISTING_KEY),
			alt: "既有商品說明圖",
		},
	],
});

test("a saved image remains renderable through canonical history snapshots", () => {
	// DescriptionEditor canonicalizes its initial history with this round trip.
	const snapshot = fromEditorData(toEditorData(existingDescription()));
	assert.equal(snapshot.blocks[1].imageUrl, undefined);
	const restored = toEditorData(snapshot);
	assert.deepEqual(
		restored.blocks.map((block) => block.id),
		["intro", "existing-image"],
	);
	assert.equal(restored.blocks[1].data.file.url, imageUrl(EXISTING_KEY));
	assert.equal(restored.blocks[1].data.file.assetId, "existing-asset");
	assert.equal(restored.blocks[1].data.caption, "既有商品說明圖");
	assert.deepEqual(fromEditorData(restored), snapshot);
});

test("a newly uploaded image retains its staged token across save and reload", () => {
	// The uploader returns a URL and staged token, with no imageKey yet.
	const uploaded: OutputData = {
		blocks: [
			{
				id: "staged-image",
				type: "image",
				data: {
					file: {
						url: imageUrl(STAGED_KEY),
						stagedToken: "staged-token",
					},
					caption: "剛上傳的說明圖",
				},
			},
		],
	};
	const snapshot = fromEditorData(uploaded);
	assert.equal(snapshot.blocks[0].imageKey, STAGED_KEY);
	assert.equal(snapshot.blocks[0].imageUrl, undefined);
	const restored = toEditorData(snapshot);
	assert.equal(restored.blocks[0].type, "image");
	assert.equal(restored.blocks[0].data.file.url, imageUrl(STAGED_KEY));
	assert.equal(restored.blocks[0].data.file.stagedToken, "staged-token");
	assert.equal(restored.blocks[0].data.caption, "剛上傳的說明圖");
	assert.deepEqual(fromEditorData(restored), snapshot);
});

test("undo and redo snapshots preserve image order and identities after deletion", () => {
	const baseline = fromEditorData(toEditorData(existingDescription()));
	const added: ProductDescription = {
		...baseline,
		blocks: [
			...baseline.blocks,
			{
				type: "image",
				id: "staged-image",
				assetId: undefined,
				imageKey: STAGED_KEY,
				stagedToken: "staged-token",
				alt: "新圖",
			},
		],
	};
	const deleted: ProductDescription = {
		...added,
		blocks: added.blocks.filter((block) => block.id !== "existing-image"),
	};
	// These are the saved data fed to applySnapshotToEditor on undo/redo.
	for (const target of [added, deleted, added, baseline, added, deleted]) {
		const restored = toEditorData(target);
		assert.deepEqual(
			restored.blocks.map((block) => block.id),
			target.blocks.map((block) => block.id),
		);
		assert.deepEqual(fromEditorData(restored), target);
	}
});

test("restoring images does not add display-only URLs to submitted descriptions", () => {
	const snapshot = fromEditorData(toEditorData(existingDescription()));
	const restored = fromEditorData(toEditorData(snapshot));
	assert.equal(Object.hasOwn(restored.blocks[1], "imageUrl"), false);
	assert.deepEqual(toDescriptionWireInput(restored), snapshot);
	assert.deepEqual(toDescriptionWireInput(existingDescription()).blocks[1], {
		type: "image",
		id: "existing-image",
		assetId: "existing-asset",
		imageKey: EXISTING_KEY,
		alt: "既有商品說明圖",
	});
});

test("explicit signed URLs stay intact and unfinished images are still excluded", () => {
	const signedUrl = `${imageUrl(EXISTING_KEY)}?sig=opaque`;
	const rendered = toEditorData({
		version: "1.0",
		blocks: [
			{
				type: "image",
				id: "signed",
				imageKey: EXISTING_KEY,
				imageUrl: signedUrl,
			},
			{
				type: "image",
				id: "pending",
				imageKey: EXISTING_KEY,
				imageUrl: "__pending__",
			},
			{ type: "image", id: "error", imageKey: "__error__" },
			{
				type: "image",
				id: "foreign-key",
				imageKey: "https://foreign.example/image.png",
			},
		],
	});
	assert.deepEqual(
		rendered.blocks.map((block) => block.id),
		["signed"],
	);
	assert.equal(rendered.blocks[0].data.file.url, signedUrl);
});
