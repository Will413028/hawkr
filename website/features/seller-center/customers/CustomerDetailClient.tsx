"use client";

import AppIcon from "@/components/shared/AppIcon";
import { BackLink } from "@/components/shared/BackLink";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import CustomerDetailPanel from "./CustomerDetailPanel";

/**
 * Full-page CRM customer detail (`/seller-center/customers/[buyerId]`). The
 * body lives in {@link CustomerDetailPanel}, shared with the right-side drawer
 * on the customer list — this route stays as the deep-link / refresh / share
 * fallback.
 */
export default function CustomerDetailClient({ buyerId }: { buyerId: string }) {
	return (
		<SellerPageShell
			className="seller-customer-detail"
			title="客戶詳情"
			toolbar={
				<BackLink
					fallbackHref="/seller-center/customers"
					className="seller-customer-detail__back"
				>
					<AppIcon name="arrow-left" size={16} />
					返回客戶管理
				</BackLink>
			}
		>
			<CustomerDetailPanel buyerId={buyerId} />
		</SellerPageShell>
	);
}
