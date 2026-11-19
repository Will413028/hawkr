import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	getCachedStore: vi.fn(),
}));
vi.mock("@/lib/api/cached", () => ({ getCachedStore: mocks.getCachedStore }));
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
		expect(props.followerStats).toEqual({ followers: 8 });
		expect(props.sellerStats).toEqual({ avgRating: 4.5, totalCount: 3 });
	});

	it("renders no link and unset stats when the block has no store_code", async () => {
		const props = await propsOf({ name: "x" });
		expect(mocks.getCachedStore).not.toHaveBeenCalled();
		expect(props.seller).toBeNull();
		expect(props.sellerStats).toBeNull();
		expect(props.followerStats).toBeNull();
	});

	it("passes no stats when the store read fails", async () => {
		mocks.getCachedStore.mockRejectedValue(new Error("down"));
		const props = await propsOf({ storeCode: "2662192214" });
		expect(props.seller).toBeNull();
		expect(props.sellerStats).toBeNull();
		expect(props.followerStats).toBeNull();
	});

	it("drops the embedded store_code when GetStore says the store is not public", async () => {
		mocks.getCachedStore.mockResolvedValue(null);
		const props = await propsOf({ storeCode: "2662192214", name: "未公開店" });
		expect(props.seller).toBeNull();
		expect(props.productSeller).toEqual({ name: "未公開店" });
		expect(props.productSeller).not.toHaveProperty("storeCode");
	});

	it("keeps the embedded store_code when the store read merely fails", async () => {
		mocks.getCachedStore.mockRejectedValue(new Error("down"));
		const props = await propsOf({ storeCode: "2662192214", name: "店" });
		expect(props.productSeller).toEqual({
			storeCode: "2662192214",
			name: "店",
		});
	});
});
