"use client";

import { useEffect, useRef } from "react";
import { trackPurchase } from "@/lib/analytics/events";
import {
	clearPendingPurchase,
	markPurchaseTracked,
	readPendingPurchase,
} from "@/lib/analytics/purchase-store";
import { fetchOrder } from "@/lib/orders/repo";

/**
 * Fires one GA4 purchase per order. Arrival at /checkout/success ⇒ paid
 * (payment resolution routes paid→success, failed→fail, ATM→submitted), so no
 * status re-check is needed. Order numbers come from sessionStorage (set at
 * confirm, survives the PayUni round-trip); falls back to the URL order param.
 * Dedup is per-orderNo so a refresh does not re-fire.
 */
export default function PurchaseTracker({
	fallbackOrderNo,
}: {
	fallbackOrderNo?: string;
}) {
	const firedRef = useRef(false);
	useEffect(() => {
		if (firedRef.current) return;
		firedRef.current = true;
		const pending = readPendingPurchase();
		const orderNos =
			pending.length > 0 ? pending : fallbackOrderNo ? [fallbackOrderNo] : [];
		if (orderNos.length === 0) return;
		void (async () => {
			for (const orderNo of orderNos) {
				if (!markPurchaseTracked(orderNo)) continue;
				try {
					const order = await fetchOrder(orderNo);
					trackPurchase(order);
				} catch {
					// order fetch failed — leave dedup flag set (avoids retry storm);
					// a missed purchase is preferable to a duplicated one.
				}
			}
			clearPendingPurchase();
		})();
	}, [fallbackOrderNo]);
	return null;
}
