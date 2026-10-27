"use client";

import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";

export type InventoryFilter = "all" | "low" | "ok";

type SellerInventoryPersistedState = {
	filter: InventoryFilter;
	search: string;
};

type SellerInventoryUiState = SellerInventoryPersistedState & {
	setFilter: (filter: InventoryFilter) => void;
	setSearch: (search: string) => void;
};

const defaultState: SellerInventoryPersistedState = {
	filter: "all",
	search: "",
};

export const useSellerInventoryUiStore = createLocalPrivateStore<
	SellerInventoryUiState,
	SellerInventoryPersistedState
>(
	(set) => ({
		...defaultState,
		setFilter: (filter) => set({ filter }, false, "seller-inventory/setFilter"),
		setSearch: (search) => set({ search }, false, "seller-inventory/setSearch"),
	}),
	{
		keyName: "sellerInventoryUi",
		devtoolsName: "seller-inventory-ui-store",
		defaults: defaultState,
		partialize: (state): SellerInventoryPersistedState => ({
			filter: state.filter,
			search: state.search,
		}),
	},
);
