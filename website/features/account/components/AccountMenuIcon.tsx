import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";

/**
 * Buyer account menu icons. Used by:
 *   - features/account/components/AccountSidebar.tsx (left rail on /account/*)
 *   - components/shared/Navbar.tsx (buyer dropdown menu)
 *
 * To add an entry, add its key to `AccountMenuIconName` and to
 * `ACCOUNT_MENU_ICONS`.
 */

export type AccountMenuIconName =
	| "overview"
	| "orders"
	| "coupons"
	| "wallet"
	| "favorites"
	| "recent"
	| "reviews"
	| "notifications"
	| "profile"
	| "addresses"
	| "invoiceProfiles"
	| "findSellers";

interface AccountMenuIconProps {
	name: AccountMenuIconName;
	className?: string;
}

const ACCOUNT_MENU_ICONS: Record<AccountMenuIconName, AppIconName> = {
	overview: "category",
	orders: "bag",
	coupons: "ticket",
	wallet: "wallet",
	favorites: "heart",
	recent: "clock",
	reviews: "star",
	notifications: "notification",
	profile: "user",
	addresses: "location",
	invoiceProfiles: "document",
	findSellers: "sparkle",
};

export default function AccountMenuIcon({
	name,
	className,
}: AccountMenuIconProps) {
	return (
		<AppIcon name={ACCOUNT_MENU_ICONS[name]} size={24} className={className} />
	);
}
