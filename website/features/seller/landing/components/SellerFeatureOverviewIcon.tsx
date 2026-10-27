import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import type { SellerFeatureOverviewIconName } from "../sellerFeatureOverviewData";

export const SELLER_FEATURE_ICONS: Readonly<
	Record<SellerFeatureOverviewIconName, AppIconName>
> = {
	storefront: "home",
	products: "bag",
	inventory: "stock",
	orders: "document",
	fulfillment: "location",
	customers: "users",
	content: "discount",
	messages: "message",
	operations: "chart",
};

interface SellerFeatureOverviewIconProps {
	icon: SellerFeatureOverviewIconName;
}

/** Decorative glyph beside a feature heading; the heading carries the meaning. */
export default function SellerFeatureOverviewIcon({
	icon,
}: SellerFeatureOverviewIconProps) {
	return (
		<span
			className="seller-feature-overview__icon"
			data-seller-feature-icon={icon}
			aria-hidden="true"
		>
			<AppIcon name={SELLER_FEATURE_ICONS[icon]} size={30} />
		</span>
	);
}
