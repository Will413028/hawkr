import { describe, expect, it, vi } from "vitest";
import {
	hydrationClientSnapshot,
	hydrationServerSnapshot,
} from "@/lib/hooks/useHydrationSafeClientRender";
import {
	type MarketListSnapshot,
	marketListSnapshotForInitialRender,
} from "@/lib/market/listSnapshot";

const snapshot: MarketListSnapshot = {
	key: "/market/products",
	products: [
		{
			id: "snapshot-product",
			name: "Snapshot product",
			image: "",
			sellerName: "Snapshot seller",
			origin: "Taiwan",
			price: "NT$ 100",
			isOnSale: false,
			hashtags: [],
			unit: "份",
			rating: null,
			reviews: 0,
			href: "/products/snapshot-product",
			priceValue: 100,
			createdAt: 1,
			salesCount: 0,
			isFavorited: false,
		},
	],
	page: 2,
	done: false,
	scrollY: 720,
	savedAt: 1,
};

describe("market snapshot hydration boundary", () => {
	it("does not read browser state for SSR or the first hydration render", () => {
		const readSnapshot = vi.fn(() => snapshot);

		for (const phase of ["SSR", "first hydration render"]) {
			expect(
				marketListSnapshotForInitialRender(
					hydrationServerSnapshot(),
					readSnapshot,
				),
				phase,
			).toBeNull();
		}
		expect(readSnapshot).not.toHaveBeenCalled();
	});

	it("restores synchronously when mounted after hydration", () => {
		const readSnapshot = vi.fn(() => snapshot);

		expect(
			marketListSnapshotForInitialRender(
				hydrationClientSnapshot(),
				readSnapshot,
			),
		).toBe(snapshot);
		expect(readSnapshot).toHaveBeenCalledOnce();
	});
});
