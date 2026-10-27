// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { ImageSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/ImageFallback", () => ({
	default: ({ label }: { label: string }) =>
		React.createElement("span", { "data-testid": "fallback" }, label),
}));

import AppImage from "./AppImage";

afterEach(cleanup);

describe("AppImage 的 API 圖片（image）", () => {
	it("只把 {preset} 換掉，不改寫成別的 preset", () => {
		const image = create(ImageSchema, {
			uriTemplate:
				"https://media.example.test/i/{preset}/public/7119/r1/r1.jpg",
		});
		render(
			React.createElement(AppImage, {
				image,
				preset: "card",
				alt: "番茄炒蛋",
				width: 360,
				height: 270,
			}),
		);
		expect(screen.getByAltText("番茄炒蛋").getAttribute("src")).toBe(
			"https://media.example.test/i/card/public/7119/r1/r1.jpg",
		);
	});

	it("沒有圖時顯示沒有圖片", () => {
		render(
			React.createElement(AppImage, {
				image: undefined,
				preset: "card",
				alt: "番茄炒蛋",
				width: 360,
				height: 270,
			}),
		);
		expect(screen.getByTestId("fallback").textContent).toBe("尚無圖片");
	});
});
