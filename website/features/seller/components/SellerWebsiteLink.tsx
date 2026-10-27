"use client";

import AppIcon from "@/components/shared/AppIcon";
import { normalizeWebsiteUrl } from "@/lib/seller/website-url";

interface SellerWebsiteLinkProps {
	href: string;
}

/**
 * Official-website affordance shown inline in the seller meta row (next to the
 * subscriber count). The globe is painted in the primary green so it matches
 * the neighbouring meta icons.
 */
export default function SellerWebsiteLink({ href }: SellerWebsiteLinkProps) {
	return (
		<a
			href={normalizeWebsiteUrl(href)}
			target="_blank"
			rel="noopener noreferrer"
			className="hkui-seller-profile__meta-item hkui-seller-profile__meta-item--website"
			aria-label="訪問官網"
			title="訪問官網"
		>
			<AppIcon
				name="discovery"
				size={16}
				style={{ color: "var(--hkui-primary)" }}
			/>
		</a>
	);
}
