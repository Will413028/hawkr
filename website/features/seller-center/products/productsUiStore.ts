"use client";

import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";

/**
 * 可以送給列表端點的 `status`。`4003`（下架中）只有代行工作區的 KPI 卡會設定 ——
 * 賣家中心沒有那張卡，而這個 store 綁帳號身份、身份轉換時整包還原成預設值，
 * 所以賣家不會繼承到一個自己畫面上沒有對應控制項的篩選值。
 */
export type ProductStatusFilter = "4001" | "4003" | "4004";
export type ProductFilter = ProductStatusFilter | "low_stock" | undefined;

type SellerProductsPersistedState = {
	search: string;
	statusFilter: ProductFilter;
};

type SellerProductsUiState = SellerProductsPersistedState & {
	setSearch: (search: string) => void;
	setStatusFilter: (filter: ProductFilter) => void;
};

function migrateProductFilter(value: unknown): ProductFilter {
	if (value === 1 || value === "1" || value === "4004") return "4004";
	if (value === 0 || value === "0" || value === "4001") return "4001";
	if (value === 2 || value === "2" || value === "4003") return "4003";
	if (value === "low_stock") return "low_stock";
	return undefined;
}

const defaultState: SellerProductsPersistedState = {
	search: "",
	statusFilter: undefined,
};

export const useSellerProductsUiStore = createLocalPrivateStore<
	SellerProductsUiState,
	SellerProductsPersistedState
>(
	(set) => ({
		...defaultState,
		setSearch: (search) => set({ search }, false, "seller-products/setSearch"),
		setStatusFilter: (statusFilter) =>
			set({ statusFilter }, false, "seller-products/setStatusFilter"),
	}),
	{
		keyName: "sellerProductsUi",
		devtoolsName: "seller-products-ui-store",
		defaults: defaultState,
		partialize: (state): SellerProductsPersistedState => ({
			search: state.search,
			statusFilter: state.statusFilter,
		}),
		persistOptions: {
			version: 1,
			migrate: (persisted): SellerProductsPersistedState => {
				const state =
					persisted && typeof persisted === "object"
						? (persisted as Partial<SellerProductsPersistedState>)
						: {};
				return {
					search: typeof state.search === "string" ? state.search : "",
					statusFilter: migrateProductFilter(state.statusFilter),
				};
			},
		},
	},
);
