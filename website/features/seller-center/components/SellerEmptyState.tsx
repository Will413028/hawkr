"use client";

/**
 * SellerEmptyState — variant-mapped icon empty state for the seller
 * workspace. Mirrors the buyer-side `AccountEmptyStateIcon` registry so
 * /seller-center pages share one icon per content type instead of
 * each page picking its own emoji.
 *
 * Use directly as a list-empty placeholder, or pass `searchQuery` to
 * switch into the "no search matches" treatment (search icon + adaptive
 * copy). The `seamless` prop drops the card chrome so the same component
 * fits inside an existing `<td colSpan>` cell without nesting a second card.
 */

import type { AppIconName } from "@/components/shared/AppIcon";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";

export type SellerEmptyVariant =
	| "orders"
	| "products"
	| "inventory"
	| "customers"
	| "posts"
	| "coupons"
	| "notifications"
	| "messages"
	| "insights"
	| "todos"
	| "violations"
	| "invoices";

interface VariantConfig {
	icon: AppIconName;
	title: string;
	description: string;
}

const VARIANT_CONFIG: Record<SellerEmptyVariant, VariantConfig> = {
	orders: {
		icon: "box",
		title: "目前沒有訂單",
		description: "買家下單後會自動出現在這裡。",
	},
	products: {
		icon: "cart",
		title: "還沒有任何商品",
		description: "去上架第一個商品開始營業吧。",
	},
	inventory: {
		icon: "cart",
		title: "目前沒有庫存資料",
		description: "上架商品後可以在這裡管理庫存。",
	},
	customers: {
		icon: "people",
		title: "還沒有客戶",
		description: "有人下單後會自動加入這份名單。",
	},
	posts: {
		icon: "message",
		title: "還沒有發布動態",
		description: "跟訂閱者分享一下今天的好食材。",
	},
	coupons: {
		icon: "ticket",
		title: "還沒有發行的優惠券",
		description: "建立優惠券吸引買家回購。",
	},
	notifications: {
		icon: "notification",
		title: "目前沒有通知",
		description: "新訊息會即時出現在這裡。",
	},
	messages: {
		icon: "message",
		title: "目前沒有訊息",
		description: "買家詢問商品時，對話會列在這裡。",
	},
	insights: {
		icon: "document",
		title: "資料還沒準備好",
		description: "累積一些訂單後就能看到分析。",
	},
	todos: {
		icon: "trophy",
		title: "目前沒有待辦事項",
		description: "繼續加油，今天也把好食材照顧好了。",
	},
	violations: {
		icon: "document",
		title: "目前沒有違規紀錄",
		description: "保持良好營運狀況，這裡會維持空空如也。",
	},
	// 58 號電子發票（S01 ⑤／S06）。
	invoices: {
		icon: "paper",
		title: "還沒有發票或收據紀錄",
		description: "訂單付款成功後，發票會自動開立並顯示在這裡。",
	},
};

interface SellerEmptyStateProps {
	variant: SellerEmptyVariant;
	/** When set, switches to a "no search matches" state — overrides the
	 * variant's default copy with `找不到「{searchQuery}」` + a search
	 * icon. Leave undefined for the empty-list state. */
	searchQuery?: string;
	/** Drop the card chrome (border / background / padding) for use inside
	 * existing table cells or page sections that already provide them. */
	seamless?: boolean;
	/** Override the default title (rarely needed; prefer variant + search). */
	title?: string;
	/** Override the default description. */
	description?: string;
	/** Tweak the icon size; defaults to 104 (matching EmptyStatePanel). */
	iconSize?: number;
	className?: string;
}

export function SellerEmptyState({
	variant,
	searchQuery,
	seamless,
	title,
	description,
	iconSize,
	className,
}: SellerEmptyStateProps) {
	const config = VARIANT_CONFIG[variant];
	const trimmed = searchQuery?.trim();
	const isSearch = Boolean(trimmed);

	const resolved: VariantConfig = isSearch
		? {
				icon: "search",
				title: title ?? `找不到「${trimmed}」`,
				description: description ?? "試試不同的關鍵字，或清除搜尋條件。",
			}
		: {
				icon: config.icon,
				title: title ?? config.title,
				description: description ?? config.description,
			};
	const composedClassName = [
		"min-h-[240px]",
		seamless &&
			"bg-transparent border-0 py-[var(--hkui-space-8)] px-[var(--hkui-space-4)]",
		className,
	]
		.filter(Boolean)
		.join(" ");

	return (
		<EmptyStatePanel
			icon={resolved.icon}
			title={resolved.title}
			description={resolved.description}
			className={composedClassName}
			iconSize={iconSize}
		/>
	);
}
