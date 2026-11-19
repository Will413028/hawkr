import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api/cached", () => ({
	getCachedSellerProducts: async () => null,
	getCachedSellerStorefrontProducts: async () => null,
}));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/BrandLogo", () => ({ default: () => null }));
vi.mock("@/components/shared/Link", () => ({
	default: ({ href, children }: { href: string; children: unknown }) =>
		createElement("a", { href }, children as never),
}));
vi.mock("@/features/seller/components/SellerCoverMedia", () => ({
	default: () => null,
}));
vi.mock("./SellerCardProductRail", () => ({ default: () => null }));

import SellerCardPage from "./SellerCardPage";

async function html(seller: Record<string, unknown>): Promise<string> {
	return renderToStaticMarkup(
		await SellerCardPage({
			seller: { name: "店", storeCode: "2662192214", ...seller },
		}),
	);
}

describe("SellerCardPage stats", () => {
	it("hides the review count segment when reviewCount is unset, like the rating", async () => {
		const out = await html({ ratingAverage: 4.5 });
		expect(out).toContain("4.5");
		expect(out).not.toContain("則評價");
	});

	it("shows the review count when it is set", async () => {
		const out = await html({ ratingAverage: 4.5, reviewCount: 12 });
		expect(out).toContain("12 則評價");
	});

	it("links the storefront by store_code", async () => {
		expect(await html({})).toContain('href="/sellers/2662192214"');
	});
});
