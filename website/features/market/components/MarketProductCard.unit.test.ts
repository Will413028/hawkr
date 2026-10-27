/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { MarketProductItem } from "@/lib/market/types";
import MarketProductCard from "./MarketProductCard";

// Exercise the real card, image and icon boundary. Business actions are test
// doubles: no cart/favourite writes, and no claim of authenticated acceptance.
vi.mock("@/components/shared/Link", () => ({
	default: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) =>
		React.createElement("a", props),
}));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/FavoriteButton", () => ({
	default: () => React.createElement("button", { type: "button" }, "收藏"),
}));
vi.mock("@/components/shared/RestockNotifyButton", () => ({
	default: () => React.createElement("button", { type: "button" }, "通知補貨"),
}));
vi.mock("@/components/shared/AddToCartButton", () => ({
	default: (props: {
		children: React.ReactNode;
		disabled: boolean;
		onAdded: () => void;
		ariaLabel: string;
	}) =>
		React.createElement(
			"button",
			{
				type: "button",
				disabled: props.disabled,
				onClick: props.onAdded,
				"aria-label": props.ariaLabel,
			},
			props.children,
		),
}));
vi.mock("@/lib/analytics/events", () => ({ trackSelectItem: vi.fn() }));

const product: MarketProductItem = {
	id: "test-card",
	name: "測試商品",
	image: "/brand/logo-mark.svg",
	sellerName: "測試賣家",
	origin: "苗栗",
	price: "$100",
	isOnSale: false,
	hashtags: [],
	unit: "包",
	rating: null,
	reviews: 0,
	href: "/products/test-card",
	priceValue: 100,
	createdAt: 0,
	salesCount: 0,
	stock: 2,
};
afterEach(() => {
	cleanup();
});

describe("MarketProductCard", () => {
	it("server-renders the complete card, eager image and the cart icon", () => {
		const host = document.createElement("div");
		host.innerHTML = renderToStaticMarkup(
			React.createElement(MarketProductCard, {
				product,
				imageLoading: "eager",
			}),
		);
		expect(host.textContent).toContain("測試商品");
		expect(
			host
				.querySelector("[data-market-product-primary-link]")
				?.getAttribute("href"),
		).toBe(product.href);
		expect(host.querySelector("img")?.getAttribute("loading")).toBe("eager");
		expect(host.querySelector("img")?.getAttribute("fetchPriority")).toBe(
			"low",
		);
		const icon = host.querySelector(".market-product-card__cart-btn-icon");
		expect(icon?.getAttribute("data-icon-name")).toBe("bag");
		expect(icon?.getAttribute("aria-hidden")).toBe("true");
	});

	it("keeps quantity and simulated success/reset usable", () => {
		render(React.createElement(MarketProductCard, { product }));
		expect(
			(screen.getByRole("button", { name: /請先選擇/ }) as HTMLButtonElement)
				.disabled,
		).toBe(true);
		const plus = screen.getByRole("button", { name: /增加.*購買數量/ });
		fireEvent.click(plus);
		fireEvent.click(plus);
		expect((plus as HTMLButtonElement).disabled).toBe(true);
		fireEvent.click(
			screen.getByRole("button", { name: /加入購物車：.*數量 2/ }),
		);
		expect(
			(screen.getByRole("button", { name: /請先選擇/ }) as HTMLButtonElement)
				.disabled,
		).toBe(true);
	});

	it("shows the restock control for sold-out products and a disabled cart for preview", () => {
		const view = render(
			React.createElement(MarketProductCard, {
				product: { ...product, stock: 0 },
			}),
		);
		expect(screen.getByRole("button", { name: "通知補貨" })).toBeDefined();
		expect(
			view.container.querySelector(".market-product-card__cart-btn-icon"),
		).toBeNull();
		view.rerender(
			React.createElement(MarketProductCard, { product, previewMode: true }),
		);
		expect(
			(screen.getByRole("button", { name: /預覽模式/ }) as HTMLButtonElement)
				.disabled,
		).toBe(true);
		expect(
			view.container.querySelector(".market-product-card__cart-btn-icon"),
		).not.toBeNull();
	});
});
