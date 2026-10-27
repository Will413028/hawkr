"use client";

import { LOCAL_PRIVATE_KEYS } from "@/lib/auth/local-private-keys";

export const CHECKOUT_DRAFT_STORAGE_KEY = LOCAL_PRIVATE_KEYS.checkoutDraft.key;
/**
 * Recipient / store-pickup carry the buyer's name, phone and address (PII).
 * They live in sessionStorage, not localStorage, so they survive a reload of
 * the checkout page but do NOT linger on a shared machine after the tab closes.
 * The non-PII selection (skus / shipping / coupon) stays in localStorage so the
 * cross-tab `storage` double-submit guard keeps working.
 */
const CHECKOUT_DRAFT_PII_KEY = LOCAL_PRIVATE_KEYS.checkoutDraftPii.key;

export interface CheckoutDraftRecipient {
	id?: number | string;
	name: string;
	phone: string;
	line: string;
	zipCode?: string;
}

export interface CheckoutDraftStorePickup {
	id?: string;
	storeId: string;
	storeName: string;
	storeAddress?: string;
	receiverName: string;
	receiverPhone: string;
}

export interface CheckoutDraftCoupon {
	templateId: string;
	instanceId?: string;
	title: string;
	kind: string;
	estimatedDiscount: number;
}

export interface CheckoutDraftSeller {
	key: string;
	name: string;
	logoUrl?: string;
}

/**
 * Checkout 跨頁載體 — 只記「選擇」不記「快照」。
 *
 * Items 來源在 checkout 頁 mount 時直接 await loadCart() 拿後端權威 cart,
 * 與 buyer-app 行為對齊(後端是唯一真實來源)。draft 只負責跨路由傳遞
 * 使用者在 /cart 頁的選擇:勾了哪些 sku、選哪家賣家、配送、優惠券、收件人。
 *
 * 不存 items 快照與 totals,避免 stale 資料被誤帶進 confirm payload。
 */
export interface CheckoutDraft {
	// version 2: items/totals 快照拿掉,改用 selectedSkuIds,checkout 頁從後端
	// cart 動態取得真實 items。舊 version=1 draft 由 isCheckoutDraft 拒絕,
	// 使用者落到「結帳資料已過期」自然降級。
	version: 2;
	createdAt: number;
	validateAt: number;
	seller: CheckoutDraftSeller;
	selectedSkuIds: string[];
	shipping: {
		method: string;
		deliveryTemp: string;
		label: string;
		fee: number;
	};
	recipient?: CheckoutDraftRecipient;
	storePickup?: CheckoutDraftStorePickup;
	coupon?: CheckoutDraftCoupon;
	note: string;
}

interface CheckoutDraftPii {
	recipient?: CheckoutDraftRecipient;
	storePickup?: CheckoutDraftStorePickup;
}

function isCheckoutDraft(value: unknown): value is CheckoutDraft {
	if (!value || typeof value !== "object") return false;
	const draft = value as Partial<CheckoutDraft>;
	return (
		draft.version === 2 &&
		Array.isArray(draft.selectedSkuIds) &&
		draft.selectedSkuIds.length > 0 &&
		draft.selectedSkuIds.every(
			(id) => typeof id === "string" && id.length > 0,
		) &&
		typeof draft.shipping?.method === "string" &&
		typeof draft.shipping?.deliveryTemp === "string" &&
		typeof draft.seller?.key === "string" &&
		typeof draft.createdAt === "number"
	);
}

function readPii(): CheckoutDraftPii {
	try {
		const raw = window.sessionStorage.getItem(CHECKOUT_DRAFT_PII_KEY);
		if (!raw) return {};
		const parsed = JSON.parse(raw) as unknown;
		return parsed && typeof parsed === "object"
			? (parsed as CheckoutDraftPii)
			: {};
	} catch {
		return {};
	}
}

export function readCheckoutDraft(): CheckoutDraft | null {
	if (typeof window === "undefined") return null;
	try {
		const raw = window.localStorage.getItem(CHECKOUT_DRAFT_STORAGE_KEY);
		if (!raw) return null;
		const parsed = JSON.parse(raw) as unknown;
		if (!isCheckoutDraft(parsed)) return null;
		const pii = readPii();
		// Prefer sessionStorage PII; fall back to any inline PII left by a
		// pre-split draft so an in-flight checkout doesn't lose its recipient.
		return {
			...parsed,
			recipient: pii.recipient ?? parsed.recipient,
			storePickup: pii.storePickup ?? parsed.storePickup,
		};
	} catch {
		return null;
	}
}

export function writeCheckoutDraft(draft: CheckoutDraft) {
	if (typeof window === "undefined") return;
	const { recipient, storePickup, ...base } = draft;
	window.localStorage.setItem(CHECKOUT_DRAFT_STORAGE_KEY, JSON.stringify(base));
	try {
		if (recipient || storePickup) {
			window.sessionStorage.setItem(
				CHECKOUT_DRAFT_PII_KEY,
				JSON.stringify({ recipient, storePickup }),
			);
		} else {
			window.sessionStorage.removeItem(CHECKOUT_DRAFT_PII_KEY);
		}
	} catch {
		// sessionStorage unavailable (privacy mode) — PII just won't survive a reload.
	}
}

export function clearCheckoutDraft() {
	if (typeof window === "undefined") return;
	window.localStorage.removeItem(CHECKOUT_DRAFT_STORAGE_KEY);
	try {
		window.sessionStorage.removeItem(CHECKOUT_DRAFT_PII_KEY);
	} catch {
		// ignore — nothing to clean up if sessionStorage is unavailable.
	}
}
