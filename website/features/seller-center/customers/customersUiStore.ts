"use client";

import type { SellerCustomerType } from "@/lib/seller/repo";
import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";

export type SellerCustomersSort =
	| "total_spent"
	| "order_count"
	| "last_order_at";

type SellerCustomersPersistedState = {
	customerType: "" | SellerCustomerType;
	search: string;
	sortBy: SellerCustomersSort;
	tagId: string | null;
};

type SellerCustomersUiState = SellerCustomersPersistedState & {
	activeCustomerId: string | null;
	setActiveCustomerId: (id: string | null) => void;
	setCustomerType: (customerType: "" | SellerCustomerType) => void;
	setSearch: (search: string) => void;
	setSortBy: (sortBy: SellerCustomersSort) => void;
	setTagId: (tagId: string | null) => void;
};

const defaultState: SellerCustomersPersistedState = {
	customerType: "",
	search: "",
	sortBy: "total_spent",
	tagId: null,
};

export const useSellerCustomersUiStore = createLocalPrivateStore<
	SellerCustomersUiState,
	SellerCustomersPersistedState
>(
	(set) => ({
		...defaultState,
		activeCustomerId: null,
		setActiveCustomerId: (activeCustomerId) =>
			set({ activeCustomerId }, false, "seller-customers/setActiveCustomerId"),
		setCustomerType: (customerType) =>
			set({ customerType }, false, "seller-customers/setCustomerType"),
		setSearch: (search) => set({ search }, false, "seller-customers/setSearch"),
		setSortBy: (sortBy) => set({ sortBy }, false, "seller-customers/setSortBy"),
		setTagId: (tagId) => set({ tagId }, false, "seller-customers/setTagId"),
	}),
	{
		keyName: "sellerCustomersUi",
		devtoolsName: "seller-customers-ui-store",
		defaults: defaultState,
		partialize: (state) => ({
			customerType: state.customerType,
			search: state.search,
			sortBy: state.sortBy,
			tagId: state.tagId,
		}),
	},
);
