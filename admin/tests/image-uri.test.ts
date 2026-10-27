import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { ImageSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import { IMAGE_PRESETS, imageUri } from "../src/utils/image";

test("imageUri 只把 {preset} 換成 preset 名稱", () => {
	const image = create(ImageSchema, {
		uriTemplate: "https://media.example.test/i/{preset}/public/7119/r1/r1.jpg",
	});
	assert.equal(
		imageUri(image, "card"),
		"https://media.example.test/i/card/public/7119/r1/r1.jpg",
	);
});

test("imageUri：外部圖片原樣回傳，沒有圖時回 undefined", () => {
	const external = create(ImageSchema, {
		uriTemplate: "https://lh3.example.test/a/photo.jpg",
	});
	assert.equal(
		imageUri(external, "avatar"),
		"https://lh3.example.test/a/photo.jpg",
	);
	assert.equal(imageUri(undefined, "card"), undefined);
});

test("preset 清單由 image gateway 的 presets.ts 生成，含 motion 與 original", () => {
	assert.ok(IMAGE_PRESETS.includes("motion"));
	assert.ok(IMAGE_PRESETS.includes("original"));
});
