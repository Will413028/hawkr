import { beforeEach, describe, expect, it, vi } from "vitest";
import { GET as getProducts } from "./products/route";
import { GET as getSuggestions } from "./suggest/route";

const mocks = vi.hoisted(() => ({ get: vi.fn(), category: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
	apiClient: { public: { get: mocks.get } },
}));
vi.mock("@/lib/api", () => ({ fetchProducts: mocks.category }));

function request(path: string, query = "q=芒果&page_size=6") {
	return new Request(`https://hawkr.test/api/public/search/${path}?${query}`, {
		headers: { "cf-connecting-ip": "203.0.113.8" },
	});
}
beforeEach(() => {
	vi.resetAllMocks();
	mocks.category.mockResolvedValue({ records: [] });
});

describe("public search BFF failure contract", () => {
	it.each([
		["products", getProducts],
		["suggest", getSuggestions],
	] as const)("returns a retryable non-cacheable error for %s instead of a successful empty result", async (path, get) => {
		mocks.get.mockRejectedValue(new Error("private upstream diagnostic"));
		const req = request(path);
		const response = await get(req);
		expect(response.status).toBe(502);
		expect(response.headers.get("cache-control")).toBe("no-store");
		const body = await response.json();
		expect(body).not.toHaveProperty("records");
		expect(body).not.toHaveProperty("suggestions");
		expect(JSON.stringify(body)).not.toContain("private upstream diagnostic");
		expect(mocks.get).toHaveBeenCalledWith(
			expect.any(String),
			expect.objectContaining({
				visitorIp: "203.0.113.8",
				signal: req.signal,
				throwOnHttpError: true,
				throwOnEnvelopeError: true,
			}),
		);
		expect(mocks.category).not.toHaveBeenCalled();
	});

	it("keeps authoritative keyword hits when the optional category fill fails", async () => {
		const records = [{ id: "mango", name: "芒果", categoryId: "fruit" }];
		mocks.get.mockResolvedValue({ data: { records, total: "21" } });
		mocks.category.mockRejectedValue(new Error("optional fill unavailable"));
		const req = request("products");
		const response = await getProducts(req);
		expect(response.status).toBe(200);
		expect(await response.json()).toEqual({ records, total: 21 });
		expect(mocks.category).toHaveBeenCalledTimes(1);
		expect(mocks.category).toHaveBeenCalledWith(
			"",
			1,
			8,
			"fruit",
			undefined,
			expect.objectContaining({ signal: req.signal }),
		);
	});

	it("does not start an optional category request after the visitor has canceled", async () => {
		const controller = new AbortController();
		const req = new Request(
			"https://hawkr.test/api/public/search/products?q=芒果",
			{
				signal: controller.signal,
			},
		);
		mocks.get.mockImplementation(async () => {
			controller.abort();
			return {
				data: {
					records: [{ id: "mango", name: "芒果", categoryId: "fruit" }],
					total: "1",
				},
			};
		});
		await getProducts(req);
		expect(mocks.category).not.toHaveBeenCalled();
	});

	it("preserves same-category fill and the keyword total for successful related-product requests", async () => {
		mocks.get.mockResolvedValue({
			data: {
				records: [{ id: "mango", name: "芒果", categoryId: "fruit" }],
				total: "1",
			},
		});
		mocks.category.mockResolvedValue({
			records: [
				{ id: "mango", name: "芒果" },
				{ id: "pineapple", name: "鳳梨", imageKey: "fruit/pineapple.jpg" },
			],
		});
		const response = await getProducts(request("products"));
		const body = await response.json();
		expect(response.status).toBe(200);
		expect(body.records.map((record: { id: string }) => record.id)).toEqual([
			"mango",
			"pineapple",
		]);
		expect(body.total).toBe(1);
	});

	it.each([
		[
			"products",
			getProducts,
			{ data: { records: [], total: "0" } },
			{ records: [], total: 0 },
		],
		["suggest", getSuggestions, { data: [] }, { suggestions: [] }],
	] as const)("only returns an empty %s success when the upstream succeeds", async (path, get, upstream, expected) => {
		mocks.get.mockResolvedValue(upstream);
		const response = await get(request(path));
		expect(response.status).toBe(200);
		expect(await response.json()).toEqual(expected);
	});

	it.each([
		["products", getProducts],
		["suggest", getSuggestions],
	] as const)("does not make an upstream %s request for an empty keyword", async (path, get) => {
		const response = await get(request(path, "q="));
		expect(response.status).toBe(200);
		expect(mocks.get).not.toHaveBeenCalled();
	});
});
