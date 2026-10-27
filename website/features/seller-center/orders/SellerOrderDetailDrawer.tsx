"use client";

import { Drawer } from "@/components/shared/Drawer";
import OrderDetailClient from "./OrderDetailClient";

interface Props {
	open: boolean;
	/** Order number to show; `null` renders nothing. */
	orderNo: string | null;
	onClose: () => void;
	/** Fired the moment an in-drawer order action (ship / label / cancel …)
	 * succeeds so the orders workspace re-syncs — orthogonal to onClose (#1247).
	 * Delivered to {@link OrderDetailClient} via {@link useDrawerMutated}. */
	onMutated?: () => void;
}

/**
 * Right-side drawer for a seller order — same {@link Drawer} primitive as
 * the customer + after-sales drawers. Reuses {@link OrderDetailClient} in `bare`
 * mode so the seller can open an order, ship / act on it, and flick to the next
 * without leaving the list. The full `/orders/[orderNo]` route stays as the
 * deep-link / refresh fallback.
 */
export default function SellerOrderDetailDrawer({
	open,
	orderNo,
	onClose,
	onMutated,
}: Props) {
	return (
		<Drawer
			open={open}
			onClose={onClose}
			onMutated={onMutated}
			side="right"
			title="訂單詳情"
			// Drawer width — the order detail's 2-column grid collapses to a single
			// column at this width, reading as a focused side panel; stays full-width
			// on phones (base w-full).
			className="max-w-lg"
		>
			{orderNo ? <OrderDetailClient orderNo={orderNo} bare /> : null}
		</Drawer>
	);
}
