"use client";

import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";

export type AccountEmptyIconVariant =
	| "orders"
	| "coupons"
	| "favorites"
	| "sellers"
	| "address"
	| "recent"
	| "reviews"
	| "wallet"
	| "tracking"
	| "return"
	| "ai"
	| "notifications";

interface AccountEmptyIconProps {
	variant: AccountEmptyIconVariant;
	className?: string;
	size?: number;
}

interface AccountEmptyStateIconProps {
	variant: AccountEmptyIconVariant;
}

const iconConfig: Record<
	AccountEmptyIconVariant,
	{
		icon: AppIconName;
		label: string;
	}
> = {
	orders: {
		icon: "box",
		label: "我的訂單",
	},
	coupons: {
		icon: "discount",
		label: "我的優惠券",
	},
	favorites: {
		icon: "heart",
		label: "收藏商品",
	},
	sellers: {
		icon: "home",
		label: "追蹤賣家",
	},
	address: {
		icon: "location",
		label: "收貨地址",
	},
	recent: {
		icon: "search",
		label: "瀏覽紀錄",
	},
	reviews: {
		icon: "star",
		label: "我的評價",
	},
	wallet: {
		icon: "wallet",
		label: "會員卡",
	},
	tracking: {
		icon: "truck",
		label: "物流追蹤",
	},
	return: {
		icon: "paper",
		label: "申請完成",
	},
	ai: {
		icon: "sparkle",
		label: "AI 食材助手",
	},
	notifications: {
		icon: "notification",
		label: "通知",
	},
};

export function AccountEmptyStateIcon({ variant }: AccountEmptyStateIconProps) {
	return (
		<span
			aria-hidden="true"
			className="w-[132px] h-[132px] grid place-items-center rounded-[var(--radius-pill)] bg-[radial-gradient(circle,rgb(var(--color-brand-secondary-rgb)/0.14),transparent_68%),rgb(var(--color-brand-ink-rgb)/0.04)] text-[var(--hkui-foreground-muted)] text-[0px] dark:bg-[radial-gradient(circle,rgb(var(--color-brand-secondary-rgb)/0.2),transparent_68%),rgb(var(--color-brand-primary-rgb)/0.075)]"
		>
			<AccountEmptyIcon variant={variant} />
		</span>
	);
}

export default function AccountEmptyIcon({
	variant,
	className,
	size = 118,
}: AccountEmptyIconProps) {
	const config = iconConfig[variant];

	return (
		<span
			className={[
				"grid place-items-center w-full h-full overflow-visible drop-shadow-[0_18px_26px_rgb(var(--color-ink-900-rgb)/0.1)] dark:drop-shadow-[0_18px_32px_rgba(0,0,0,0.34)]",
				className,
			]
				.filter(Boolean)
				.join(" ")}
		>
			<span className="grid place-items-center w-full h-full">
				<AppIcon name={config.icon} size={size} title={config.label} />
			</span>
		</span>
	);
}
