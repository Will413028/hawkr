// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const hook = vi.hoisted(() => ({
	customers: {} as Record<string, unknown>,
}));
vi.mock("@/lib/hooks/useSellerCustomers", () => ({
	useSellerCustomers: () => hook.customers,
	useInvalidateSellerCustomers: () => async () => {},
}));
vi.mock("@/lib/hooks/useSellerTags", () => ({
	useSellerTags: () => ({ tags: [], error: null }),
}));
vi.mock("@/lib/orders/repo", () => ({
	fetchOrders: async () => ({ records: [], total: 0 }),
}));
vi.mock("@/lib/state/chat-drawer-store", () => ({
	useChatDrawerStore: () => () => {},
}));
vi.mock("./customersUiStore", () => ({
	useSellerCustomersUiStore: (sel: (s: unknown) => unknown) =>
		sel({
			customerType: 0,
			sort: 1,
			tagId: null,
			activeCustomerId: null,
			setActiveCustomerId: () => {},
			setCustomerType: () => {},
			setSort: () => {},
			setTagId: () => {},
		}),
}));
vi.mock("./CustomerDetailDrawer", () => ({ default: () => null }));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({
		children,
		subtitle,
	}: {
		children: React.ReactNode;
		subtitle: string;
	}) => React.createElement("div", null, subtitle, children),
}));
vi.mock("@/components/shared/ScrollableTableRegion", () => ({
	ScrollableTableRegion: ({ children }: { children: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
vi.mock("@/features/seller-center", () => ({
	SellerCustomersSkeletonRows: () => null,
	SellerEmptyState: () => React.createElement("span", null, "empty"),
}));
vi.mock(
	"@/features/seller-center/components/SellerMobileDisclosureList",
	() => ({
		SellerMobileDisclosureList: () => null,
	}),
);

import CustomersClient from "./CustomersClient";

const row = (id: string, spent: number, orders: number) => ({
	id,
	name: `n${id}`,
	tags: [],
	totalSpent: spent,
	orderCount: orders,
	avatar: "",
});

afterEach(cleanup);

describe("CustomersClient summary", () => {
	it("shows only the loaded count (with + when more pages exist) and no totals computed from loaded pages", () => {
		hook.customers = {
			data: [row("1", 500, 3), row("2", 700, 1)],
			loading: false,
			error: null,
			hasMore: true,
			loadingMore: false,
			loadMore: async () => {},
		};
		const { container } = render(React.createElement(CustomersClient));
		expect(container.textContent).toContain("2+");
		expect(screen.queryByText("客戶累積消費")).toBeNull();
		expect(screen.queryByText(/回購客戶/)).toBeNull();
		expect(container.textContent).not.toContain("NT$ 1,200");
	});
});
