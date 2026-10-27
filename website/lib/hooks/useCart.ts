"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import {
	addToCart,
	CART_UPDATED_EVENT,
	CartError,
	type CartInput,
	type CartItem,
	clearCart,
	loadCart,
	parsePriceValue,
	removeFromCart,
	setCartQuantity,
} from "@/lib/cart";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * Informational banner shown when the cart state changes due to background
 * events (e.g. the same buyer just completed checkout on another device).
 * Distinct from `error` (which signals a failed mutation requiring user
 * action) — banner is non-blocking informational.
 */
export interface CartBanner {
	kind: "info" | "warning";
	text: string;
	autoDismissMs?: number;
}

/** Outcome reported back by mutation helpers so callers can react. */
export interface MutationOutcome {
	ok: boolean;
	error?: CartError;
}

interface UseCartResult {
	items: CartItem[];
	totalQuantity: number;
	totalAmount: number;
	loading: boolean;
	/** Any cart query fetch, including a manual/background reconciliation. */
	refreshing: boolean;
	error: CartError | null;
	banner: CartBanner | null;
	dismissBanner: () => void;
	refresh: () => Promise<void>;
	add: (input: CartInput) => Promise<MutationOutcome>;
	updateQty: (skuId: string, n: number) => Promise<MutationOutcome>;
	remove: (skuId: string) => Promise<MutationOutcome>;
	clear: () => Promise<MutationOutcome>;
	itemsBySeller: Array<{
		sellerName: string;
		sellerStoreId: string;
		sellerLogoUrl?: string;
		sellerCity?: string;
		sellerDistrict?: string;
		items: CartItem[];
		subtotal: number;
	}>;
}

/**
 * Cart hook — backend authoritative, member-only.
 *
 * Mount 時以空陣列起始(無 localStorage first-paint cache),立刻 await
 * 後端 `/public/carts` 取得真實 cart 並 setItems。Loading 期間 UI 顯示
 * skeleton,backend RTT 通常 < 100ms 體感無感。對齊 buyer-app 行為,
 * 確保前端顯示永遠是 backend 真相(避免 stale local 被誤帶進 checkout)。
 *
 * 數量 / 新增採樂觀更新:setItems 立刻 predict 結果 → await backend →
 * 成功用 backend 結果覆蓋、失敗 rollback。刪除則保留 row 到 request settle，
 * 讓發起按鈕能完整回報 pending；失敗同樣透過 surfaceError 分流為:
 *   - 6001 CART_ITEM_NOT_FOUND → banner + auto refresh (其他裝置已完成結帳)
 *   - 6003 CART_ITEM_LOCKED → banner + auto refresh (其他裝置結帳處理中)
 *   - 其他 CartError → setError 給 UI 顯示
 *
 * 同 window 內多個 useCart instance(navbar / cart 頁同開時)透過
 * `CART_UPDATED_EVENT` 互通;跨 tab / 跨裝置同步靠 mount await loadCart,
 * 不額外做 push 機制(已知 trade-off,業界電商標準做法)。
 */
export function useCart(): UseCartResult {
	const { isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const cartKey = queryKeys.checkout.cart(identity);

	// Mount with empty state; backend response below populates. Cart 模組
	// 已對齊 buyer-app:沒有 localStorage first-paint cache,避免 stale 資料
	// 被誤當真相帶進 checkout(這是先前 6005 的根因之一)。Backend RTT
	// 通常 < 100ms,loading skeleton 體感無感。
	const [error, setError] = React.useState<CartError | null>(null);
	const [banner, setBanner] = React.useState<CartBanner | null>(null);
	const cartQuery = useQuery({
		queryKey: cartKey,
		queryFn: loadCart,
		enabled: isLoggedIn,
		placeholderData: [] as CartItem[],
		// Transactional state is never a discovery snapshot. Keep cached rows on
		// screen, but reconcile whenever this tab mounts/foregrounds/reconnects so
		// another browser tab or device cannot leave the checkout entry stale.
		staleTime: 0,
		refetchOnMount: "always",
		refetchOnWindowFocus: "always",
		refetchOnReconnect: "always",
	});
	const items = isLoggedIn ? (cartQuery.data ?? []) : [];
	const loading = cartQuery.isLoading;
	const refreshing = cartQuery.isFetching;
	const currentError =
		error ??
		(cartQuery.error instanceof CartError
			? cartQuery.error
			: cartQuery.error
				? new CartError("載入購物車失敗")
				: null);

	const dismissBanner = React.useCallback(() => setBanner(null), []);

	// True while THIS instance is running a mutation. The cart mutation fns
	// broadcast CART_UPDATED on success; without this guard the echo bounces
	// straight back into the listener below and fires a redundant refetch
	// (skeleton flash) over the data onSuccess already wrote. Other instances
	// (navbar / cart page) aren't mutating, so they still sync on the event.
	const mutatingRef = React.useRef(false);

	const refetchCart = cartQuery.refetch;
	const refresh = React.useCallback(async () => {
		setError(null);
		await refetchCart();
	}, [refetchCart]);

	// Cross-tab / cross-instance sync: a mutation elsewhere → re-fetch backend.
	React.useEffect(() => {
		if (typeof window === "undefined") return;
		const onUpdated = () => {
			if (mutatingRef.current) return; // skip our own mutation's echo
			void refresh();
		};
		window.addEventListener(CART_UPDATED_EVENT, onUpdated);
		return () => window.removeEventListener(CART_UPDATED_EVENT, onUpdated);
	}, [refresh]);

	/**
	 * Map a CartError to the right surface: banner for informational
	 * background events, redirect/silent for auth, toast/state for the rest.
	 * Returns the (possibly enriched) error so callers can decide whether
	 * to keep it or treat it as already-handled.
	 */
	const surfaceError = React.useCallback(
		(err: CartError): CartError => {
			// 6001 CART_ITEM_NOT_FOUND — typically means the row was cleaned up
			// by another tab/device that completed checkout. Treat as info.
			if (err.code === 6001) {
				setBanner({
					kind: "info",
					text: "您剛在其他地方完成結帳,購物車已更新",
					autoDismissMs: 5000,
				});
				void refresh();
				return err;
			}
			// 6003 CART_ITEM_LOCKED — same buyer is mid-confirm somewhere. The
			// confirm gRPC is server-side sync (< 5s) and the lock auto-expires
			// after 2 min (CartService.LOCK_TTL_MINUTES). No "cancel" UI exists.
			if (err.code === 6003) {
				setBanner({
					kind: "info",
					text: "商品結帳處理中,請稍候片刻後再試",
					autoDismissMs: 5000,
				});
				void refresh();
				return err;
			}
			// Real errors that need user attention.
			setError(err);
			return err;
		},
		[refresh],
	);

	// Optimistic cart mutation as a real useMutation: onMutate cancels in-flight
	// refetches (so a late GET can't clobber the optimistic write), snapshots the
	// cache for rollback, and predicts the result; onSuccess overwrites with the
	// backend truth; onError rolls back. `predict`/`call` ride in as variables so
	// add / updateQty / remove / clear can share one mutation.
	const cartMutation = useMutation({
		mutationFn: ({
			call,
		}: {
			predict: (prev: CartItem[]) => CartItem[];
			call: () => Promise<CartItem[]>;
		}) => call(),
		onMutate: async ({ predict }) => {
			await queryClient.cancelQueries({ queryKey: cartKey });
			const snapshot = queryClient.getQueryData<CartItem[]>(cartKey) ?? [];
			queryClient.setQueryData<CartItem[]>(cartKey, predict(snapshot));
			setError(null);
			return { snapshot };
		},
		onError: (_e, _v, ctx) => {
			if (ctx) queryClient.setQueryData<CartItem[]>(cartKey, ctx.snapshot);
		},
		onSuccess: (fresh) => {
			queryClient.setQueryData<CartItem[]>(cartKey, fresh);
		},
	});
	const { mutateAsync: mutateCart } = cartMutation;

	/** Adapter to the MutationOutcome + error-surfacing contract callers expect;
	 *  also flips the self-echo guard around the mutation. */
	const runMutation = React.useCallback(
		async (
			predict: (prev: CartItem[]) => CartItem[],
			call: () => Promise<CartItem[]>,
		): Promise<MutationOutcome> => {
			mutatingRef.current = true;
			try {
				await mutateCart({ predict, call });
				return { ok: true };
			} catch (e) {
				const err = e instanceof CartError ? e : new CartError("操作失敗");
				return { ok: false, error: surfaceError(err) };
			} finally {
				mutatingRef.current = false;
			}
		},
		[mutateCart, surfaceError],
	);

	const add = React.useCallback(
		async (input: CartInput) => {
			const skuId = String(input.skuId ?? input.id);
			const qty = Math.max(1, Number(input.quantity || 1));
			return runMutation(
				(prev) => {
					const existing = prev.find((it) => it.skuId === skuId);
					if (existing) {
						return prev.map((it) =>
							it.skuId === skuId ? { ...it, quantity: it.quantity + qty } : it,
						);
					}
					return [
						...prev,
						{
							id: skuId,
							skuId,
							productId: input.productId,
							name: input.name,
							price: input.price ?? "0",
							priceValue: parsePriceValue(input.price),
							quantity: qty,
							status: input.status,
							image: input.image,
							unit: input.unit,
							sellerName: input.sellerName,
							sellerStoreId: input.sellerStoreId,
							sellerLogoUrl: input.sellerLogoUrl,
							href: input.href,
						},
					];
				},
				() => addToCart(input),
			);
		},
		[runMutation],
	);

	const updateQty = React.useCallback(
		async (skuId: string, n: number) => {
			const next = Math.max(0, Number(n || 0));
			return runMutation(
				(prev) =>
					next === 0
						? prev.filter((it) => it.skuId !== skuId)
						: prev.map((it) =>
								it.skuId === skuId ? { ...it, quantity: next } : it,
							),
				() => setCartQuantity(skuId, next),
			);
		},
		[runMutation],
	);

	const remove = React.useCallback(
		async (skuId: string) =>
			runMutation(
				// Keep the initiating row mounted until the DELETE settles so its
				// branded pending button remains visible. The backend response still
				// replaces the cache on success; failures retain the same snapshot.
				(prev) => prev,
				() => removeFromCart(skuId),
			),
		[runMutation],
	);

	// Clearing is a destructive whole-cart mutation. Keep the in-flight promise
	// in a ref so two clicks in the same render frame can never issue two DELETEs.
	const clearPendingRef = React.useRef<Promise<MutationOutcome> | null>(null);
	const clearAll = React.useCallback(() => {
		if (clearPendingRef.current) return clearPendingRef.current;
		const pending = runMutation(
			() => [],
			() => clearCart(),
		).finally(() => {
			if (clearPendingRef.current === pending) clearPendingRef.current = null;
		});
		clearPendingRef.current = pending;
		return pending;
	}, [runMutation]);

	const totalQuantity = React.useMemo(
		() => items.reduce((sum, it) => sum + it.quantity, 0),
		[items],
	);
	const totalAmount = React.useMemo(
		() => items.reduce((sum, it) => sum + it.priceValue * it.quantity, 0),
		[items],
	);

	const itemsBySeller = React.useMemo(() => {
		const map = new Map<
			string,
			{
				sellerName: string;
				sellerStoreId: string;
				sellerStoreCode?: string;
				sellerLogoUrl?: string;
				sellerCity?: string;
				sellerDistrict?: string;
				items: CartItem[];
				subtotal: number;
			}
		>();
		for (const it of items) {
			const key = it.sellerStoreId || it.sellerName || "unknown";
			const entry = map.get(key);
			if (entry) {
				entry.items.push(it);
				entry.subtotal += it.priceValue * it.quantity;
				if (!entry.sellerStoreCode && it.sellerStoreCode) {
					entry.sellerStoreCode = it.sellerStoreCode;
				}
				if (!entry.sellerLogoUrl && it.sellerLogoUrl) {
					entry.sellerLogoUrl = it.sellerLogoUrl;
				}
				if (!entry.sellerCity && it.sellerCity) {
					entry.sellerCity = it.sellerCity;
				}
				if (!entry.sellerDistrict && it.sellerDistrict) {
					entry.sellerDistrict = it.sellerDistrict;
				}
			} else {
				map.set(key, {
					sellerName: it.sellerName ?? "未指定賣家",
					sellerStoreId: it.sellerStoreId ?? "",
					sellerStoreCode: it.sellerStoreCode,
					sellerLogoUrl: it.sellerLogoUrl,
					sellerCity: it.sellerCity,
					sellerDistrict: it.sellerDistrict,
					items: [it],
					subtotal: it.priceValue * it.quantity,
				});
			}
		}
		return Array.from(map.values());
	}, [items]);

	return {
		items,
		totalQuantity,
		totalAmount,
		loading,
		refreshing,
		error: currentError,
		banner,
		dismissBanner,
		refresh,
		add,
		updateQty,
		remove,
		clear: clearAll,
		itemsBySeller,
	};
}

export { parsePriceValue };
