"use client";

import { Drawer } from "@/components/shared/Drawer";
import type { SellerCustomer } from "@/lib/seller/repo";
import CustomerDetailPanel from "./CustomerDetailPanel";

interface Props {
	open: boolean;
	/** The list row that was clicked — seeds the header instantly while the
	 * full detail loads. `null` renders nothing. */
	customer: SellerCustomer | null;
	onClose: () => void;
	/** Fired the moment the seller mutates the customer from inside the drawer
	 * (classification / tags) so the list re-syncs immediately — orthogonal to
	 * onClose (#1247). Delivered to the inner panel via {@link useDrawerMutated}. */
	onMutated?: () => void;
}

/**
 * Right-side drawer for a CRM customer — same primitive as the after-sales
 * decision drawer ({@link Drawer} `side="right"`). Lets a seller open a
 * customer from the list, act on them, and flick to the next without a page
 * navigation.
 */
export default function CustomerDetailDrawer({
	open,
	customer,
	onClose,
	onMutated,
}: Props) {
	return (
		<Drawer
			open={open}
			onClose={onClose}
			onMutated={onMutated}
			side="right"
			title="客戶詳情"
			// Wider than the default sheet (max-w-md) so the purchase-history
			// table breathes; still full-width on phones via the base w-full.
			className="max-w-2xl"
		>
			{customer ? (
				<CustomerDetailPanel buyerId={customer.id} summary={customer} />
			) : null}
		</Drawer>
	);
}
