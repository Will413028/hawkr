"use client";

import {
	CustomerSort,
	CustomerType,
} from "@hawkr/api/gen/hawkr/crm/v1alpha1/crm_pb";
import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";

/** 篩選的分類：UNSPECIFIED 是「全部」（請求不帶 type）。 */
export type SellerCustomersTypeFilter = CustomerType;

type SellerCustomersPersistedState = {
	customerType: CustomerType;
	sort: CustomerSort;
	tagId: string | null;
};

type SellerCustomersUiState = SellerCustomersPersistedState & {
	activeCustomerId: string | null;
	setActiveCustomerId: (id: string | null) => void;
	setCustomerType: (customerType: CustomerType) => void;
	setSort: (sort: CustomerSort) => void;
	setTagId: (tagId: string | null) => void;
};

const defaultState: SellerCustomersPersistedState = {
	customerType: CustomerType.UNSPECIFIED,
	sort: CustomerSort.TOTAL_SPENT,
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
		setSort: (sort) => set({ sort }, false, "seller-customers/setSort"),
		setTagId: (tagId) => set({ tagId }, false, "seller-customers/setTagId"),
	}),
	{
		keyName: "sellerCustomersUi",
		devtoolsName: "seller-customers-ui-store",
		defaults: defaultState,
		partialize: (state) => ({
			customerType: state.customerType,
			sort: state.sort,
			tagId: state.tagId,
		}),
		// 版本 0 存的是字串分類與排序（含 blocklist、last_order_at）與搜尋字串，契約已沒有這些值：丟掉，回預設。
		persistOptions: { version: 1, migrate: () => defaultState },
	},
);
