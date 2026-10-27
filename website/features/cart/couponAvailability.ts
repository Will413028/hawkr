import type { CartAvailableCoupon } from "@/lib/coupons/repo";

export interface CartCouponAvailabilityContext {
	orderAmount: number;
	shippingFee: number;
	now?: number;
}

export interface CartCouponAvailability {
	usable: boolean;
	reason: string | null;
}

function ntFormat(n: number): string {
	return `NT$ ${Math.max(0, Math.ceil(n)).toLocaleString()}`;
}

/**
 * Explain whether a backend cart-context coupon can be selected.
 *
 * `estimatedDiscount` remains the authoritative applicability signal. The
 * client only adds reasons that can be proven from fields already returned by
 * the coupon contract, so the UI never invents a discount calculation.
 */
export function getCartCouponAvailability(
	coupon: CartAvailableCoupon,
	context: CartCouponAvailabilityContext,
): CartCouponAvailability {
	const now = context.now ?? Date.now();
	const threshold = Number(coupon.thresholdAmount ?? coupon.min_spend ?? 0);

	if (coupon.claimRequired === true && coupon.claimed === false) {
		return { usable: false, reason: "需先到領券中心領取" };
	}

	if (coupon.starts_at != null && coupon.starts_at > now) {
		return { usable: false, reason: "尚未到可使用時間" };
	}

	if (coupon.expires_at != null && coupon.expires_at < now) {
		return { usable: false, reason: "優惠券已逾期" };
	}

	if (Number.isFinite(threshold) && threshold > context.orderAmount) {
		return {
			usable: false,
			reason: `再購買 ${ntFormat(threshold - context.orderAmount)} 即可使用`,
		};
	}

	if (
		!Number.isFinite(coupon.estimatedDiscount) ||
		coupon.estimatedDiscount <= 0
	) {
		return { usable: false, reason: "本次訂單不適用" };
	}

	return { usable: true, reason: null };
}
