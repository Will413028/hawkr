import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	getCachedStore: vi.fn(),
	fetchStoreCards: vi.fn(),
}));
vi.mock("@/lib/api/cached", () => ({ getCachedStore: mocks.getCachedStore }));
vi.mock("@/lib/seller/fetchStoreCards", () => ({
	fetchStoreCards: mocks.fetchStoreCards,
}));
vi.mock("./components/ProductSellerIntro", () => ({ default: () => null }));

import SellerIntroAsync from "./SellerIntroAsync";

type Props = Record<string, unknown>;
async function propsOf(productSeller: unknown): Promise<Props> {
	const el = (await SellerIntroAsync({
		sellerId: "7487768729113814280",
		productSeller,
	})) as ReactElement<Props>;
	return el.props;
}

beforeEach(() => {
	mocks.getCachedStore.mockReset();
	mocks.fetchStoreCards.mockReset();
});

describe("SellerIntroAsync", () => {
	it("reads GetStore by store_code when the product seller block carries one", async () => {
		mocks.getCachedStore.mockResolvedValue({
			storeId: "7487768729113814280",
			storeCode: "2662192214",
			logo_url: "https://m/x.png",
			ratingAverage: 4.5,
			reviewCount: 3,
			followerCount: 8,
		});
		const props = await propsOf({ storeCode: "2662192214" });
		expect(mocks.getCachedStore).toHaveBeenCalledWith("2662192214");
		expect(mocks.fetchStoreCards).not.toHaveBeenCalled();
		expect(props.followerStats).toEqual({ followers: 8 });
		expect(props.sellerStats).toEqual({ avgRating: 4.5, totalCount: 3 });
	});

	it("resolves a block without store_code through BatchGetStores by id and leaves followers unset", async () => {
		mocks.fetchStoreCards.mockResolvedValue(
			new Map([
				[
					"7487768729113814280",
					{
						id: "7487768729113814280",
						storeCode: "2662192214",
						name: "花蓮小鋪",
						reviewCount: 3,
						ratingAverage: 4.5,
					},
				],
			]),
		);
		const props = await propsOf({ name: "x" });
		expect(mocks.getCachedStore).not.toHaveBeenCalled();
		expect(mocks.fetchStoreCards).toHaveBeenCalledWith(["7487768729113814280"]);
		expect((props.seller as Props).storeCode).toBe("2662192214");
		expect(props.sellerStats).toEqual({ avgRating: 4.5, totalCount: 3 });
		expect(props.followerStats).toBeNull();
	});

	it("passes no stats when the store read fails", async () => {
		mocks.fetchStoreCards.mockRejectedValue(new Error("down"));
		const props = await propsOf(null);
		expect(props.seller).toBeNull();
		expect(props.sellerStats).toBeNull();
		expect(props.followerStats).toBeNull();
	});
});
