// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const beacon = vi.hoisted(() => ({
	calls: [] as unknown[],
	outcome: "ok" as "ok" | "reject",
}));
vi.mock("@/lib/connect/seller-public", () => ({
	sellerPublicClient: () => ({
		recordProductView: (req: unknown) => {
			beacon.calls.push(req);
			return beacon.outcome === "ok"
				? Promise.resolve({})
				: Promise.reject(new Error("resource_exhausted"));
		},
	}),
}));
vi.mock("@/lib/hooks/useRecordHistory", () => ({
	useRecordHistory: () => ({ enabled: false, record: vi.fn() }),
}));

import { ProductViewTracker } from "./ProductViewTracker";

beforeEach(() => {
	beacon.calls = [];
	beacon.outcome = "ok";
});
afterEach(cleanup);

describe("ProductViewTracker", () => {
	it("sends the view beacon through SellerPublicService once per product", async () => {
		const { rerender } = render(
			createElement(ProductViewTracker, { productId: "p1" }),
		);
		rerender(createElement(ProductViewTracker, { productId: "p1" }));
		expect(beacon.calls).toEqual([{ productId: "p1" }]);
		rerender(createElement(ProductViewTracker, { productId: "p2" }));
		expect(beacon.calls).toEqual([{ productId: "p1" }, { productId: "p2" }]);
	});

	it("swallows a rejected beacon such as resource_exhausted", async () => {
		beacon.outcome = "reject";
		render(createElement(ProductViewTracker, { productId: "p1" }));
		await new Promise((r) => setTimeout(r, 0));
		expect(beacon.calls).toEqual([{ productId: "p1" }]);
	});
});
