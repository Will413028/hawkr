"use client";

import type { CartAvailableCoupon } from "@/lib/coupons/repo";
import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";

export interface AppliedCoupon {
	templateId: string;
	instanceId?: string;
	title: string;
	kind: CartAvailableCoupon["kind"];
	estimatedDiscount: number;
	/** 券適用賣家的 store surrogate（來自 `applicableStoreId`；空 = 全平台券）。 */
	storeId: string;
}

type SelectionUpdater =
	| Record<string, boolean>
	| ((current: Record<string, boolean>) => Record<string, boolean>);

type CartUiPersistedState = {
	coupon: AppliedCoupon | null;
	note: string;
	selected: Record<string, boolean>;
	selectedAddressId: string | null;
	selectedPickupId: string | null;
	shippingDeliveryTemp: string;
	shippingMethod: string;
};

type CartUiState = CartUiPersistedState & {
	addressPickerOpen: boolean;
	clearing: boolean;
	couponSheetOpen: boolean;
	lastValidateAt: number;
	preciseShippingFee: number | null;
	shippingFeeLoading: boolean;
	shippingRefreshNonce: number;
	storePickupPickerOpen: boolean;
	clearCoupon: () => void;
	requestShippingRefresh: () => void;
	setAddressPickerOpen: (open: boolean) => void;
	setClearing: (clearing: boolean) => void;
	setCoupon: (coupon: AppliedCoupon | null) => void;
	setCouponSheetOpen: (open: boolean) => void;
	setLastValidateAt: (timestamp: number) => void;
	setNote: (note: string) => void;
	setPreciseShippingFee: (fee: number | null) => void;
	setSelected: (selected: SelectionUpdater) => void;
	setSelectedAddressId: (id: string | null) => void;
	setSelectedPickupId: (id: string | null) => void;
	setShippingFeeLoading: (loading: boolean) => void;
	setShippingSelection: (method: string, deliveryTemp: string) => void;
	setStorePickupPickerOpen: (open: boolean) => void;
};

const defaultPersistedState: CartUiPersistedState = {
	coupon: null,
	note: "",
	selected: {},
	selectedAddressId: null,
	selectedPickupId: null,
	shippingDeliveryTemp: "",
	shippingMethod: "",
};

export const useCartUiStore = createLocalPrivateStore<
	CartUiState,
	CartUiPersistedState
>(
	(set) => ({
		...defaultPersistedState,
		addressPickerOpen: false,
		clearing: false,
		couponSheetOpen: false,
		lastValidateAt: 0,
		preciseShippingFee: null,
		shippingFeeLoading: false,
		shippingRefreshNonce: 0,
		storePickupPickerOpen: false,
		clearCoupon: () => set({ coupon: null }, false, "cart-ui/clearCoupon"),
		requestShippingRefresh: () =>
			set(
				(state) => ({
					shippingRefreshNonce: state.shippingRefreshNonce + 1,
				}),
				false,
				"cart-ui/requestShippingRefresh",
			),
		setAddressPickerOpen: (addressPickerOpen) =>
			set({ addressPickerOpen }, false, "cart-ui/setAddressPickerOpen"),
		setClearing: (clearing) => set({ clearing }, false, "cart-ui/setClearing"),
		setCoupon: (coupon) => set({ coupon }, false, "cart-ui/setCoupon"),
		setCouponSheetOpen: (couponSheetOpen) =>
			set({ couponSheetOpen }, false, "cart-ui/setCouponSheetOpen"),
		setLastValidateAt: (lastValidateAt) =>
			set({ lastValidateAt }, false, "cart-ui/setLastValidateAt"),
		setNote: (note) => set({ note }, false, "cart-ui/setNote"),
		setPreciseShippingFee: (preciseShippingFee) =>
			set({ preciseShippingFee }, false, "cart-ui/setPreciseShippingFee"),
		setSelected: (selected) =>
			set(
				(state) => ({
					selected:
						typeof selected === "function"
							? selected(state.selected)
							: selected,
				}),
				false,
				"cart-ui/setSelected",
			),
		setSelectedAddressId: (selectedAddressId) =>
			set({ selectedAddressId }, false, "cart-ui/setSelectedAddressId"),
		setSelectedPickupId: (selectedPickupId) =>
			set({ selectedPickupId }, false, "cart-ui/setSelectedPickupId"),
		setShippingFeeLoading: (shippingFeeLoading) =>
			set({ shippingFeeLoading }, false, "cart-ui/setShippingFeeLoading"),
		setShippingSelection: (shippingMethod, shippingDeliveryTemp) =>
			set(
				{ shippingDeliveryTemp, shippingMethod },
				false,
				"cart-ui/setShippingSelection",
			),
		setStorePickupPickerOpen: (storePickupPickerOpen) =>
			set({ storePickupPickerOpen }, false, "cart-ui/setStorePickupPickerOpen"),
	}),
	{
		keyName: "cartUi",
		devtoolsName: "cart-ui-store",
		defaults: defaultPersistedState,
		partialize: (state): CartUiPersistedState => ({
			coupon: state.coupon,
			note: state.note,
			selected: state.selected,
			selectedAddressId: state.selectedAddressId,
			selectedPickupId: state.selectedPickupId,
			shippingDeliveryTemp: state.shippingDeliveryTemp,
			shippingMethod: state.shippingMethod,
		}),
		persistOptions: {
			// SSR-safe：跳過建立時同步 rehydrate，改由 CartClient 掛載後手動 rehydrate，
			// 讓 client 首次渲染與 server 一致（避免 hydration mismatch / 閃爍）。
			// 不 bump version：:ui 持久 schema 未變，移除的只是從未寫入 :ui 的舊散 key。
			skipHydration: true,
		},
	},
);
