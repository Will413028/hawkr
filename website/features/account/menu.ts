/**
 * Single source of truth for the buyer's account-area menu.
 *
 * Consumed by:
 *   - Navbar dropdown (`components/shared/Navbar.tsx`)
 *   - `/account/*` left sidebar (`features/account/components/AccountSidebar.tsx`)
 *
 * Keep the order + labels in sync with the dropdown screenshot the
 * designer ships against (帳號總覽 → 通知 → 個人資料).
 *
 * `icon` is the key into `features/account/components/AccountMenuIcon` (inherits
 * color via currentColor).
 */

import type { AccountMenuIconName } from "@/features/account/components/AccountMenuIcon";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import { ASSISTANT_PROCUREMENT_PATH } from "@/features/procurement-assistant/routes";

export interface BuyerMenuItem {
	label: string;
	href: string;
	icon: AccountMenuIconName;
}

const BASE_BUYER_MENU_ITEMS = [
	{ label: "帳號總覽", href: "/account", icon: "overview" },
	{ label: "我的訂單", href: "/account/orders", icon: "orders" },
	{ label: "我的優惠券", href: "/account/coupons", icon: "coupons" },
	{ label: "會員卡", href: "/account/wallet", icon: "wallet" },
	{ label: "收藏", href: "/account/favorites", icon: "favorites" },
	{ label: "瀏覽紀錄", href: "/account/recent", icon: "recent" },
	{ label: "我的評論", href: "/account/reviews", icon: "reviews" },
	{ label: "通知", href: "/account/notifications", icon: "notifications" },
	{ label: "個人資料", href: "/account/profile", icon: "profile" },
	{ label: "收貨地址", href: "/account/addresses", icon: "addresses" },
	// 追加一律到陣列尾端：BUYER_MENU_GROUPS 用 index 引用本陣列，中間插入會讓
	// 選單項目變成別的頁面，而且不編譯錯、不測試紅。
	{
		label: "發票資料",
		href: "/account/invoice-profiles",
		icon: "invoiceProfiles",
	},
] as const satisfies ReadonlyArray<BuyerMenuItem>;

/**
 * 「發票與收據」（B09，58A §B09 指定放在「我的訂單」之後）。
 *
 * 刻意**不放進 `BASE_BUYER_MENU_ITEMS`**：那個陣列被 `BUYER_MENU_GROUPS` 以 index
 * 引用，往中段插入會讓每個群組的項目變成別的頁面，而且不編譯錯、不測試紅。獨立
 * 常數是這個檔案已經有的解法（見下方的 `HAWKR_AGENT_MENU_ITEM`）。
 *
 * 沿用 `invoiceProfiles` 圖示：兩者都是發票語意，B09 明訂「需新增元件：無」。
 */
const INVOICE_DOCUMENTS_MENU_ITEM = {
	label: "發票與收據",
	href: "/account/invoices",
	icon: "invoiceProfiles",
} as const satisfies BuyerMenuItem;

const HAWKR_AGENT_MENU_ITEM = {
	label: "Hawkr Agent",
	href: ASSISTANT_PROCUREMENT_PATH,
	icon: "findSellers",
} as const satisfies BuyerMenuItem;

export const BUYER_MENU_ITEMS: ReadonlyArray<BuyerMenuItem> = [
	BASE_BUYER_MENU_ITEMS[0], // 帳號總覽
	BASE_BUYER_MENU_ITEMS[1], // 我的訂單
	INVOICE_DOCUMENTS_MENU_ITEM, // 發票與收據（緊接「我的訂單」）
	...BASE_BUYER_MENU_ITEMS.slice(2),
	...(HAWKR_AGENT_ENABLED ? [HAWKR_AGENT_MENU_ITEM] : []),
];

/**
 * Grouped view of the same menu, used by the `/account/*` left sidebar.
 * Mirrors the seller-center sidebar's "營運 / 行銷 / 財務與設定" pattern so
 * buyer and seller workspaces share the same chrome (glass card + uppercase
 * eyebrow headings above each group). The navbar dropdown keeps consuming
 * the flat `BUYER_MENU_ITEMS` array so it stays compact.
 */
export interface BuyerMenuGroup {
	heading: string;
	items: ReadonlyArray<BuyerMenuItem>;
}

export const BUYER_MENU_GROUPS: ReadonlyArray<BuyerMenuGroup> = [
	{
		heading: "購物",
		items: [
			BASE_BUYER_MENU_ITEMS[0], // 帳號總覽
			BASE_BUYER_MENU_ITEMS[1], // 我的訂單
			INVOICE_DOCUMENTS_MENU_ITEM, // 發票與收據
			BASE_BUYER_MENU_ITEMS[4], // 收藏
			BASE_BUYER_MENU_ITEMS[5], // 瀏覽紀錄
			...(HAWKR_AGENT_ENABLED ? [HAWKR_AGENT_MENU_ITEM] : []),
		],
	},
	{
		heading: "優惠與會員",
		items: [
			BASE_BUYER_MENU_ITEMS[2], // 我的優惠券
			BASE_BUYER_MENU_ITEMS[3], // 會員卡
		],
	},
	{
		heading: "互動",
		items: [
			BASE_BUYER_MENU_ITEMS[6], // 我的評論
			BASE_BUYER_MENU_ITEMS[7], // 通知
		],
	},
	{
		heading: "個人設定",
		items: [
			BASE_BUYER_MENU_ITEMS[8], // 個人資料
			BASE_BUYER_MENU_ITEMS[9], // 收貨地址
			BASE_BUYER_MENU_ITEMS[10], // 發票資料
		],
	},
];

/** Match logic shared with the dropdown — exact match or descendant. */
export function isAccountMenuActive(pathname: string, href: string): boolean {
	if (href === "/account") return pathname === "/account";
	return pathname === href || pathname.startsWith(`${href}/`);
}
