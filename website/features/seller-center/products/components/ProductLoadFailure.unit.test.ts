/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/Link", () => ({
	default: ({ href, children }: { href: string; children: React.ReactNode }) =>
		React.createElement("a", { href }, children),
}));

import ProductLoadFailure from "./ProductLoadFailure";

afterEach(cleanup);

describe("ProductLoadFailure", () => {
	it("notFound uses seller-center copy with a link back to the list", () => {
		render(
			React.createElement(ProductLoadFailure, {
				status: "notFound",
				onRetry: vi.fn(),
			}),
		);
		expect(screen.getByText("找不到這個商品")).toBeTruthy();
		expect(
			screen.getByText("這個商品可能已被刪除，或不屬於你的店家。"),
		).toBeTruthy();
		expect(
			screen.getByRole("link", { name: "回到商品列表" }).getAttribute("href"),
		).toBe("/seller-center/products");
		expect(screen.queryByText("這頁不在貨架上")).toBeNull();
		expect(screen.queryByText(/商品已經下架/)).toBeNull();
	});

	it("failed keeps the retry button", () => {
		const onRetry = vi.fn();
		render(
			React.createElement(ProductLoadFailure, { status: "failed", onRetry }),
		);
		screen.getByRole("button", { name: "重新載入" }).click();
		expect(onRetry).toHaveBeenCalledTimes(1);
	});
});
