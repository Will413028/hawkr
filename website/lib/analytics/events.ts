/**
 * Typed GA4 ecommerce event functions. Each trackX wraps a pure build*
 * helper (unit-testable) + a single track() dispatch. value/currency follow
 * GA4 recommended-events semantics.
 */

import type { CartInput } from "@/lib/cart";
import type { CartItem } from "@/lib/cart/types";
import { parsePriceValue } from "@/lib/cart/types";
import type { MarketProductItem } from "@/lib/market/types";
import type { Order } from "@/lib/orders/types";
import { setUserProperties, track } from "./gtag";
import {
	cartInputToGA4,
	cartItemToGA4,
	marketItemToGA4,
	orderItemToGA4,
	sumValue,
} from "./items";
import { type AnalyticsRole, CURRENCY, type GA4Item } from "./types";

interface ListContext {
	listId: string;
	listName: string;
}

export function trackViewItemList(
	products: MarketProductItem[],
	ctx: ListContext,
): void {
	if (products.length === 0) return;
	track("view_item_list", {
		item_list_id: ctx.listId,
		item_list_name: ctx.listName,
		items: products.map((p, i) => ({
			...marketItemToGA4(p, i),
			item_list_id: ctx.listId,
			item_list_name: ctx.listName,
		})),
	});
}

export function trackSelectItem(
	product: MarketProductItem,
	ctx: ListContext,
	index?: number,
): void {
	track("select_item", {
		item_list_id: ctx.listId,
		item_list_name: ctx.listName,
		items: [
			{
				...marketItemToGA4(product, index),
				item_list_id: ctx.listId,
				item_list_name: ctx.listName,
			},
		],
	});
}

export function trackViewItem(item: GA4Item): void {
	track("view_item", {
		currency: CURRENCY,
		value: item.price ?? 0,
		items: [item],
	});
}

export function trackAddToCart(input: CartInput, quantity: number): void {
	const item = cartInputToGA4(input, quantity);
	track("add_to_cart", {
		currency: CURRENCY,
		value: (item.price ?? 0) * quantity,
		items: [item],
	});
}

export function trackRemoveFromCart(cartItem: CartItem): void {
	const item = cartItemToGA4(cartItem);
	track("remove_from_cart", {
		currency: CURRENCY,
		value: (item.price ?? 0) * (item.quantity ?? 1),
		items: [item],
	});
}

export function trackViewCart(cartItems: CartItem[]): void {
	if (cartItems.length === 0) return;
	const items = cartItems.map(cartItemToGA4);
	track("view_cart", { currency: CURRENCY, value: sumValue(items), items });
}

export function trackAddToWishlist(args: {
	productId: string | number;
	productName?: string;
	price?: number;
}): void {
	track("add_to_wishlist", {
		currency: CURRENCY,
		value: args.price ?? 0,
		items: [
			{
				item_id: String(args.productId),
				item_name: args.productName ?? "",
				...(args.price ? { price: args.price } : {}),
			},
		],
	});
}

export function trackBeginCheckout(
	cartItems: CartItem[],
	ctx: { value: number; coupon?: string },
): void {
	track("begin_checkout", {
		currency: CURRENCY,
		value: ctx.value,
		...(ctx.coupon ? { coupon: ctx.coupon } : {}),
		items: cartItems.map(cartItemToGA4),
	});
}

export function trackAddShippingInfo(
	cartItems: CartItem[],
	ctx: { value: number; shippingTier: string; coupon?: string },
): void {
	track("add_shipping_info", {
		currency: CURRENCY,
		value: ctx.value,
		shipping_tier: ctx.shippingTier,
		...(ctx.coupon ? { coupon: ctx.coupon } : {}),
		items: cartItems.map(cartItemToGA4),
	});
}

export function trackAddPaymentInfo(
	cartItems: CartItem[],
	ctx: { value: number; paymentType: string; coupon?: string },
): void {
	track("add_payment_info", {
		currency: CURRENCY,
		value: ctx.value,
		payment_type: ctx.paymentType,
		...(ctx.coupon ? { coupon: ctx.coupon } : {}),
		items: cartItems.map(cartItemToGA4),
	});
}

export interface PurchaseParams {
	transaction_id: string;
	value: number;
	currency: string;
	shipping?: number;
	items: GA4Item[];
}

/** Pure: build a GA4 purchase payload from one Order (one order = one txn). */
export function buildPurchaseParams(order: Order): PurchaseParams {
	const shipping = parsePriceValue(order.shippingFee);
	return {
		transaction_id: order.orderNo,
		value: parsePriceValue(order.totalActualAmount),
		currency: CURRENCY,
		...(shipping > 0 ? { shipping } : {}),
		items: order.items.map(orderItemToGA4),
	};
}

export function trackPurchase(order: Order): void {
	track("purchase", { ...buildPurchaseParams(order) });
}

export function trackLogin(method: "google" | "line"): void {
	setUserProperties({ login_method: method });
	track("login", { method });
}

export interface SellerLeadAnalyticsContext {
	plan: "starter" | "pro" | "undecided";
	billingCycle: "monthly" | "annual";
	serviceInterest: "none" | "managed";
}

/**
 * Track an accepted seller contact request as GA4's recommended lead event.
 * The typed input intentionally contains only non-PII categorical fields.
 */
export function trackSellerLead(context: SellerLeadAnalyticsContext): void {
	track("generate_lead", {
		lead_source: "seller_pricing",
		plan: context.plan,
		billing_cycle: context.billingCycle,
		service_interest: context.serviceInterest,
	});
}

/**
 * Track completion of the seller setup flow. This is intentionally not GA4's
 * recommended `sign_up`: an existing buyer can add a seller role without
 * creating a new Hawkr account.
 */
export function trackSellerOnboardingCompleted(
	sellerProfileAction: "created" | "updated",
): void {
	track("seller_onboarding_completed", {
		completion_source: "seller_self_service",
		seller_profile_action: sellerProfileAction,
	});
}

export function roleProperty(
	role: AnalyticsRole | null,
): Record<string, string> {
	return { role: role ?? "anonymous" };
}
