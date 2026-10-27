// @vitest-environment jsdom

import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type {
	ProductCertificationApplication,
	ProductCreateResult,
	SellerProductDetail,
} from "@/lib/products/seller-repo";
import productDetailFixture from "../adapters/__tests__/fixtures/app-products-866542657374814537.json";
import { emptyDraft, validateDraft } from "../adapters/productFormAdapter";
import { useProductForm } from "./useProductForm";

const mocks = vi.hoisted(() => ({
	createProduct: vi.fn(),
	fetchProductCertificationApplications: vi.fn(),
	fetchSellerProductEdit: vi.fn(),
	submitProductCertificationApplication: vi.fn(),
	updateProduct: vi.fn(),
	updateProductStatus: vi.fn(),
	withdrawProductCertificationApplication: vi.fn(),
	resolveStagedImages: vi.fn(),
	invalidateSellerProducts: vi.fn(),
	router: { push: vi.fn() },
}));

vi.mock("@/lib/products/seller-repo", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/products/seller-repo")>()),
	createProduct: mocks.createProduct,
	fetchProductCertificationApplications:
		mocks.fetchProductCertificationApplications,
	fetchSellerProductEdit: mocks.fetchSellerProductEdit,
	submitProductCertificationApplication:
		mocks.submitProductCertificationApplication,
	updateProduct: mocks.updateProduct,
	updateProductStatus: mocks.updateProductStatus,
	withdrawProductCertificationApplication:
		mocks.withdrawProductCertificationApplication,
}));

vi.mock("@/lib/media/resolve-staged-images", async (importOriginal) => ({
	...(await importOriginal<
		typeof import("@/lib/media/resolve-staged-images")
	>()),
	resolveStagedImages: mocks.resolveStagedImages,
}));

vi.mock("@/lib/hooks/useSellerProducts", () => ({
	useInvalidateSellerProducts: () => mocks.invalidateSellerProducts,
}));

vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => mocks.router,
}));

const productId = "866542657374814537";

function application(
	certificationType = "3701",
	certificateNo = "ORG-123",
): ProductCertificationApplication {
	return {
		id: `application-${certificationType}`,
		productId,
		certificationType,
		certificateNo,
		status: "SUBMITTED",
		version: "1",
	};
}

function validDraft(): SellerProductDetail {
	return {
		...emptyDraft(),
		name: "測試商品",
		category_id: 829,
		images: [
			{ url: "https://media.example/product.jpg", stagedToken: "photo" },
		],
		skus: [
			{
				id: "",
				price: "100",
				stock: 10,
				unit: "3301",
				weight: "1",
				length_cm: "10",
				width_cm: "10",
				height_cm: "10",
			},
		],
	};
}

function renderCreateForm() {
	const hook = renderHook(() => useProductForm({ mode: "create" }));
	act(() => hook.result.current.update(validDraft()));
	return hook;
}

beforeEach(() => {
	vi.resetAllMocks();
	mocks.createProduct.mockResolvedValue({ id: productId });
	mocks.fetchProductCertificationApplications.mockResolvedValue([]);
	mocks.submitProductCertificationApplication.mockImplementation(
		async (_id, input) =>
			application(input.certificationType, input.certificateNo),
	);
	mocks.resolveStagedImages.mockResolvedValue([{ stagedToken: "photo" }]);
	mocks.invalidateSellerProducts.mockResolvedValue(undefined);
	mocks.updateProduct.mockResolvedValue({});
	mocks.updateProductStatus.mockResolvedValue({});
});

afterEach(cleanup);

describe("useProductForm create certification applications", () => {
	it("creates before submitting selected applications and keeps claims out of the product payload", async () => {
		const { result } = renderCreateForm();
		expect(validateDraft(result.current.draft)).toEqual([]);
		act(() => {
			result.current.updateCertificationDraft("3701", "  ORG-123  ");
			result.current.updateCertificationDraft("3702", "TAP-456");
		});
		expect(result.current.draft.certifications).toEqual([]);
		expect(mocks.submitProductCertificationApplication).not.toHaveBeenCalled();

		let succeeded = false;
		await act(async () => {
			succeeded = await result.current.submit("publish");
		});

		expect(succeeded).toBe(true);
		expect(mocks.createProduct).toHaveBeenCalledExactlyOnceWith(
			expect.objectContaining({
				certifications: [],
				images: [{ stagedToken: "photo" }],
			}),
		);
		expect(mocks.submitProductCertificationApplication).toHaveBeenCalledTimes(
			2,
		);
		expect(mocks.submitProductCertificationApplication).toHaveBeenNthCalledWith(
			1,
			productId,
			expect.objectContaining({
				certificationType: "3701",
				certificateNo: "ORG-123",
			}),
		);
		expect(mocks.submitProductCertificationApplication).toHaveBeenNthCalledWith(
			2,
			productId,
			expect.objectContaining({
				certificationType: "3702",
				certificateNo: "TAP-456",
			}),
		);
		expect(mocks.createProduct.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.submitProductCertificationApplication.mock.invocationCallOrder[0],
		);
		expect(result.current.createdProduct).toEqual({ id: productId });
		expect(result.current.createdCertificationCount).toBe(2);
		expect(result.current.certificationDrafts).toEqual([]);
		expect(mocks.router.push).toHaveBeenCalledWith(
			`/seller-center/products/${productId}/edit`,
		);
	});

	it("blocks an empty selected certificate before creating and allows deselection", async () => {
		const { result } = renderCreateForm();
		act(() => result.current.updateCertificationDraft("3701", "   "));

		await act(async () => {
			expect(await result.current.submit("publish")).toBe(false);
		});
		expect(result.current.certErrors["3701"]).toBeTruthy();
		expect(mocks.createProduct).not.toHaveBeenCalled();
		expect(mocks.resolveStagedImages).not.toHaveBeenCalled();
		expect(mocks.submitProductCertificationApplication).not.toHaveBeenCalled();

		act(() => result.current.updateCertificationDraft("3701", null));
		expect(result.current.certificationDrafts).toEqual([]);
		expect(result.current.certErrors["3701"]).toBeFalsy();
		await act(async () => {
			expect(await result.current.submit("publish")).toBe(true);
		});
		expect(mocks.createProduct).toHaveBeenCalledTimes(1);
		expect(mocks.submitProductCertificationApplication).not.toHaveBeenCalled();
		expect(mocks.router.push).toHaveBeenCalledWith("/seller-center/products");
	});

	it("does not submit an application from an unselected public projection", async () => {
		const { result } = renderCreateForm();
		act(() =>
			result.current.update({
				certifications: [{ code: "3701", certNo: "legacy-claim" }],
			}),
		);

		await act(async () => {
			expect(await result.current.submit("publish")).toBe(true);
		});

		expect(mocks.createProduct).toHaveBeenCalledWith(
			expect.objectContaining({ certifications: [] }),
		);
		expect(mocks.submitProductCertificationApplication).not.toHaveBeenCalled();
		expect(mocks.router.push).toHaveBeenCalledWith("/seller-center/products");
	});

	it("retains application drafts when product creation fails", async () => {
		mocks.createProduct.mockRejectedValueOnce(new Error("建立商品失敗"));
		const { result } = renderCreateForm();
		act(() => result.current.updateCertificationDraft("3701", "ORG-123"));

		await act(async () => {
			expect(await result.current.submit("publish")).toBe(false);
		});

		expect(result.current.createdProduct).toBeNull();
		expect(result.current.certificationDrafts).toEqual([
			{ code: "3701", certNo: "ORG-123" },
		]);
		expect(result.current.errors).toContain("建立商品失敗");
		expect(result.current.submitting).toBe(false);
		expect(mocks.submitProductCertificationApplication).not.toHaveBeenCalled();
		expect(mocks.router.push).not.toHaveBeenCalled();
	});

	it("retries only failed applications with the same idempotency key without recreating or restaging", async () => {
		mocks.submitProductCertificationApplication
			.mockResolvedValueOnce(application())
			.mockRejectedValueOnce(new Error("TAP 申請暫時失敗"));
		const { result } = renderCreateForm();
		act(() => {
			result.current.updateCertificationDraft("3701", "ORG-123");
			result.current.updateCertificationDraft("3702", "TAP-456");
		});

		await act(async () => {
			expect(await result.current.submit("publish")).toBe(false);
		});

		expect(result.current.createdProduct).toEqual({ id: productId });
		expect(result.current.createdCertificationCount).toBe(1);
		expect(result.current.certificationDrafts).toEqual([
			{ code: "3702", certNo: "TAP-456" },
		]);
		expect(result.current.certErrors["3702"]).toContain("TAP 申請暫時失敗");
		expect(result.current.submitting).toBe(false);
		expect(mocks.router.push).not.toHaveBeenCalled();
		const failedInput =
			mocks.submitProductCertificationApplication.mock.calls[1][1];
		expect(failedInput.idempotencyKey).toEqual(expect.any(String));
		expect(failedInput.idempotencyKey.length).toBeGreaterThan(0);

		await act(async () => {
			expect(await result.current.submit("publish")).toBe(true);
		});

		expect(mocks.createProduct).toHaveBeenCalledTimes(1);
		expect(mocks.resolveStagedImages).toHaveBeenCalledTimes(1);
		expect(mocks.submitProductCertificationApplication).toHaveBeenCalledTimes(
			3,
		);
		expect(
			mocks.submitProductCertificationApplication,
		).toHaveBeenLastCalledWith(productId, failedInput);
		expect(result.current.createdCertificationCount).toBe(2);
		expect(result.current.certificationDrafts).toEqual([]);
		expect(result.current.certErrors["3702"]).toBeFalsy();
		expect(mocks.router.push).toHaveBeenCalledWith(
			`/seller-center/products/${productId}/edit`,
		);
	});

	it("allows draft corrections before create but preserves the original application after an uncertain response", async () => {
		mocks.submitProductCertificationApplication.mockRejectedValueOnce(
			new Error("連線中斷"),
		);
		const { result } = renderCreateForm();
		act(() => result.current.updateCertificationDraft("3701", "wrong-number"));
		act(() => result.current.updateCertificationDraft("3701", "ORG-123"));
		expect(result.current.certificationDrafts).toEqual([
			{ code: "3701", certNo: "ORG-123" },
		]);
		await act(async () => {
			expect(await result.current.submit("publish")).toBe(false);
		});
		const originalInput =
			mocks.submitProductCertificationApplication.mock.calls[0][1];
		expect(originalInput.certificateNo).toBe("ORG-123");
		expect(originalInput.idempotencyKey).toEqual(expect.any(String));

		// A network error does not prove the server rejected the application.
		// Local edits or deselection must not masquerade as a remote withdrawal.
		act(() => {
			result.current.updateCertificationDraft("3701", "replacement-number");
			result.current.updateCertificationDraft("3701", null);
			result.current.updateCertificationDraft("3702", "TAP-456");
		});
		expect(result.current.certificationDrafts).toEqual([
			{ code: "3701", certNo: "ORG-123" },
		]);
		await act(async () => {
			expect(await result.current.submit("publish")).toBe(true);
		});

		expect(mocks.submitProductCertificationApplication).toHaveBeenCalledTimes(
			2,
		);
		expect(
			mocks.submitProductCertificationApplication,
		).toHaveBeenLastCalledWith(productId, originalInput);
		expect(mocks.createProduct).toHaveBeenCalledTimes(1);
		expect(mocks.resolveStagedImages).toHaveBeenCalledTimes(1);
	});

	it("holds a successful create receipt without an ID and never creates a duplicate on retry", async () => {
		mocks.createProduct.mockResolvedValueOnce({});
		const { result } = renderCreateForm();
		act(() => result.current.updateCertificationDraft("3701", "ORG-123"));

		await act(async () => {
			expect(await result.current.submit("publish")).toBe(false);
		});
		expect(result.current.createdProduct).toEqual({});
		expect(result.current.errors.join(" ")).toMatch(/商品.*(編號|ID|id)/);
		expect(result.current.certificationDrafts).toEqual([
			{ code: "3701", certNo: "ORG-123" },
		]);

		await act(async () => {
			expect(await result.current.submit("publish")).toBe(false);
		});
		expect(mocks.createProduct).toHaveBeenCalledTimes(1);
		expect(mocks.resolveStagedImages).toHaveBeenCalledTimes(1);
		expect(mocks.submitProductCertificationApplication).not.toHaveBeenCalled();
		expect(mocks.router.push).not.toHaveBeenCalled();
	});

	it("guards overlapping submissions before React can render the disabled state", async () => {
		let finishCreate: (value: ProductCreateResult) => void = () => {
			throw new Error("Create request has not started");
		};
		mocks.createProduct.mockImplementationOnce(
			() =>
				new Promise<ProductCreateResult>((resolve) => {
					finishCreate = resolve;
				}),
		);
		const { result } = renderCreateForm();
		act(() => result.current.updateCertificationDraft("3701", "ORG-123"));
		let firstSubmit: Promise<boolean> = Promise.resolve(false);
		let duplicateSubmit: Promise<boolean> = Promise.resolve(true);
		act(() => {
			const submit = result.current.submit;
			firstSubmit = submit("publish");
			duplicateSubmit = submit("publish");
		});
		await waitFor(() => expect(mocks.createProduct).toHaveBeenCalledTimes(1));
		expect(mocks.submitProductCertificationApplication).not.toHaveBeenCalled();
		expect(await duplicateSubmit).toBe(false);

		await act(async () => {
			finishCreate({ id: productId });
			expect(await firstSubmit).toBe(true);
		});
		expect(mocks.createProduct).toHaveBeenCalledTimes(1);
		expect(mocks.resolveStagedImages).toHaveBeenCalledTimes(1);
		expect(mocks.submitProductCertificationApplication).toHaveBeenCalledTimes(
			1,
		);
	});
});

describe("useProductForm edit certification compatibility", () => {
	it("continues submitting edit applications directly and preserves the existing publish pipeline", async () => {
		mocks.fetchSellerProductEdit.mockResolvedValue(productDetailFixture.data);
		const { result } = renderHook(() =>
			useProductForm({ mode: "edit", productId }),
		);
		await waitFor(() => expect(result.current.loading).toBe(false));
		expect(validateDraft(result.current.draft)).toEqual([]);

		await act(async () => {
			await result.current.submitCert("3701", "  ORG-123  ");
		});
		expect(mocks.submitProductCertificationApplication).toHaveBeenCalledWith(
			productId,
			expect.objectContaining({
				certificationType: "3701",
				certificateNo: "ORG-123",
			}),
		);
		expect(mocks.fetchProductCertificationApplications).toHaveBeenCalledTimes(
			2,
		);
		expect(mocks.createProduct).not.toHaveBeenCalled();

		await act(async () => {
			expect(await result.current.submit("publish")).toBe(true);
		});
		expect(mocks.updateProduct).toHaveBeenCalledTimes(1);
		expect(mocks.updateProduct.mock.calls[0][0]).toBe(productId);
		expect(mocks.updateProduct.mock.calls[0][1]).not.toHaveProperty("images");
		expect(mocks.resolveStagedImages).not.toHaveBeenCalled();
		expect(mocks.updateProductStatus).toHaveBeenCalledWith(productId, "4004");
		expect(mocks.submitProductCertificationApplication).toHaveBeenCalledTimes(
			1,
		);
		expect(mocks.router.push).toHaveBeenCalledWith("/seller-center/products");
	});
});
