import { beforeEach, describe, expect, it, vi } from "vitest";
import { fetchSuggestions, searchProducts } from "./repo";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
	apiClient: { public: { get: mocks.get } },
}));
beforeEach(() => {
	vi.clearAllMocks();
});

describe("search transport caller policies", () => {
	it("lets the suggestion BFF opt into strict errors and cancellation without changing soft callers", async () => {
		const controller = new AbortController();
		const outage = new Error("suggestion outage");
		mocks.get.mockRejectedValue(outage);
		await expect(fetchSuggestions("芒果")).resolves.toEqual([]);
		await expect(
			fetchSuggestions("芒果", "203.0.113.8", {
				strict: true,
				signal: controller.signal,
			}),
		).rejects.toBe(outage);
		expect(mocks.get).toHaveBeenLastCalledWith(
			expect.any(String),
			expect.objectContaining({
				visitorIp: "203.0.113.8",
				signal: controller.signal,
				throwOnHttpError: true,
				throwOnEnvelopeError: true,
			}),
		);
	});

	it("normalizes successful strict suggestions and preserves a genuinely empty response", async () => {
		mocks.get.mockResolvedValueOnce({
			data: [" 芒果 ", "芒果", { keyword: "青芒果", category: "水果" }],
		});
		await expect(
			fetchSuggestions("芒果", null, { strict: true }),
		).resolves.toEqual([
			{ keyword: "芒果" },
			{ keyword: "青芒果", category: "水果" },
		]);
		mocks.get.mockResolvedValueOnce({ data: [] });
		await expect(
			fetchSuggestions("芒果", null, { strict: true }),
		).resolves.toEqual([]);
	});

	it("does not silently accept a missing strict suggestion payload", async () => {
		mocks.get.mockResolvedValue({ data: undefined });
		await expect(
			fetchSuggestions("芒果", null, { strict: true }),
		).rejects.toThrow("public payload missing");
	});

	it.each([
		searchProducts,
	])("passes strict errors and cancellation for client callers", async (search) => {
		const controller = new AbortController();
		const outage = new Error("service unavailable");
		mocks.get.mockRejectedValue(outage);
		await expect(
			search({ q: "芒果", signal: controller.signal, strict: true }),
		).rejects.toBe(outage);
		expect(mocks.get).toHaveBeenCalledWith(
			expect.any(String),
			expect.objectContaining({
				signal: controller.signal,
				throwOnHttpError: true,
				throwOnEnvelopeError: true,
			}),
		);
	});

	it.each([
		searchProducts,
	])("retains the existing fallback for callers that did not opt into strict mode", async (search) => {
		mocks.get.mockRejectedValue(new Error("service unavailable"));
		await expect(search({ q: "芒果" })).resolves.toEqual({
			records: [],
			total: 0,
		});
	});

	it("retains the backend product total rather than inferring it from one result page", async () => {
		mocks.get.mockResolvedValue({
			data: {
				records: [{ id: "fixture-product", name: "測試商品" }],
				total: "27",
			},
		});
		await expect(
			searchProducts({ q: "芒果", strict: true }),
		).resolves.toMatchObject({ total: 27 });
	});
});
