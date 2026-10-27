import AppIcon, { type AppIconProps } from "./AppIcon";

export type SellerSidebarIconName =
	| "chart"
	| "preview"
	| "box"
	| "goods"
	| "stock"
	| "edit"
	| "ticket"
	| "users"
	| "wallet"
	| "truck"
	| "shield"
	| "settings"
	| "chat"
	| "notification"
	| "paper"
	| "logout";

type SellerSidebarIconProps = Omit<AppIconProps, "name"> & {
	name: SellerSidebarIconName;
};

/** Seller navigation glyphs; the sidebar row's visible label names the entry. */
export default function SellerSidebarIcon({
	name,
	size = 24,
	...rest
}: SellerSidebarIconProps) {
	return (
		<AppIcon
			{...rest}
			name={name}
			size={size}
			data-seller-sidebar-icon={name}
		/>
	);
}
