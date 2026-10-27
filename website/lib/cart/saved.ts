/**
 * Save-for-later (稍後購買) — browser-local list of items the buyer
 * temporarily parked from the cart. No backend endpoint exists today, so
 * this lives in localStorage and is mirrored across tabs via the native
 * storage event. Same-tab updates are applied by the caller re-reading
 * getSavedItems() after each mutation.
 */

import { LOCAL_PRIVATE_KEYS } from "@/lib/auth/local-private-keys";
import { type CartItem, parsePriceValue } from "./types";

const SAVED_STORAGE_KEY = LOCAL_PRIVATE_KEYS.savedItems.key;

function isBrowser() {
	return typeof window !== "undefined" && typeof localStorage !== "undefined";
}

function read(): CartItem[] {
	if (!isBrowser()) return [];
	try {
		const parsed = JSON.parse(localStorage.getItem(SAVED_STORAGE_KEY) || "[]");
		if (!Array.isArray(parsed)) return [];
		return parsed.map((p: Record<string, unknown>) => ({
			id: String(p.id ?? p.skuId ?? ""),
			skuId: String(p.skuId ?? p.id ?? ""),
			productId: p.productId as string | number | undefined,
			name: String(p.name ?? ""),
			price: String(p.price ?? "0"),
			priceValue:
				typeof p.priceValue === "number"
					? p.priceValue
					: parsePriceValue(String(p.price ?? "0")),
			quantity: Number(p.quantity ?? 1),
			status: p.status as string | undefined,
			image: p.image as string | undefined,
			unit: p.unit as string | undefined,
			sellerName: p.sellerName as string | undefined,
			sellerStoreId: p.sellerStoreId as string | undefined,
			href: p.href as string | undefined,
		}));
	} catch {
		return [];
	}
}

function write(items: CartItem[]) {
	if (!isBrowser()) return;
	localStorage.setItem(SAVED_STORAGE_KEY, JSON.stringify(items));
}

export function getSavedItems(): CartItem[] {
	return read();
}

export function saveItem(item: CartItem): CartItem[] {
	const items = read();
	if (!items.find((it) => it.id === item.id)) {
		items.push({ ...item, quantity: Math.max(1, item.quantity) });
	}
	write(items);
	return items;
}

export function removeSavedItem(id: string): CartItem[] {
	const items = read().filter((it) => it.id !== id);
	write(items);
	return items;
}
