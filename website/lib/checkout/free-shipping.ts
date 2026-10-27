import type { AvailableShippingOption } from "@/lib/checkout/repo";
import { ShippingFeePayer } from "@/lib/codemeta/enums";

/**
 * The backend's `freeShippingEligible` remains authoritative. The threshold
 * fallback only covers stale/missing eligibility flags and must still require
 * that the seller, rather than the buyer, funds the carrier fee.
 */
export function isFreeShippingForSubtotal(
	option: AvailableShippingOption | undefined,
	subtotal: number,
): boolean {
	if (!option) return false;
	if (option.freeShippingEligible) return true;
	const threshold = option.freeShippingThreshold ?? 0;
	return (
		option.feePayer === ShippingFeePayer.SELLER &&
		threshold > 0 &&
		subtotal >= threshold
	);
}
