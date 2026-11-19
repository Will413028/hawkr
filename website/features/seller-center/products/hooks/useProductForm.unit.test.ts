/** @vitest-environment jsdom */

import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	fetchSellerProductEdit: vi.fn(),
	updateProduct: vi.fn(),
	createProduct: vi.fn(),
	updateProductStatus: vi.fn(),
	push: vi.fn(),
}));

vi.mock("@/lib/products/seller-repo", () => ({
	fetchSellerProductEdit: mocks.fetchSellerProductEdit,
	updateProduct: mocks.updateProduct,
	createProduct: mocks.createProduct,
	updateProductStatus: mocks.updateProductStatus,
}));
vi.mock("@/lib/hooks/useSellerProducts", () => ({
	useInvalidateSellerProducts: () => () => Promise.resolve(),
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ push: mocks.push }),
}));

import { useProductForm } from "./useProductForm";

beforeEach(() => {
	for (const m of Object.values(mocks)) m.mockReset();
});
afterEach(cleanup);

describe("useProductForm edit load", () => {
	it("refuses to submit an empty draft after the load throws, and recovers on retry", async () => {
		mocks.fetchSellerProductEdit.mockRejectedValueOnce(new Error("down"));
		const { result } = renderHook(() =>
			useProductForm({ mode: "edit", productId: "9" }),
		);
		await waitFor(() => expect(result.current.loadStatus).toBe("failed"));
		let ok = true;
		await act(async () => {
			ok = await result.current.submit("save");
		});
		expect(ok).toBe(false);
		expect(mocks.updateProduct).not.toHaveBeenCalled();

		mocks.fetchSellerProductEdit.mockResolvedValueOnce({
			id: "9",
			name: "Tea",
			status: 4003,
		});
		await act(async () => {
			await result.current.reload();
		});
		expect(result.current.loadStatus).toBe("ready");
	});

	it("refuses to submit when the product is not found", async () => {
		mocks.fetchSellerProductEdit.mockResolvedValueOnce(null);
		const { result } = renderHook(() =>
			useProductForm({ mode: "edit", productId: "9" }),
		);
		await waitFor(() => expect(result.current.loadStatus).toBe("notFound"));
		let ok = true;
		await act(async () => {
			ok = await result.current.submit("publish");
		});
		expect(ok).toBe(false);
		expect(mocks.updateProduct).not.toHaveBeenCalled();
		expect(mocks.updateProductStatus).not.toHaveBeenCalled();
	});

	it("keeps create mode submittable", () => {
		const { result } = renderHook(() => useProductForm({ mode: "create" }));
		expect(result.current.loadStatus).toBe("ready");
	});
});
