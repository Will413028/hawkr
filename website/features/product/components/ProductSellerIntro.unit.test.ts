import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/Link", () => ({
	default: ({ href, children }: { href: string; children: unknown }) =>
		createElement("a", { href }, children as never),
}));
vi.mock("@/features/seller/components/SellerChatButton", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));

import ProductSellerIntro from "./ProductSellerIntro";

const render = (props: Record<string, unknown>) =>
	renderToStaticMarkup(
		createElement(ProductSellerIntro, {
			sellerId: "7487768729113814280",
			...props,
		}),
	);

describe("ProductSellerIntro", () => {
	it("links the storefront by store_code", () => {
		const html = render({
			productSeller: { storeCode: "2662192214", name: "店" },
		});
		expect(html).toContain('href="/sellers/2662192214"');
	});

	it("renders no storefront link when no store_code is available, never a store_id link", () => {
		const html = render({ productSeller: { name: "店" } });
		expect(html).not.toContain("/sellers/");
	});

	it("shows unset stats as a dash, not zero", () => {
		const html = render({
			productSeller: { storeCode: "2662192214" },
			followerStats: null,
		});
		expect(html).toMatch(/<strong[^>]*>—<\/strong><span>位訂閱/);
	});

	it("shows a real zero follower count as 0", () => {
		const html = render({
			productSeller: { storeCode: "2662192214" },
			followerStats: { followers: 0 },
		});
		expect(html).toMatch(/<strong[^>]*>0<\/strong><span>位訂閱/);
	});
});
