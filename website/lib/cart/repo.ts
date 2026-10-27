/**
 * Cart HTTP repository — 對齊 buyer-app 的 `/public/carts/*` 路徑。
 *
 * Web 與 buyer-app 共用同一組 backend endpoint:
 *   - buyer-app:dio.{get,post,put,delete}('/public/carts/...')
 *     + dio interceptor 自動注入 Authorization Bearer
 *   - web:authedFetch('/public/carts/...') → BFF proxy 自動從 cookie
 *     讀 token 注入 Authorization Bearer
 * 兩端打到的 backend URL 完全一致 (`/api/v1/public/carts/...`),認證模式
 * 一致 (OptionalJWT + 帶 Bearer → 視為 member)。
 *
 * Backend 為 OptionalJWTAuth:有 JWT → member;無 JWT + guest_id →
 * guest。Web 端透過 BFF 一定帶 JWT,所以一律走 member 路徑,寫入
 * member cart_items,不會誤入 guest cart 觸發 6005 CHECKOUT_LOCK_FAILED。
 */

import { authedFetch } from "../api/client";
import type { CartSummary } from "./types";

/* ===== Reads ===== */

export async function fetchCart(): Promise<CartSummary> {
	const env = await authedFetch<CartSummary | { groups?: never }>(
		"/public/carts",
		{ method: "GET" },
	);
	const data = env.data as CartSummary | undefined;
	return {
		groups: data?.groups ?? [],
		totalItems: data?.totalItems ?? 0,
		estimatedTotalAmount: data?.estimatedTotalAmount ?? "0",
	};
}

/* ===== Writes ===== */

export interface AddCartItemBody {
	skuId: string;
	quantity: number;
}

export async function addCartItem({
	skuId,
	quantity,
}: AddCartItemBody): Promise<{ totalItems: number }> {
	const env = await authedFetch<{ totalItems?: string | number }>(
		"/public/carts/items",
		{
			method: "POST",
			// OpenAPI: handler.AddToCartRequest quantity is a string.
			json: { skuId, quantity: String(quantity) },
		},
	);
	return { totalItems: Number(env.data?.totalItems ?? 0) };
}

export async function setCartItemQuantity(
	skuId: string,
	quantity: number,
): Promise<{ totalItems: number }> {
	const env = await authedFetch<{ totalItems?: string | number }>(
		`/public/carts/items/${encodeURIComponent(skuId)}/quantity`,
		{
			method: "PUT",
			// OpenAPI: handler.SetItemQuantityRequest quantity is a string.
			json: { quantity: String(quantity) },
		},
	);
	return { totalItems: Number(env.data?.totalItems ?? 0) };
}

export async function removeCartItem(
	skuId: string,
): Promise<{ totalItems: number }> {
	const env = await authedFetch<{ totalItems?: number }>(
		`/public/carts/items/${encodeURIComponent(skuId)}`,
		{ method: "DELETE" },
	);
	return { totalItems: env.data?.totalItems ?? 0 };
}
