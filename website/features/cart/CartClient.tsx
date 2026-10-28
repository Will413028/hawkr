"use client";

import { postalCodeByTownCode } from "@hawkr/twaddr/lookup";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import { emitCartToast } from "@/components/shared/CartSnackbar";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import {
	cartSellerKeyOf,
	reconcileCartSelection,
	selectCartItem,
	selectCartSeller,
} from "@/features/cart/cartSelection";
import { useCartUiStore } from "@/features/cart/cartUiStore";
import { AddressPickerSheet } from "@/features/cart/components/AddressPickerSheet";
import {
	CartRowOperationContext,
	type CartRowPendingAction,
} from "@/features/cart/components/CartProductTile";
import { CouponSelectionSheet } from "@/features/cart/components/CouponSelectionSheet";
import { SellerGroupCard } from "@/features/cart/components/SellerGroupCard";
import { ShippingMethodGroup } from "@/features/cart/components/ShippingMethodGroup";
import { StorePickupPickerSheet } from "@/features/cart/components/StorePickupPickerSheet";
import { getCartCouponAvailability } from "@/features/cart/couponAvailability";
import { type Address, addressText } from "@/lib/addresses/repo";
import { trackRemoveFromCart, trackViewCart } from "@/lib/analytics/events";
import { useAuth } from "@/lib/auth/context";
import { type CartItem, parsePriceValue } from "@/lib/cart";
import { getSavedItems, removeSavedItem, saveItem } from "@/lib/cart/saved";
import { writeCheckoutDraft } from "@/lib/checkout/draft";
import { isFreeShippingForSubtotal } from "@/lib/checkout/free-shipping";
import {
	type AvailableShippingOption,
	calculateShippingFee,
} from "@/lib/checkout/repo";
import {
	isShippingDestinationReady,
	shippingDestinationKind,
} from "@/lib/checkout/shipping-requirements";
import {
	formatUnitLabel,
	SHIPPING_TEMPERATURE_LABEL,
	ShippingMethod,
	type ShippingTemperatureValue,
} from "@/lib/codemeta/enums";
import type { CartAvailableCoupon } from "@/lib/coupons/repo";
import { useAddresses } from "@/lib/hooks/useAddresses";
import { useCart } from "@/lib/hooks/useCart";
import { useCartCoupons } from "@/lib/hooks/useCartCoupons";
import { useShippingOptions } from "@/lib/hooks/useShippingOptions";
import { useStorePickups } from "@/lib/hooks/useStorePickups";
import { ossImageUrl } from "@/lib/media/cdn";
import { CartPageHeader } from "./components/CartPageHeader";

const STALE_VALIDATE_MS = 60_000;

function ntFormat(n: number) {
	return `NT$ ${Math.max(0, Math.round(n)).toLocaleString()}`;
}

function readStringField(value: unknown, keys: string[]) {
	const record =
		value && typeof value === "object"
			? (value as Record<string, unknown>)
			: {};
	for (const key of keys) {
		const next = record[key];
		if (next != null && String(next) !== "") return String(next);
	}
	return "";
}

function addressZipCode(address: Pick<Address, "townCode">) {
	return postalCodeByTownCode(address.townCode);
}

function sameSelection(
	a: Record<string, boolean>,
	b: Record<string, boolean>,
): boolean {
	const aKeys = Object.keys(a);
	const bKeys = Object.keys(b);
	if (aKeys.length !== bKeys.length) return false;
	for (const key of aKeys) {
		if (a[key] !== b[key]) return false;
	}
	return true;
}

export default function CartClient() {
	const { isLoggedIn, hydrated: authHydrated } = useAuth();
	const cart = useCart();
	const [mounted, setMounted] = React.useState(false);
	const [saved, setSaved] = React.useState<CartItem[]>([]);

	// This client boundary can hydrate later than AuthProvider/Navbar under
	// streamed SSR. By then the shared auth/cart query may already be settled,
	// while the server HTML for this boundary still contains the loading state.
	// Keep the first client render deterministic, then reveal the resolved cart
	// only after the persisted UI + save-for-later state have been read.
	//
	// cartUiStore uses skipHydration for the same reason: localStorage must never
	// influence the server render or the first client render (React #418).
	React.useEffect(() => {
		let active = true;
		const hydrateBrowserState = async () => {
			try {
				await useCartUiStore.persist.rehydrate();
			} catch {
				// Storage can be unavailable in restricted/private browser contexts.
				// The in-memory defaults still produce a fully usable cart.
			}
			if (!active) return;
			setSaved(getSavedItems());
			setMounted(true);
		};
		void hydrateBrowserState();
		return () => {
			active = false;
		};
	}, []);

	const viewCartFiredRef = React.useRef(false);
	React.useEffect(() => {
		if (viewCartFiredRef.current) return;
		if (cart.loading || cart.items.length === 0) return;
		viewCartFiredRef.current = true;
		trackViewCart(cart.items);
	}, [cart.loading, cart.items]);

	const addresses = useAddresses();
	const storePickups = useStorePickups();

	const selectionInitializedRef = React.useRef(false);
	const clearCartPendingRef = React.useRef(false);
	const rowPendingRef = React.useRef(new Map<string, CartRowPendingAction>());
	const savedRestorePendingRef = React.useRef(new Set<string>());
	const [rowPendingBySku, setRowPendingBySku] = React.useState<
		Record<string, CartRowPendingAction | undefined>
	>({});
	const [savedRestorePendingById, setSavedRestorePendingById] = React.useState<
		Record<string, boolean | undefined>
	>({});
	const addressPickerOpen = useCartUiStore((state) => state.addressPickerOpen);
	const clearing = useCartUiStore((state) => state.clearing);
	const coupon = useCartUiStore((state) => state.coupon);
	const couponSheetOpen = useCartUiStore((state) => state.couponSheetOpen);
	const lastValidateAt = useCartUiStore((state) => state.lastValidateAt);
	const note = useCartUiStore((state) => state.note);
	const preciseShippingFee = useCartUiStore(
		(state) => state.preciseShippingFee,
	);
	const selected = useCartUiStore((state) => state.selected);
	const selectedAddressId = useCartUiStore((state) => state.selectedAddressId);
	const selectedPickupId = useCartUiStore((state) => state.selectedPickupId);
	const shippingDeliveryTemp = useCartUiStore(
		(state) => state.shippingDeliveryTemp,
	);
	const shippingFeeLoading = useCartUiStore(
		(state) => state.shippingFeeLoading,
	);
	const shippingMethod = useCartUiStore((state) => state.shippingMethod);
	const shippingRefreshNonce = useCartUiStore(
		(state) => state.shippingRefreshNonce,
	);
	const storePickupPickerOpen = useCartUiStore(
		(state) => state.storePickupPickerOpen,
	);
	const clearCoupon = useCartUiStore((state) => state.clearCoupon);
	const requestShippingRefresh = useCartUiStore(
		(state) => state.requestShippingRefresh,
	);
	const setAddressPickerOpen = useCartUiStore(
		(state) => state.setAddressPickerOpen,
	);
	const setClearing = useCartUiStore((state) => state.setClearing);
	const setCoupon = useCartUiStore((state) => state.setCoupon);
	const setCouponSheetOpen = useCartUiStore(
		(state) => state.setCouponSheetOpen,
	);
	const setLastValidateAt = useCartUiStore((state) => state.setLastValidateAt);
	const setNote = useCartUiStore((state) => state.setNote);
	const setPreciseShippingFee = useCartUiStore(
		(state) => state.setPreciseShippingFee,
	);
	const setSelected = useCartUiStore((state) => state.setSelected);
	const setSelectedAddressId = useCartUiStore(
		(state) => state.setSelectedAddressId,
	);
	const setSelectedPickupId = useCartUiStore(
		(state) => state.setSelectedPickupId,
	);
	const setShippingFeeLoading = useCartUiStore(
		(state) => state.setShippingFeeLoading,
	);
	const setShippingSelection = useCartUiStore(
		(state) => state.setShippingSelection,
	);
	const setStorePickupPickerOpen = useCartUiStore(
		(state) => state.setStorePickupPickerOpen,
	);
	const [editingQty, setEditingQty] = React.useState<Record<string, string>>(
		{},
	);

	/* ───── Save-for-later sync ───── */

	React.useEffect(() => {
		const onChange = () => setSaved(getSavedItems());
		window.addEventListener("storage", onChange);
		return () => {
			window.removeEventListener("storage", onChange);
		};
	}, []);

	/* Reconcile persisted choices once and preserve deliberate deselection. */
	React.useEffect(() => {
		if (!mounted || cart.loading) return;
		// useCart can expose an empty placeholder before its first response.
		// Do not consume the initial choice or erase persisted choices there.
		if (!selectionInitializedRef.current && cart.items.length === 0) return;
		const prev = useCartUiStore.getState().selected;
		const next = reconcileCartSelection(
			cart.items,
			prev,
			!selectionInitializedRef.current,
		);
		selectionInitializedRef.current = true;
		if (!sameSelection(prev, next)) setSelected(next);
	}, [cart.items, cart.loading, mounted, setSelected]);

	/* ───── 不可購買商品改由 CartProductTile 反灰呈現（後端 issue #1127）。 ─────
	 * 舊版的「自動移除 + toast」邏輯已下線，避免：
	 *  - 兩端（web / iOS）行為不一致
	 *  - user agency 缺失：購物車莫名少一件造成困惑
	 *  - 賣家補貨後無法救回
	 * 反灰由 CartProductTile 讀 item.purchasable / item.unavailableReason 處理。
	 */

	/* ───── Active seller (single-seller checkout enforcement) ───── */

	const activeSellerKey = React.useMemo<string | null>(() => {
		for (const it of cart.items) {
			if (selected[it.id]) return cartSellerKeyOf(it);
		}
		return null;
	}, [cart.items, selected]);

	// 保留使用者目前勾選；不可購買商品不計入結帳，可從商品列逐項移除。
	const selectedItems = React.useMemo(
		() =>
			cart.items.filter(
				(it) =>
					selected[it.id] &&
					(activeSellerKey == null || cartSellerKeyOf(it) === activeSellerKey),
			),
		[cart.items, selected, activeSellerKey],
	);

	// 結帳用：從 selectedItems 排除不可購買 row（issue #1127）。
	const selectedPurchasableItems = React.useMemo(
		() => selectedItems.filter((it) => it.purchasable !== false),
		[selectedItems],
	);

	const itemsSubtotal = React.useMemo(
		() =>
			selectedPurchasableItems.reduce(
				(s, it) => s + it.priceValue * it.quantity,
				0,
			),
		[selectedPurchasableItems],
	);
	const selectedCount = selectedPurchasableItems.reduce(
		(s, it) => s + it.quantity,
		0,
	);

	/* ───── Aggregate shipping options + auto-default ───── */

	const checkoutItems = React.useMemo(
		() =>
			selectedPurchasableItems
				.filter((it) => it.productId != null)
				.map((it) => ({
					skuId: it.skuId,
					productId: it.productId,
					quantity: it.quantity,
				})),
		[selectedPurchasableItems],
	);
	const shippingState = useShippingOptions(
		checkoutItems,
		isLoggedIn && checkoutItems.length > 0,
		shippingRefreshNonce,
	);
	const shippingOptions: AvailableShippingOption[] = React.useMemo(
		() => shippingState.options ?? [],
		[shippingState.options],
	);
	// Stamp lastValidateAt only after a successful validate result. APP blocks
	// checkout when the validate snapshot is older than 60 seconds.
	React.useEffect(() => {
		if (!shippingState.loading && shippingState.options !== null) {
			setLastValidateAt(Date.now());
		}
	}, [shippingState.loading, shippingState.options, setLastValidateAt]);

	React.useEffect(() => {
		if (shippingOptions.length === 0) {
			if (shippingMethod !== "" || shippingDeliveryTemp !== "") {
				setShippingSelection("", "");
			}
			return;
		}
		// Drop the persisted selection when neither (method, temp) tuple
		// nor the bare method exists in the latest options — default to
		// the first option's full identity.
		const exactMatch = shippingOptions.some(
			(o) =>
				o.shippingMethod === shippingMethod &&
				(o.deliveryTemp ?? "") === shippingDeliveryTemp,
		);
		if (!exactMatch) {
			const methodMatch = shippingOptions.find(
				(o) => o.shippingMethod === shippingMethod,
			);
			const fallback = methodMatch ?? shippingOptions[0];
			setShippingSelection(
				fallback.shippingMethod,
				fallback.deliveryTemp ?? "",
			);
		}
	}, [
		shippingOptions,
		shippingMethod,
		shippingDeliveryTemp,
		setShippingSelection,
	]);

	const selectedShipping = React.useMemo<AvailableShippingOption | undefined>(
		() =>
			shippingOptions.find(
				(o) =>
					o.shippingMethod === shippingMethod &&
					(o.deliveryTemp ?? "") === shippingDeliveryTemp,
			) ?? shippingOptions.find((o) => o.shippingMethod === shippingMethod),
		[shippingOptions, shippingMethod, shippingDeliveryTemp],
	);
	const isStorePickup =
		shippingMethod === ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP;
	const isHomeDelivery = shippingMethod === ShippingMethod.TAQBIN;
	const freeShippingHit = isFreeShippingForSubtotal(
		selectedShipping,
		itemsSubtotal,
	);

	/* ───── Default address / pickup auto-pick ───── */

	React.useEffect(() => {
		// Keep an existing selection only if it still points at a saved address.
		// selectedAddressId is persisted to localStorage, so a stale id (an
		// address that was deleted/recreated, or one from a previous account)
		// must NOT block auto-pick — otherwise the cart shows "請選擇收件地址"
		// even though a default exists.
		if (
			selectedAddressId != null &&
			addresses.addresses.some((a) => a.id === selectedAddressId)
		) {
			return;
		}
		if (addresses.defaultAddress) {
			setSelectedAddressId(addresses.defaultAddress.id);
		} else if (addresses.addresses.length > 0) {
			setSelectedAddressId(addresses.addresses[0].id);
		}
	}, [
		addresses.addresses,
		addresses.defaultAddress,
		selectedAddressId,
		setSelectedAddressId,
	]);

	React.useEffect(() => {
		// Same stale-persisted-id guard as the address effect above.
		if (
			selectedPickupId != null &&
			storePickups.pickups.some((p) => p.id === selectedPickupId)
		) {
			return;
		}
		if (storePickups.defaultPickup) {
			setSelectedPickupId(storePickups.defaultPickup.id);
		} else if (storePickups.pickups.length > 0) {
			setSelectedPickupId(storePickups.pickups[0].id);
		}
	}, [
		storePickups.pickups,
		storePickups.defaultPickup,
		selectedPickupId,
		setSelectedPickupId,
	]);

	const selectedAddress = React.useMemo(
		() => addresses.addresses.find((a) => a.id === selectedAddressId) ?? null,
		[addresses.addresses, selectedAddressId],
	);
	const selectedPickup = React.useMemo(
		() => storePickups.pickups.find((p) => p.id === selectedPickupId) ?? null,
		[storePickups.pickups, selectedPickupId],
	);

	/* ───── Precise shipping fee (home delivery only) ───── */

	const baseShippingFee = selectedShipping?.fee ?? 0;
	const shippingFee = React.useMemo(() => {
		if (selectedItems.length === 0) return 0;
		if (freeShippingHit) return 0;
		if (isHomeDelivery && preciseShippingFee != null) return preciseShippingFee;
		return baseShippingFee;
	}, [
		selectedItems.length,
		freeShippingHit,
		isHomeDelivery,
		preciseShippingFee,
		baseShippingFee,
	]);

	// Serialized signature of everything the precise fee was computed FOR.
	// Keyed by value (not array identity) so unrelated re-renders don't
	// invalidate, but any address / method / temp / basket change does —
	// the fetch effect below early-returns while `preciseShippingFee` is
	// set, so without this reset the cart kept showing the fee quoted for
	// the previous selection.
	const preciseFeeKey = React.useMemo(
		() =>
			[
				selectedAddressId ?? "",
				selectedShipping?.shippingMethod ?? "",
				selectedShipping?.deliveryTemp ?? "",
				checkoutItems
					.map((it) => `${it.skuId ?? it.productId}x${it.quantity}`)
					.join(","),
			].join("|"),
		[selectedAddressId, selectedShipping, checkoutItems],
	);
	React.useEffect(() => {
		void preciseFeeKey;
		setPreciseShippingFee(null);
	}, [preciseFeeKey, setPreciseShippingFee]);

	React.useEffect(() => {
		// Invariant: effect 不打 fetch 時 shippingFeeLoading 必為 false。任何
		// 早期 return 都要先清掉 loading,否則前一輪 in-flight fetch 被
		// cancelled 守衛攔下後,UI 會永卡「計算中」。新增 early-return 條件
		// 時也必須沿用此 invariant。
		if (
			!isHomeDelivery ||
			!selectedAddress ||
			!selectedShipping ||
			selectedItems.length === 0 ||
			preciseShippingFee != null
		) {
			setShippingFeeLoading(false);
			return;
		}
		const zipCode = addressZipCode(selectedAddress);
		if (!zipCode) {
			setShippingFeeLoading(false);
			return;
		}
		let cancelled = false;
		setShippingFeeLoading(true);
		void calculateShippingFee({
			items: checkoutItems,
			shippingMethod: selectedShipping.shippingMethod,
			deliveryTemp: selectedShipping.deliveryTemp ?? "",
			receiverZipCode: zipCode,
		})
			.then((r) => {
				if (cancelled) return;
				setPreciseShippingFee(r.shippingFee);
			})
			.catch(() => {
				// Stick with estimate; surface only when checkout fails.
			})
			.finally(() => {
				if (!cancelled) setShippingFeeLoading(false);
			});
		return () => {
			cancelled = true;
		};
	}, [
		isHomeDelivery,
		selectedAddress,
		selectedShipping,
		selectedItems.length,
		preciseShippingFee,
		checkoutItems,
		setPreciseShippingFee,
		setShippingFeeLoading,
	]);

	/* ───── ErrorCode-specific reactions ───── */

	const reportedErrorRef = React.useRef<string>("");
	React.useEffect(() => {
		const errs = shippingState.validationErrors;
		if (errs.length === 0) return;
		const codes = errs.map((e) => e.errorCode);
		const sig = `${codes.join(",")}|${errs[0].errorMessage}`;
		if (reportedErrorRef.current === sig) return;
		reportedErrorRef.current = sig;
		const first = errs[0];
		const isStock = first.errorCode === "4001" || first.errorCode === "4002";
		const isOffShelf = first.errorCode === "4003";
		const isSelfPurchase = first.errorCode === "4015";
		const fallback = isStock
			? "部分商品庫存不足，請調整數量"
			: isOffShelf
				? "部分商品已下架，請重新確認購物車"
				: isSelfPurchase
					? "不可購買自己上架的商品"
					: "商品驗證失敗，請重新確認";
		emitCartToast({
			message: first.errorMessage || fallback,
			tone: "error",
			actionLabel: "查看購物車",
			actionHref: "/cart",
		});
		// Auto-reload cart for stock/off-shelf/self-purchase resolutions.
		if (isStock || isOffShelf || isSelfPurchase) {
			void cart.refresh();
		}
	}, [shippingState.validationErrors, cart.refresh]);

	// 46 §7.2：賣家停權（9241）是硬擋整個 validate 呼叫的 thrown error，不會進
	// 上面的 validationErrors[] 陣列，所以獨立一支 effect 處理。「已自結帳移除」
	// 是既定文案；實際上目前僅擋下這次驗證（backend 尚無 cart 列表級即時反映機
	// 制，見 CartProductTile 的 DELISTED fallback），refresh() 讓商品下架連動
	// （若已跑完）在購物車立刻可見。
	const reportedSuspensionRef = React.useRef(false);
	React.useEffect(() => {
		if (shippingState.errorCode !== 9241) {
			reportedSuspensionRef.current = false;
			return;
		}
		if (reportedSuspensionRef.current) return;
		reportedSuspensionRef.current = true;
		emitCartToast({
			message: "部分商品暫停販售，已自結帳移除",
			tone: "error",
			actionLabel: "查看購物車",
			actionHref: "/cart",
		});
		void cart.refresh();
	}, [shippingState.errorCode, cart.refresh]);

	/* ───── Cold-chain warning ───── */

	const coldChainConflict = React.useMemo(() => {
		// If validate returned valid=false AND temp intersection is empty AND
		// the buyer has selected items with multiple temp classes, we surface
		// a "can't ship together" hint.
		if (shippingState.loading) return null;
		const temps = new Set<string>();
		for (const it of selectedPurchasableItems) {
			for (const t of it.deliveryTemps ?? []) temps.add(t);
		}
		const intersection = shippingState.deliveryTempIntersection;
		if (
			temps.size > 1 &&
			shippingOptions.length === 0 &&
			(intersection.length === 0 || intersection.length < temps.size)
		) {
			return Array.from(temps);
		}
		return null;
	}, [
		selectedPurchasableItems,
		shippingState.loading,
		shippingOptions.length,
		shippingState.deliveryTempIntersection,
	]);

	/* ───── Cart-context coupons ───── */

	const cartCoupons = useCartCoupons({
		storeId: activeSellerKey,
		orderAmount: itemsSubtotal,
	});
	const usableCouponCount = React.useMemo(
		() =>
			cartCoupons.coupons.filter(
				(candidate) =>
					getCartCouponAvailability(candidate, {
						orderAmount: itemsSubtotal,
						shippingFee,
					}).usable,
			).length,
		[cartCoupons.coupons, itemsSubtotal, shippingFee],
	);

	// Reconcile applied coupon when seller changes / threshold lost.
	const activeCoupon = React.useMemo(() => {
		if (!coupon) return null;
		const match = cartCoupons.coupons.find(
			(c) =>
				(c.templateId && coupon.templateId === c.templateId) ||
				(c.instanceId && coupon.instanceId === c.instanceId),
		);
		if (!match) return null;
		return getCartCouponAvailability(match, {
			orderAmount: itemsSubtotal,
			shippingFee,
		}).usable
			? match
			: null;
	}, [coupon, cartCoupons.coupons, itemsSubtotal, shippingFee]);

	const discount = React.useMemo(() => {
		if (!activeCoupon) return 0;
		return activeCoupon.estimatedDiscount;
	}, [activeCoupon]);

	const grandTotal = Math.max(0, itemsSubtotal + shippingFee - discount);

	const shippingFeeLabel =
		selectedPurchasableItems.length === 0
			? "—"
			: shippingState.loading || shippingFeeLoading
				? "計算中…"
				: shippingFee !== 0
					? ntFormat(shippingFee)
					: "免運";

	/* ───── Handlers ───── */

	const selectionIsBusy = () =>
		clearCartPendingRef.current ||
		rowPendingRef.current.size > 0 ||
		cart.refreshing;

	const toggleGroup = (sellerKey: string, next: boolean) => {
		if (selectionIsBusy()) return;
		if (next && activeSellerKey !== sellerKey) clearCoupon();
		setSelected((prev) => selectCartSeller(cart.items, prev, sellerKey, next));
	};

	const toggleItem = (item: CartItem, next: boolean) => {
		if (selectionIsBusy()) return;
		if (next && activeSellerKey !== cartSellerKeyOf(item)) clearCoupon();
		setSelected((prev) => selectCartItem(cart.items, prev, item, next));
	};

	const onChangeQty = (id: string, raw: string) => {
		// Draft only — the mutation happens on commit (blur / Enter).
		// Mutating per keystroke turned typing "15" into qty 1 then 15, and
		// an emptied field into qty 0, which the cart API treats as REMOVE
		// (the buyer just selected-all to retype the number).
		setEditingQty((m) => ({ ...m, [id]: raw }));
	};

	const commitQty = async (id: string) => {
		const raw = editingQty[id];
		setEditingQty((m) => {
			const next = { ...m };
			delete next[id];
			return next;
		});
		if (raw == null) return;
		const item = cart.items.find((it) => it.id === id);
		if (!item) return;
		const parsed = Math.floor(Number(raw));
		// Empty/garbage/zero input reverts to the current quantity (the draft
		// is cleared above, so the field snaps back). Removal stays on the
		// explicit 移除/stepper paths, which confirm first.
		if (raw.trim() === "" || !Number.isFinite(parsed) || parsed < 1) return;
		const max = item.stockQty != null ? item.stockQty : 999;
		if (max < 1) return;
		const clamped = Math.min(parsed, Math.min(999, max));
		if (clamped !== item.quantity) await cart.updateQty(id, clamped);
		if (clamped < parsed) {
			emitCartToast({
				message: `「${item.name}」目前僅剩 ${clamped} 件`,
				actionLabel: "知道了",
				actionHref: "/cart",
			});
		}
	};

	const directUpdateQty = async (item: CartItem, n: number) => {
		const max = item.stockQty != null ? item.stockQty : 999;
		const clamped = Math.max(0, Math.min(max, n));
		if (clamped === 0) {
			// Stepper "−" at qty 1: confirm like the explicit 移除 button —
			// a silent delete here was inconsistent and easy to fat-finger.
			if (!window.confirm(`確定要從購物車移除「${item.name}」？`)) return;
			trackRemoveFromCart(item);
			await cart.remove(item.id);
			return;
		}
		await cart.updateQty(item.id, clamped);
		if (clamped !== n) {
			emitCartToast({
				message: `「${item.name}」目前僅剩 ${clamped} 件`,
				actionLabel: "知道了",
				actionHref: "/cart",
			});
		}
	};

	const handleSaveForLater = async (item: CartItem) => {
		const outcome = await cart.remove(item.id);
		if (!outcome.ok) return;
		saveItem(item);
		setSaved(getSavedItems());
	};

	const handleMoveBackToCart = async (item: CartItem) => {
		if (savedRestorePendingRef.current.has(item.id)) return;
		savedRestorePendingRef.current.add(item.id);
		setSavedRestorePendingById((current) => ({
			...current,
			[item.id]: true,
		}));
		try {
			const outcome = await cart.add({
				id: item.id,
				skuId: item.skuId,
				productId: item.productId,
				name: item.name,
				price: item.price,
				image: item.image,
				unit: item.unit,
				href: item.href,
				status: item.status,
				sellerName: item.sellerName,
				sellerStoreId: item.sellerStoreId,
				sellerLogoUrl: item.sellerLogoUrl,
				quantity: item.quantity,
			});
			if (outcome.ok) {
				removeSavedItem(item.id);
				setSaved(getSavedItems());
			}
		} finally {
			savedRestorePendingRef.current.delete(item.id);
			setSavedRestorePendingById((current) => {
				const next = { ...current };
				delete next[item.id];
				return next;
			});
		}
		// useCart 已 surfaceError 處理失敗訊息
	};

	const runRowOperation = React.useCallback(
		async (
			skuId: string,
			action: CartRowPendingAction,
			operation: () => unknown,
		) => {
			if (clearCartPendingRef.current || rowPendingRef.current.has(skuId))
				return;
			rowPendingRef.current.set(skuId, action);
			setRowPendingBySku((current) => ({ ...current, [skuId]: action }));
			try {
				await operation();
			} finally {
				rowPendingRef.current.delete(skuId);
				setRowPendingBySku((current) => {
					const next = { ...current };
					delete next[skuId];
					return next;
				});
			}
		},
		[],
	);

	const hasRowPending = Object.keys(rowPendingBySku).length > 0;
	const selectionBusy = clearing || hasRowPending || cart.refreshing;
	const rowOperationCoordinator = React.useMemo(
		() => ({
			pendingBySku: rowPendingBySku,
			blocked: clearing,
			selectionBlocked: selectionBusy,
			run: runRowOperation,
		}),
		[clearing, rowPendingBySku, runRowOperation, selectionBusy],
	);

	const handleClearCart = async () => {
		if (
			clearCartPendingRef.current ||
			rowPendingRef.current.size > 0 ||
			savedRestorePendingRef.current.size > 0 ||
			cart.items.length === 0
		)
			return;
		if (!window.confirm("確定要清空購物車？此動作無法復原。")) return;
		clearCartPendingRef.current = true;
		setClearing(true);
		try {
			const outcome = await cart.clear();
			if (outcome.ok) clearCoupon();
		} finally {
			clearCartPendingRef.current = false;
			setClearing(false);
		}
	};

	const handleApplyCoupon = (target: CartAvailableCoupon) => {
		setCoupon({
			templateId: target.templateId ?? String(target.id),
			instanceId: target.instanceId,
			title: target.title ?? target.name,
			kind: target.kind,
			estimatedDiscount: target.estimatedDiscount,
			storeId: target.applicableStoreId ?? "",
		});
	};

	const handleClearCoupon = () => clearCoupon();

	/* ───── Stale revalidate before checkout ───── */

	const checkoutHref = isLoggedIn ? "/checkout" : "/login?redirect=/checkout";
	// 🔴 未知配送方式一律 fail-closed。舊寫法三元的最後一支是
	// `: true` —— 語意等於「沒見過的配送方式就不用選收件地址也不用選門市」，
	// 於是 3004 大宗寄倉／3005 商家自送可以直接進結帳。
	const addressReady = isShippingDestinationReady(shippingMethod, {
		hasAddress: selectedAddress != null,
		hasStorePickup: selectedPickup != null,
	});
	// `addressReady` 為 false 的三種原因各有自己的話術。未知配送方式**不能**
	// 借用「請選擇收件地址」—— 那句話會叫使用者去做一件解決不了問題的事。
	//
	// 🔴 話術與開哪個選取器都錨在 `shippingDestinationKind`，**不再各自用
	// `isHomeDelivery`／`isStorePickup` 推一次**：那兩個旗標是「是不是黑貓／7-11」，
	// 而這裡問的是「要哪一種收件目標」。
	const destinationKind = shippingDestinationKind(shippingMethod);
	const addressPromptLabel =
		destinationKind === "store-pickup"
			? "請選擇取貨門市"
			: destinationKind === "address"
				? "請選擇收件地址"
				: "此配送方式尚未開放，請改選其他配送方式";
	const canCheckout =
		isLoggedIn &&
		selectedPurchasableItems.length > 0 &&
		!selectionBusy &&
		selectedShipping != null &&
		addressReady &&
		!shippingState.loading &&
		!shippingState.error &&
		!shippingFeeLoading &&
		!selectedPurchasableItems.some((it) => !it.skuId || it.productId == null);

	const writeFrozenCheckoutDraft = React.useCallback(() => {
		if (!activeSellerKey || !selectedShipping) return false;
		// 收件／取貨目標齊備才寫草稿。`addressReady` 對未知配送方式回 false，
		// 所以這裡不會再放行一張「既無地址也無門市」的草稿到結帳頁。
		if (!addressReady) return false;
		const firstItem = selectedPurchasableItems[0];
		if (!firstItem) return false;
		writeCheckoutDraft({
			version: 2,
			createdAt: Date.now(),
			validateAt: lastValidateAt,
			seller: {
				key: activeSellerKey,
				name: firstItem.sellerName ?? "未指定賣家",
				logoUrl: firstItem.sellerLogoUrl,
			},
			selectedSkuIds: selectedPurchasableItems.map((it) => it.skuId),
			shipping: {
				method: selectedShipping.shippingMethod,
				deliveryTemp: selectedShipping.deliveryTemp ?? "",
				label: selectedShipping.label || "運送方式",
				fee: shippingFee,
			},
			recipient:
				// 判準是「這個配送方式要不要地址」，不是「它是不是黑貓」。
				destinationKind === "address" && selectedAddress
					? {
							id: selectedAddress.id,
							name: readStringField(selectedAddress, [
								"receiverName",
								"receiverName",
							]),
							phone: readStringField(selectedAddress, [
								"receiverPhone",
								"receiverPhone",
							]),
							line: addressText(selectedAddress),
							zipCode: addressZipCode(selectedAddress),
						}
					: undefined,
			storePickup:
				isStorePickup && selectedPickup
					? {
							id: selectedPickup.id,
							storeId: selectedPickup.storeId,
							storeName: selectedPickup.storeName,
							storeAddress: selectedPickup.storeAddress,
							receiverName: selectedPickup.receiverName,
							receiverPhone: selectedPickup.receiverPhone,
						}
					: undefined,
			coupon:
				coupon && activeCoupon
					? {
							templateId: coupon.templateId,
							instanceId: coupon.instanceId,
							title: coupon.title,
							kind: String(coupon.kind ?? activeCoupon.kind ?? "discount"),
							estimatedDiscount: discount,
						}
					: undefined,
			note: note.trim(),
		});
		return true;
	}, [
		addressReady,
		activeSellerKey,
		activeCoupon,
		coupon,
		destinationKind,
		discount,
		isStorePickup,
		lastValidateAt,
		note,
		selectedAddress,
		selectedPurchasableItems,
		selectedPickup,
		selectedShipping,
		shippingFee,
	]);

	const handleCheckoutClick = (e: React.MouseEvent<HTMLAnchorElement>) => {
		if (!canCheckout || selectionIsBusy()) {
			e.preventDefault();
			if (selectionBusy) {
				emitCartToast({ message: "購物車更新中，請稍候再試", tone: "error" });
			} else if (selectedPurchasableItems.length === 0) {
				emitCartToast({ message: "請先選取本店可購買的商品", tone: "error" });
			} else if (!selectedShipping) {
				emitCartToast({ message: "請等待運送方式載入", tone: "error" });
			} else if (shippingState.loading || shippingFeeLoading) {
				emitCartToast({
					message: "正在同步物流與運費，請稍候再試",
					tone: "error",
				});
			} else if (shippingState.error) {
				emitCartToast({ message: shippingState.error, tone: "error" });
			} else if (
				selectedPurchasableItems.some((it) => !it.skuId || it.productId == null)
			) {
				emitCartToast({ message: "商品資料同步中，請稍後再試", tone: "error" });
			} else if (!addressReady) {
				emitCartToast({
					message: addressPromptLabel,
					tone: "error",
				});
				// 未知配送方式沒有對應的選取器可開 —— 不猜一個開下去。
				if (destinationKind === "store-pickup") {
					setStorePickupPickerOpen(true);
				} else if (destinationKind === "address") {
					setAddressPickerOpen(true);
				}
			}
			return;
		}
		const stale = Date.now() - lastValidateAt > STALE_VALIDATE_MS;
		if (stale) {
			e.preventDefault();
			emitCartToast({
				message: "正在重新驗證商品資訊，請稍候再試",
				tone: "error",
			});
			requestShippingRefresh();
			return;
		}
		if (!writeFrozenCheckoutDraft()) {
			e.preventDefault();
			emitCartToast({
				message: "結帳資料尚未同步完成，請再試一次",
				tone: "error",
			});
		}
	};

	/* ───── Renders ───── */

	// While auth is still hydrating from cookie/storage, hold the loader so we
	// don't briefly render either the logged-in cart or the guest gate.
	if (!mounted || !authHydrated) {
		return (
			<section className="flex flex-col gap-7 mb-8">
				<div
					className="flex flex-col items-center gap-3 py-[60px] px-5 text-hk-muted font-semibold"
					aria-busy="true"
				>
					<span
						className="size-8 rounded-full border-[3px] border-b-hk-primary-strong/18 border-l-hk-primary-strong/18 border-r-hk-primary-strong/18 border-t-hk-primary-strong animate-spin [animation-duration:0.9s]"
						aria-hidden="true"
					/>
					<p>正在同步購物車…</p>
				</div>
			</section>
		);
	}

	if (!isLoggedIn) {
		return (
			<section className="flex flex-col gap-7 mb-8">
				<div className="flex flex-col gap-3 rounded-[28px] border border-hk-border-soft bg-hk-surface-card p-9 shadow-hk-card">
					<h1 className="m-0 text-hk-heading cart-type-page-title">
						登入後才能查看購物車
					</h1>
					<p className="max-w-[60ch] cart-type-body text-hk-body">
						購物車與優惠券需要登入後才會同步。登入後即可看到上次挑選的食材，並接續結帳。
					</p>
					<div className="mt-1 flex flex-wrap gap-2.5">
						<Link
							href="/login?redirect=/cart"
							hardNavigation
							className="hk-cta-primary inline-flex items-center justify-center text-base leading-6 min-h-[48px] px-[22px] rounded-[var(--radius-pill)] border border-hk-primary bg-hk-primary text-hk-inverse font-semibold no-underline transition-all duration-200 ease-in-out hover:-translate-y-0.5 hover:shadow-hk-card"
						>
							登入
						</Link>
						<Link
							href="/market/products"
							className="inline-flex items-center justify-center text-base leading-6 min-h-[48px] px-[22px] rounded-[var(--radius-pill)] border border-hk-border-soft bg-hk-surface-glass text-hk-heading font-semibold no-underline transition-all duration-200 ease-in-out hover:-translate-y-0.5 hover:shadow-hk-card"
						>
							先逛逛市集
						</Link>
					</div>
				</div>
			</section>
		);
	}

	if (cart.loading && cart.items.length === 0) {
		return (
			<section className="flex flex-col gap-7 mb-8">
				<div
					className="flex flex-col items-center gap-3 py-[60px] px-5 text-hk-muted font-semibold"
					aria-busy="true"
				>
					<span
						className="size-8 rounded-full border-[3px] border-b-hk-primary-strong/18 border-l-hk-primary-strong/18 border-r-hk-primary-strong/18 border-t-hk-primary-strong animate-spin [animation-duration:0.9s]"
						aria-hidden="true"
					/>
					<p>正在同步購物車…</p>
				</div>
			</section>
		);
	}

	if (!cart.loading && cart.items.length === 0) {
		return (
			<section className="flex flex-col gap-7 mb-8">
				<div className="flex flex-col gap-3 rounded-[28px] border border-hk-border-soft bg-hk-surface-card p-9 shadow-hk-card">
					<CartEmptyIcon />
					<h1 className="m-0 text-hk-heading cart-type-page-title">
						購物車是空的
					</h1>
					<p className="max-w-[60ch] cart-type-body text-hk-body">
						還沒有挑到喜歡的食材？從市集、食譜或當季專區開始，找到屬於這週的鮮味。
					</p>
					<div className="mt-1 flex flex-wrap gap-2.5">
						<Link
							href="/market/products"
							className="hk-cta-primary inline-flex items-center justify-center text-base leading-6 min-h-[48px] px-[22px] rounded-[var(--radius-pill)] border border-hk-primary bg-hk-primary text-hk-inverse font-semibold no-underline transition-all duration-200 ease-in-out hover:-translate-y-0.5 hover:shadow-hk-card"
						>
							前往市集
						</Link>
					</div>
				</div>

				{saved.length > 0 ? (
					<SavedForLater
						items={saved}
						onRestore={handleMoveBackToCart}
						restorePendingById={savedRestorePendingById}
						disabled={clearing || hasRowPending}
						onDelete={(id) => {
							removeSavedItem(id);
							setSaved(getSavedItems());
						}}
					/>
				) : null}
			</section>
		);
	}

	return (
		<>
			<section className="flex flex-col gap-7 mb-8">
				<CartPageHeader
					onClear={() => void handleClearCart()}
					clearing={clearing}
					disabled={
						clearing ||
						hasRowPending ||
						Object.keys(savedRestorePendingById).length > 0 ||
						cart.items.length === 0
					}
				/>
				{cart.error ? (
					<p className="hkui-login__hint hkui-login__hint--error" role="alert">
						{cart.error.message}
						<button
							type="button"
							className="cart-page__link-btn"
							style={{ marginLeft: 8 }}
							onClick={() => void cart.refresh()}
						>
							重試
						</button>
					</p>
				) : null}
				{cart.banner ? (
					<p
						className={`hkui-login__hint hkui-login__hint--${cart.banner.kind}`}
						role="status"
					>
						{cart.banner.text}
						<button
							type="button"
							className="cart-page__link-btn"
							style={{ marginLeft: 8 }}
							onClick={cart.dismissBanner}
						>
							知道了
						</button>
					</p>
				) : null}

				<div className="grid grid-cols-[minmax(0,1fr)_minmax(320px,380px)] items-start gap-6 max-[960px]:grid-cols-1">
					<div className="flex flex-col gap-5">
						{coldChainConflict ? (
							<div className="cart-page__cold-warning" role="alert">
								<strong>無法一起出貨</strong>
								<span>
									已選的商品包含
									{coldChainConflict
										.map(
											(t) =>
												SHIPPING_TEMPERATURE_LABEL[
													t as ShippingTemperatureValue
												] ?? t,
										)
										.join(" / ")}
									不同溫層，請分批選購後分次下單。
								</span>
							</div>
						) : null}

						<CartRowOperationContext.Provider value={rowOperationCoordinator}>
							<div className="flex flex-col gap-4">
								{cart.itemsBySeller.map((group) => {
									const groupKey = cartSellerKeyOf(group);
									const isLocked =
										activeSellerKey != null && activeSellerKey !== groupKey;
									return (
										<SellerGroupCard
											key={groupKey}
											group={group}
											groupKey={groupKey}
											isLocked={isLocked}
											isLoggedIn={isLoggedIn}
											selectionBusy={selectionBusy}
											selected={selected}
											preferredMethod={shippingMethod}
											editingQty={editingQty}
											onToggleGroup={toggleGroup}
											onToggleItem={toggleItem}
											onChangeQty={onChangeQty}
											onCommitQty={commitQty}
											onUpdateQty={directUpdateQty}
											onSaveForLater={handleSaveForLater}
											onRemove={async (item) => {
												if (
													window.confirm(`確定要從購物車移除「${item.name}」？`)
												) {
													trackRemoveFromCart(item);
													await cart.remove(item.id);
												}
											}}
										/>
									);
								})}
							</div>
						</CartRowOperationContext.Provider>

						<section className="flex flex-col gap-2 rounded-[22px] border border-hk-border-soft bg-hk-surface-card p-[18px]">
							<label
								htmlFor="cart-note"
								className="flex flex-wrap items-baseline gap-2 text-hk-heading"
							>
								<strong className="cart-type-subtitle">訂單備註</strong>
								<small className="cart-type-support text-hk-body">
									會帶到結帳頁，最多 200 字
								</small>
							</label>
							<textarea
								id="cart-note"
								value={note}
								onChange={(e) => setNote(e.target.value.slice(0, 200))}
								placeholder="例：請避開週末送達 / 樓下管理室代收"
								rows={3}
								maxLength={200}
								className="min-h-[84px] w-full resize-y rounded-hk-md border border-hk-border-soft bg-hk-surface-page px-3 py-2.5 [font:inherit] text-hk-heading focus:border-hk-primary-strong focus:shadow-[0_0_0_3px_rgb(var(--color-brand-primary-rgb)/0.16)] focus:outline-none"
							/>
							<small className="self-end cart-type-support text-hk-body">
								{note.length} / 200
							</small>
						</section>

						<SavedForLater
							items={saved}
							onRestore={handleMoveBackToCart}
							restorePendingById={savedRestorePendingById}
							disabled={clearing || hasRowPending}
							onDelete={(id) => {
								removeSavedItem(id);
								setSaved(getSavedItems());
							}}
						/>
					</div>

					<aside className="sticky top-[calc(var(--nav-height)+24px)] flex flex-col gap-4 rounded-hk-xl border border-hk-border-soft bg-hk-surface-card p-[22px] shadow-hk-card max-[960px]:static">
						<h2 className="m-0 cart-type-section-title text-hk-heading">
							訂單摘要
						</h2>
						<div className="cart-page__checkout-seller" aria-live="polite">
							{selectedItems.length > 0 ? (
								<>
									<strong className="cart-type-subtitle">
										{selectedItems[0].sellerName || "本店"}
									</strong>
									<span className="cart-type-support">
										本次結帳 {selectedCount} 件商品
									</span>
									{selectedItems.length > selectedPurchasableItems.length ? (
										<span className="cart-type-support">
											不可購買的商品不計入結帳，可在店家區塊移除。
										</span>
									) : null}
								</>
							) : (
								<p className="m-0 cart-type-body">請先從一家店勾選商品。</p>
							)}
						</div>

						<section className="rounded-hk-lg border border-hk-border-soft bg-hk-surface-page p-3.5">
							<h3 className="m-0 mb-2.5 cart-type-subtitle text-hk-heading">
								運送方式
							</h3>
							{shippingState.loading && shippingOptions.length === 0 ? (
								<p className="cart-page__summary-foot">計算可用運送方式…</p>
							) : shippingState.validationErrors.length > 0 ? (
								<ul className="cart-page__shipping-errors">
									{shippingState.validationErrors.map((e) => (
										<li key={`${e.errorCode}-${e.errorMessage}`}>
											{e.errorMessage}
										</li>
									))}
								</ul>
							) : null}
							{shippingState.error ? (
								<p
									className="hkui-login__hint hkui-login__hint--error"
									role="alert"
								>
									{shippingState.error}
								</p>
							) : null}
							{shippingOptions.length === 0 ? (
								selectedItems.length === 0 && isLoggedIn ? (
									<div
										className="flex flex-col items-center gap-2.5"
										role="note"
									>
										<p className="cart-page__summary-foot">
											請先從店家區塊勾選商品，才會計算可用運送方式
										</p>
									</div>
								) : (
									<p className="cart-page__summary-foot">
										{!isLoggedIn
											? "登入後可查看可用運送方式"
											: selectedPurchasableItems.some(
														(it) => it.productId == null,
													)
												? "商品資料同步中⋯如持續顯示，請移除後重新加入購物車"
												: "目前選擇的商品沒有合適的運送方式，請調整購物車內容"}
									</p>
								)
							) : (
								<>
									<ShippingMethodGroup
										options={shippingOptions}
										selectedMethod={shippingMethod}
										selectedDeliveryTemp={shippingDeliveryTemp}
										onSelect={setShippingSelection}
										addressLabel={
											selectedAddress ? addressText(selectedAddress) : null
										}
										storePickupLabel={
											selectedPickup
												? `${selectedPickup.storeName} (店號 ${selectedPickup.storeId})`
												: null
										}
										onPickAddress={() => setAddressPickerOpen(true)}
										onPickStorePickup={() => setStorePickupPickerOpen(true)}
										addressRequired={isHomeDelivery}
										storePickupRequired={isStorePickup}
									/>
									{shippingFeeLoading ? (
										<p className="cart-page__summary-foot">計算精準運費中…</p>
									) : null}
								</>
							)}
						</section>

						<section className="rounded-hk-lg border border-hk-border-soft bg-hk-surface-page p-3.5">
							<h3 className="m-0 mb-2.5 cart-type-subtitle text-hk-heading">
								優惠券
							</h3>
							{!isLoggedIn ? (
								<p className="cart-page__summary-foot">
									<Link href="/login?redirect=/cart" hardNavigation>
										登入
									</Link>{" "}
									後可使用優惠券
								</p>
							) : !activeSellerKey ? (
								<p className="cart-page__summary-foot">
									請先勾選同一賣家的商品才能套用優惠券
								</p>
							) : coupon && activeCoupon ? (
								<div className="flex flex-wrap items-center justify-between gap-2.5 rounded-hk-md border border-hk-primary-strong bg-[rgb(var(--color-brand-primary-rgb)/0.06)] px-3 py-2.5 text-hk-heading">
									<div>
										<strong className="block cart-type-subtitle">
											{coupon.title}
										</strong>
										<small className="cart-type-support text-hk-muted">
											{activeCoupon.sellerName || "全平台券"}
										</small>
									</div>
									<div className="inline-flex items-center gap-2.5">
										<b className="font-semibold text-[#c0392b]">
											{`−${ntFormat(discount)}`}
										</b>
										<button
											type="button"
											className="cart-page__link-btn"
											onClick={() => setCouponSheetOpen(true)}
										>
											更換
										</button>
										<button
											type="button"
											className="cart-page__link-btn cart-page__link-btn--danger"
											onClick={handleClearCoupon}
										>
											取消
										</button>
									</div>
								</div>
							) : cartCoupons.loading && cartCoupons.coupons.length === 0 ? (
								<p className="cart-page__summary-foot">載入優惠券中…</p>
							) : cartCoupons.coupons.length === 0 ? (
								<p className="cart-page__summary-foot">
									目前沒有可用優惠券，去{" "}
									<Link href="/account/coupons">領券中心</Link> 看看
								</p>
							) : (
								<button
									type="button"
									className="cart-page__coupon-trigger"
									onClick={() => setCouponSheetOpen(true)}
								>
									選擇優惠券（{usableCouponCount} 張可用）
								</button>
							)}
							{cartCoupons.error ? (
								<p
									className="hkui-login__hint hkui-login__hint--error"
									role="alert"
								>
									{cartCoupons.error}
								</p>
							) : null}
						</section>

						<dl className="m-0 flex flex-col gap-1.5 pt-1.5">
							<div className="flex items-baseline justify-between gap-3">
								<dt className="m-0 cart-type-body text-hk-body">
									商品小計（{selectedCount} 件）
								</dt>
								<dd className="m-0 cart-type-amount text-hk-heading">
									{ntFormat(itemsSubtotal)}
								</dd>
							</div>
							<div className="flex items-baseline justify-between gap-3">
								<dt className="m-0 cart-type-body text-hk-body">
									運費
									{isHomeDelivery && preciseShippingFee == null ? (
										<small className="cart-type-support"> 估算</small>
									) : null}
								</dt>
								<dd className="m-0 cart-type-amount text-hk-heading">
									{shippingFeeLabel}
								</dd>
							</div>
							{discount > 0 ? (
								<div className="flex items-baseline justify-between gap-3">
									<dt className="m-0 cart-type-body text-hk-body">折抵</dt>
									<dd className="m-0 cart-type-amount text-[#c0392b]">
										−{ntFormat(discount)}
									</dd>
								</div>
							) : null}
							<div className="mt-2 flex items-baseline justify-between gap-3 border-t border-dashed border-hk-border-soft pt-3">
								<dt className="m-0 cart-type-subtitle text-hk-heading">
									應付金額
								</dt>
								<dd className="m-0 cart-type-total text-hk-primary-strong">
									{shippingState.loading || shippingFeeLoading
										? "計算中…"
										: ntFormat(grandTotal)}
								</dd>
							</div>
						</dl>

						<Link
							href={checkoutHref}
							className={`hk-cta-primary inline-flex items-center justify-center text-base leading-6 min-h-[48px] px-[22px] rounded-[var(--radius-pill)] border border-hk-primary bg-hk-primary text-hk-inverse font-semibold no-underline transition-all duration-200 ease-in-out hover:-translate-y-0.5 hover:shadow-hk-card cart-page__checkout${
								canCheckout ? "" : " cart-page__checkout--disabled"
							}`}
							aria-disabled={!canCheckout}
							onClick={handleCheckoutClick}
						>
							{isLoggedIn
								? selectedPurchasableItems.length === 0
									? "請先選取可購買商品"
									: selectionBusy
										? "購物車更新中"
										: selectedPurchasableItems.some(
													(it) => !it.skuId || it.productId == null,
												)
											? "商品資料同步中"
											: shippingState.loading || shippingFeeLoading
												? "物流計算中"
												: shippingState.error
													? "物流驗證失敗"
													: !selectedShipping
														? "請選擇運送方式"
														: !addressReady
															? addressPromptLabel
															: `結帳本店商品（${selectedCount} 件）`
								: "登入後結帳"}
						</Link>

						<p className="cart-page__summary-foot">
							下單即代表你同意 <Link href="/terms">服務條款</Link> 與
							<Link href="/return-policy">退換貨政策</Link>。
						</p>
					</aside>
				</div>
			</section>

			<AddressPickerSheet
				open={addressPickerOpen}
				addresses={addresses.addresses}
				selectedId={selectedAddressId}
				loading={addresses.loading}
				onSelect={(id) => setSelectedAddressId(id)}
				onClose={() => setAddressPickerOpen(false)}
			/>

			<StorePickupPickerSheet
				open={storePickupPickerOpen}
				pickups={storePickups.pickups}
				selectedId={selectedPickupId}
				loading={storePickups.loading}
				error={storePickups.error}
				onSelect={(id) => setSelectedPickupId(id)}
				onCreate={async (body) => {
					await storePickups.create(body);
				}}
				onClose={() => setStorePickupPickerOpen(false)}
			/>

			<CouponSelectionSheet
				open={couponSheetOpen}
				coupons={cartCoupons.coupons}
				selectedTemplateId={coupon?.templateId ?? null}
				orderAmount={itemsSubtotal}
				shippingFee={shippingFee}
				loading={cartCoupons.loading}
				error={cartCoupons.error}
				onSelect={(c) => {
					handleApplyCoupon(c);
					setCouponSheetOpen(false);
				}}
				onClear={handleClearCoupon}
				onRetry={() => void cartCoupons.refresh()}
				onClose={() => setCouponSheetOpen(false)}
			/>
		</>
	);
}

function SavedForLater({
	items,
	onRestore,
	onDelete,
	restorePendingById,
	disabled,
}: {
	items: CartItem[];
	onRestore: (item: CartItem) => Promise<void>;
	onDelete: (id: string) => void;
	restorePendingById: Readonly<Record<string, boolean | undefined>>;
	disabled?: boolean;
}) {
	if (items.length === 0) return null;
	return (
		<section className="flex flex-col gap-3 rounded-[22px] border border-hk-border-soft bg-hk-surface-card p-[18px]">
			<header className="flex flex-col gap-0.5">
				<h2 className="m-0 cart-type-section-title text-hk-heading">
					稍後購買 ({items.length})
				</h2>
				<small className="cart-type-support text-hk-muted">
					暫時收起來的食材，需要時可以一鍵加回購物車
				</small>
			</header>
			<ul className="m-0 flex list-none flex-col gap-2.5 p-0">
				{items.map((item) => (
					<li
						key={item.id}
						className="grid grid-cols-[64px_minmax(0,1fr)_auto] items-center gap-3 rounded-[14px] border border-hk-border-soft bg-hk-surface-page p-2.5 max-[560px]:grid-cols-[56px_minmax(0,1fr)]"
					>
						<div className="flex size-16 items-center justify-center overflow-hidden rounded-hk-md bg-hk-surface-card">
							{item.image ? (
								// eslint-disable-next-line @next/next/no-img-element
								// biome-ignore lint/performance/noImgElement: backend cart thumbnail
								<img
									src={ossImageUrl(item.image, "productThumb")}
									alt={item.name}
									loading="eager"
									className="size-full object-cover"
								/>
							) : (
								<AppImage
									src=""
									alt={item.name}
									width={64}
									height={64}
									loading="eager"
									className="size-full"
								/>
							)}
						</div>
						<div className="flex min-w-0 flex-col gap-0.5">
							{item.href ? (
								<Link
									href={item.href}
									className="cart-type-subtitle text-hk-heading no-underline hover:text-hk-primary-strong"
								>
									{item.name}
								</Link>
							) : (
								<strong className="cart-type-subtitle text-hk-heading no-underline">
									{item.name}
								</strong>
							)}
							<small className="cart-type-support text-hk-muted">
								{formatUnitLabel(item.unit)}
								{item.sellerName ? `｜${item.sellerName}` : ""}
							</small>
							<b className="cart-type-amount text-hk-primary-strong">
								{item.priceValue
									? `NT$ ${parsePriceValue(item.price).toLocaleString()}`
									: "價格確認中"}
							</b>
						</div>
						<div className="inline-flex flex-col gap-1.5 max-[560px]:col-span-full max-[560px]:flex-row max-[560px]:justify-end">
							<Button
								type="button"
								variant="light"
								color="default"
								size="md"
								className="cart-page__link-btn"
								onClick={() => void onRestore(item)}
								disabled={disabled || restorePendingById[item.id]}
								isLoading={restorePendingById[item.id] === true}
								loadingLabel="加回購物車中…"
							>
								加回購物車
							</Button>
							<Button
								type="button"
								variant="light"
								color="danger"
								size="md"
								className="cart-page__link-btn cart-page__link-btn--danger"
								onClick={() => onDelete(item.id)}
								disabled={disabled || restorePendingById[item.id]}
							>
								刪除
							</Button>
						</div>
					</li>
				))}
			</ul>
		</section>
	);
}

/**
 * Cart icon shown above the empty-cart copy. Same icon as the navbar's
 * `購物車` action so it feels familiar.
 */
function CartEmptyIcon() {
	return (
		<div
			className="mb-1 inline-flex size-[140px] items-center justify-center text-hk-heading"
			aria-hidden="true"
		>
			<AppIcon name="cart" size={140} />
		</div>
	);
}
