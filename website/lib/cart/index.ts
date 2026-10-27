/**
 * Cart orchestrator — backend-authoritative, member-only.
 *
 * Single source of truth: cart-service `/public/carts/*` via BFF proxy (auto
 * injects Bearer cookie session). localStorage is no longer used for cart
 * state at all — the previous first-paint cache was the root cause of the
 * 6005 incident:
 *   - 之前 lib/cart/repo 從 localStorage 撈 token,因為 BFF 登入只寫 cookie
 *     不寫 localStorage,cart API 拿不到 token → backend 視為 guest →
 *     sku 寫進 guest cart → checkout 時 backend 用 member account 找不到 → 6005。
 *   - 新版 cart API 走 BFF (authedFetch),server-side 從 cookie 拿 token
 *     注入 Authorization。已登入買家所有 cart 操作一律進 member cart。
 *
 * UX:首次進 cart 頁短暫 loading skeleton(無 first-paint cache),backend
 * RTT < 100ms 體感無感;mutation 用樂觀更新 + 失敗 rollback。
 */

import {
	addCartItem as bAdd,
	removeCartItem as bRemove,
	setCartItemQuantity as bSet,
	fetchCart,
} from "./repo";
import {
	type CartItem as CartItemType,
	flattenGroups,
	parsePriceValue as parsePrice,
} from "./types";

export type CartItem = CartItemType;

/**
 * Input shape for `addToCart`. `id` is required and acts as the SKU id
 * (defaults to `skuId` if both provided). Existing call sites at PDP /
 * AddToCartButton already supply this shape.
 */
export type CartInput = {
	id: string;
	skuId?: string;
	productId?: string | number;
	name: string;
	price?: string;
	image?: string;
	unit?: string;
	href?: string;
	status?: string;
	sellerName?: string;
	sellerStoreId?: string;
	sellerLogoUrl?: string;
	quantity?: number;
};

/** Cross-tab change signal. Other tabs listen and re-fetch backend. */
export const CART_UPDATED_EVENT = "hawkr:cart-updated";

/** Typed error surfaced from any cart mutation / load failure. */
export class CartError extends Error {
	constructor(
		message: string,
		public code?: number,
		public status?: number,
		public payload?: unknown,
	) {
		super(message);
		this.name = "CartError";
	}
}

function extractStatus(err: unknown): number | undefined {
	if (err && typeof err === "object" && "status" in err) {
		const v = (err as { status?: unknown }).status;
		return typeof v === "number" ? v : undefined;
	}
	return undefined;
}

function extractCode(err: unknown): number | undefined {
	if (err && typeof err === "object" && "code" in err) {
		const v = (err as { code?: unknown }).code;
		return typeof v === "number" ? v : undefined;
	}
	return undefined;
}

function wrapApiError(err: unknown, fallbackMessage: string): CartError {
	return new CartError(
		err instanceof Error ? err.message : fallbackMessage,
		extractCode(err),
		extractStatus(err),
		err,
	);
}

function broadcastCartChanged() {
	if (typeof window === "undefined") return;
	window.dispatchEvent(new CustomEvent(CART_UPDATED_EVENT));
}

/**
 * Fetch the authoritative cart from the backend. Throws `CartError` on any
 * failure (network / 401 / 5xx) — caller must surface or retry; never falls
 * back to stale local data.
 */
export async function loadCart(): Promise<CartItem[]> {
	try {
		const summary = await fetchCart();
		return flattenGroups(summary.groups);
	} catch (err) {
		throw wrapApiError(err, "載入購物車失敗");
	}
}

/** Add a SKU to the cart. Returns the fresh backend snapshot. */
export async function addToCart(input: CartInput): Promise<CartItem[]> {
	const skuId = String(input.skuId ?? input.id);
	const quantity = Math.max(1, Number(input.quantity || 1));
	try {
		await bAdd({ skuId, quantity });
	} catch (err) {
		throw wrapApiError(err, "加入購物車失敗");
	}
	const fresh = await loadCart();
	broadcastCartChanged();
	return fresh;
}

/** Update quantity (qty=0 removes the item). */
export async function setCartQuantity(
	skuId: string,
	quantity: number,
): Promise<CartItem[]> {
	const next = Math.max(0, Number(quantity || 0));
	try {
		if (next === 0) {
			await bRemove(skuId);
		} else {
			await bSet(skuId, next);
		}
	} catch (err) {
		throw wrapApiError(err, "更新購物車失敗");
	}
	const fresh = await loadCart();
	broadcastCartChanged();
	return fresh;
}

/** Remove a SKU from the cart. */
export async function removeFromCart(skuId: string): Promise<CartItem[]> {
	try {
		await bRemove(skuId);
	} catch (err) {
		throw wrapApiError(err, "移除商品失敗");
	}
	const fresh = await loadCart();
	broadcastCartChanged();
	return fresh;
}

/** Clear everything in the cart. */
export async function clearCart(): Promise<CartItem[]> {
	const current = await loadCart();
	if (current.length === 0) {
		return current;
	}
	// No bulk endpoint — fan-out delete. Settle all then re-fetch.
	await Promise.allSettled(current.map((it) => bRemove(it.skuId)));
	const fresh = await loadCart();
	broadcastCartChanged();
	return fresh;
}

export const parsePriceValue = parsePrice;
