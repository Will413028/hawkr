/** @vitest-environment jsdom */

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import HomeCompactNewArrivals from "@/features/home/components/HomeCompactNewArrivals";
import { trackSelectItem } from "@/lib/analytics/events";
import { useMarketProducts } from "@/lib/hooks/useMarketProducts";
import type { MarketProductItem } from "@/lib/market/types";
import MarketProductsList from "./MarketProductsList";

const { imageRender, imagePolicy } = vi.hoisted(() => ({
	imageRender: vi.fn(),
	imagePolicy: vi.fn(),
}));
vi.mock("@/lib/hooks/useMarketProducts", () => ({
	useMarketProducts: vi.fn(),
}));
vi.mock("@/lib/analytics/events", () => ({
	trackSelectItem: vi.fn(),
	trackViewItemList: vi.fn(),
}));
vi.mock("@/components/shared/Link", () => ({
	default: ({
		scroll: _scroll,
		...props
	}: React.AnchorHTMLAttributes<HTMLAnchorElement> & { scroll?: boolean }) =>
		React.createElement("a", props),
}));
// A render probe at a direct child of the REAL card. The REAL list and masonry
// still run their hydration, append and measured-layout passes. Network/actions
// are isolated; this is a render-count test, not authenticated checkout evidence.
vi.mock("@/components/shared/AppImage", () => ({
	default: (props: { alt: string }) => {
		imageRender(props.alt);
		imagePolicy(props);
		return null;
	},
}));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/EmptyStatePanel", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/FavoriteButton", () => ({
	default: (props: { initialFavorited?: boolean }) =>
		React.createElement(
			"button",
			{ type: "button", "aria-pressed": Boolean(props.initialFavorited) },
			"收藏",
		),
}));
vi.mock("@/components/shared/RestockNotifyButton", () => ({
	default: () => React.createElement("button", { type: "button" }, "通知補貨"),
}));
vi.mock("@/components/shared/AddToCartButton", () => ({
	default: (props: {
		children: React.ReactNode;
		disabled: boolean;
		ariaLabel: string;
	}) =>
		React.createElement(
			"button",
			{
				type: "button",
				disabled: props.disabled,
				"aria-label": props.ariaLabel,
			},
			props.children,
		),
}));

function product(index: number): MarketProductItem {
	return {
		id: String(index),
		name: `商品 ${index}`,
		image: "/test.jpg",
		sellerName: "測試賣家",
		origin: "苗栗",
		price: "$100",
		priceValue: 100,
		isOnSale: false,
		hashtags: [],
		unit: "包",
		rating: null,
		reviews: 0,
		href: `/products/${index}`,
		createdAt: 0,
		salesCount: 0,
		stock: 10,
	};
}
const fetchParams = { keyword: "" };
const sortLinks = [{ value: "推薦", label: "推薦", href: "/market/products" }];
const loadMore = vi.fn();
let products: MarketProductItem[];
let loading: boolean;
let initialLoading: boolean;

function list() {
	return React.createElement(MarketProductsList, {
		products,
		fetchParams,
		sortLinks,
		currentSort: "推薦",
		pageSize: 24,
		initialDone: false,
	});
}

beforeEach(() => {
	vi.clearAllMocks();
	window.sessionStorage.clear();
	products = Array.from({ length: 24 }, (_, index) => product(index));
	loading = false;
	initialLoading = false;
	vi.mocked(useMarketProducts).mockImplementation(
		() =>
			({
				products,
				page: 1,
				done: false,
				loading,
				initialLoading,
				loadMore,
				refreshing: false,
				hasPendingFresh: false,
				pendingNewCount: 0,
				applyPendingFresh: vi.fn(),
				refresh: vi.fn(),
			}) as ReturnType<typeof useMarketProducts>,
	);
	vi.stubGlobal(
		"IntersectionObserver",
		class {
			observe() {}
			unobserve() {}
			disconnect() {}
		},
	);
	vi.stubGlobal(
		"ResizeObserver",
		class {
			observe() {}
			disconnect() {}
		},
	);
	vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(1000);
	vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(300);
	const original = window.getComputedStyle;
	vi.spyOn(window, "getComputedStyle").mockImplementation((element, pseudo) => {
		const styles = original(element, pseudo);
		if (element.hasAttribute("data-masonry-probe")) {
			Object.defineProperty(styles, "gridTemplateColumns", {
				value: "240px 240px 240px 240px",
			});
			Object.defineProperty(styles, "columnGap", { value: "12px" });
		}
		return styles;
	});
});
afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
	vi.unstubAllGlobals();
});

describe("product list append render isolation", () => {
	it("keeps one critical image and a small eager row, while appended cards stay lazy", () => {
		const view = render(list());
		const initial = imagePolicy.mock.calls.map(([props]) => props);
		expect(initial.filter((props) => props.priority)).toHaveLength(1);
		expect(initial[0]).toMatchObject({ alt: "商品 0", priority: true });
		expect(
			initial.slice(1, 4).every((props) => props.loading === "eager"),
		).toBe(true);
		expect(initial.slice(4).every((props) => props.loading === "lazy")).toBe(
			true,
		);
		imagePolicy.mockClear();
		products = [...products, product(24), product(25)];
		view.rerender(list());
		expect(imagePolicy.mock.calls.map(([props]) => props.alt)).toEqual([
			"商品 24",
			"商品 25",
		]);
		expect(
			imagePolicy.mock.calls.every(
				([props]) => props.loading === "lazy" && !props.priority,
			),
		).toBe(true);
		expect(
			screen.getByRole("link", { name: "查看商品：商品 25" }),
		).toBeTruthy();
	});

	it("renders lower home shelf content with lazy images and no critical preload", () => {
		render(React.createElement(HomeCompactNewArrivals, { products }));
		expect(imagePolicy.mock.calls).toHaveLength(24);
		expect(
			imagePolicy.mock.calls.every(
				([props]) => props.loading === "lazy" && !props.priority,
			),
		).toBe(true);
		expect(
			screen.getByRole("link", { name: "查看商品：商品 23" }),
		).toBeTruthy();
	});

	it("shows a product-only skeleton until the initial filtered data arrives", () => {
		products = [];
		initialLoading = true;
		loading = true;
		const view = render(list());
		expect(screen.getByRole("status", { name: "商品載入中" })).toBeTruthy();
		expect(
			view.container.querySelector(".route-loading-skeleton__story-row"),
		).toBeNull();
		products = [product(0)];
		initialLoading = false;
		loading = false;
		view.rerender(list());
		expect(screen.queryByRole("status", { name: "商品載入中" })).toBeNull();
		expect(screen.getByRole("link", { name: "查看商品：商品 0" })).toBeTruthy();
	});

	it.each([
		1, 24, 100,
	])("keeps %i existing cards idle through loading, append and masonry reflow", (count) => {
		products = Array.from({ length: count }, (_, index) => product(index));
		const view = render(list());
		const cards = Array.from(view.container.querySelectorAll("article"));
		imageRender.mockClear();
		loading = true;
		view.rerender(list());
		const loadingRenders = imageRender.mock.calls.length;
		imageRender.mockClear();
		products = [
			...products,
			...Array.from({ length: 24 }, (_, index) => product(count + index)),
		];
		loading = false;
		view.rerender(list());
		const oldRenders = imageRender.mock.calls.filter(
			([name]) => Number(name.split(" ")[1]) < count,
		).length;
		const newRenders = imageRender.mock.calls.length - oldRenders;
		imageRender.mockClear();
		act(() => window.dispatchEvent(new Event("resize")));
		const resizeRenders = imageRender.mock.calls.length;
		console.info(
			JSON.stringify({
				count,
				appended: 24,
				loadingRenders,
				oldRenders,
				newRenders,
				resizeRenders,
			}),
		);
		expect(loadingRenders).toBe(0);
		expect(oldRenders).toBe(0);
		expect(newRenders).toBe(24);
		expect(resizeRenders).toBe(0);
		expect(view.container.querySelectorAll("article")).toHaveLength(count + 24);
		cards.forEach((card, index) => {
			expect(view.container.querySelectorAll("article")[index]).toBe(card);
		});
	});

	it("still updates quantity locally and refreshed business data on the affected card", () => {
		const view = render(list());
		imageRender.mockClear();
		fireEvent.click(
			screen.getByRole("button", { name: "增加 商品 0 購買數量" }),
		);
		expect(imageRender.mock.calls).toEqual([["商品 0"]]);
		expect(
			screen.getByRole("button", { name: "加入購物車：商品 0，數量 1" }),
		).toBeDefined();
		imageRender.mockClear();
		products = products.map((item, index) =>
			index === 1
				? {
						...item,
						name: "更新商品",
						price: "$80",
						priceValue: 80,
						stock: 0,
						isFavorited: true,
						hashtags: ["新標籤"],
						href: "/products/updated",
					}
				: item,
		);
		view.rerender(list());
		expect(imageRender.mock.calls).toEqual([["更新商品"]]);
		expect(screen.getByText("$80")).toBeDefined();
		expect(screen.getByText("#新標籤")).toBeDefined();
		expect(screen.getByRole("button", { name: "通知補貨" })).toBeDefined();
		expect(
			screen
				.getAllByRole("button", { name: "收藏" })[1]
				.getAttribute("aria-pressed"),
		).toBe("true");
		expect(
			screen
				.getByRole("link", { name: "查看商品：更新商品" })
				.getAttribute("href"),
		).toBe("/products/updated");
	});

	it("keeps home rail context stable on append but updates analytics when its title changes", () => {
		const view = render(
			React.createElement(HomeCompactNewArrivals, { products }),
		);
		imageRender.mockClear();
		products = [...products, product(24)];
		view.rerender(React.createElement(HomeCompactNewArrivals, { products }));
		expect(imageRender.mock.calls).toEqual([["商品 24"]]);
		view.rerender(
			React.createElement(HomeCompactNewArrivals, {
				products,
				title: "當季推薦",
			}),
		);
		const link = screen.getByRole("link", { name: "查看商品：商品 0" });
		// Prevent jsdom navigation only after the real card's analytics callback.
		document.addEventListener("click", (event) => event.preventDefault(), {
			once: true,
		});
		fireEvent.click(link);
		expect(trackSelectItem).toHaveBeenLastCalledWith(
			products[0],
			{ listId: "home_new_arrivals", listName: "當季推薦" },
			0,
		);
	});
});
