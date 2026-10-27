import { create } from "@bufbuild/protobuf";
import { ImageSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import { describe, expect, it } from "vitest";
import { IMAGE_PRESETS, imageUri } from "../image";

describe("imageUri", () => {
	it("只把 {preset} 換成 preset 名稱", () => {
		const image = create(ImageSchema, {
			uriTemplate:
				"https://media.example.test/i/{preset}/public/7109/b1/p1.jpg",
		});
		expect(imageUri(image, "banner")).toBe(
			"https://media.example.test/i/banner/public/7109/b1/p1.jpg",
		);
	});

	it("外部圖片的樣板沒有變數，原樣回傳", () => {
		const image = create(ImageSchema, {
			uriTemplate: "https://lh3.example.test/a/photo.jpg",
		});
		expect(imageUri(image, "avatar")).toBe(
			"https://lh3.example.test/a/photo.jpg",
		);
	});

	it("沒有圖時回 undefined", () => {
		expect(imageUri(undefined, "card")).toBeUndefined();
	});

	it("preset 清單由 image gateway 的 presets.ts 生成", () => {
		expect(IMAGE_PRESETS).toContain("original");
		expect(IMAGE_PRESETS).toContain("motion");
	});
});
