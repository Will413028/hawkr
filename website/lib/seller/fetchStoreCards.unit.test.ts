import { create } from "@bufbuild/protobuf";
import { StoreCardSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	batchGetStores: vi.fn(),
}));

vi.mock("@/lib/connect/seller-public", () => ({
	sellerPublicClient: () => ({ batchGetStores: mocks.batchGetStores }),
}));

import { fetchStoreCards, normalizeStoreIds } from "./fetchStoreCards";

const idsOf = (n: number) => Array.from({ length: n }, (_, i) => `s${i}`);

beforeEach(() => {
	mocks.batchGetStores.mockReset();
	// 回傳每個請求的 id 對應的卡片，但 "missing" 沒有卡片。
	mocks.batchGetStores.mockImplementation(
		async ({ ids }: { ids: string[] }) => ({
			stores: ids
				.filter((id) => id !== "missing")
				.map((id) => create(StoreCardSchema, { id, name: id })),
		}),
	);
});

describe("normalizeStoreIds", () => {
	it("sorts, dedupes and drops blanks", () => {
		expect(normalizeStoreIds(["b", " a ", "b", "", "  "])).toEqual(["a", "b"]);
	});
});

describe("fetchStoreCards", () => {
	it("splits more than 100 ids into batches of at most 100", async () => {
		const cards = await fetchStoreCards(idsOf(250));
		const sizes = mocks.batchGetStores.mock.calls.map(
			([req]) => (req as { ids: string[] }).ids.length,
		);
		expect(sizes).toEqual([100, 100, 50]);
		expect(cards.size).toBe(250);
	});

	it("makes one request for exactly 100 ids", async () => {
		await fetchStoreCards(idsOf(100));
		expect(mocks.batchGetStores).toHaveBeenCalledTimes(1);
	});

	it("leaves a store without a card out of the result", async () => {
		const cards = await fetchStoreCards(["a", "missing", "b"]);
		expect([...cards.keys()].sort()).toEqual(["a", "b"]);
		expect(cards.has("missing")).toBe(false);
	});

	it("does not call the server for an empty or blank id list", async () => {
		const cards = await fetchStoreCards(["", " "]);
		expect(cards.size).toBe(0);
		expect(mocks.batchGetStores).not.toHaveBeenCalled();
	});
});
