import type * as React from "react";
import SellerSidebarIcon from "@/components/shared/SellerSidebarIcon";

/**
 * 賣家中心側欄的**選單資料**（不含渲染）。
 *
 * 抽成獨立模組的唯一理由：ADM 代行工作區沿用同一份選單，只在渲染時套一份禁區
 * 排除清單（56 號 §6.1.14／定案第 3 條）。資料只有一份，賣家中心新增模組時代行
 * 側欄自動跟上；複製一份就會漂移，而漂移的症狀是「代行看得到的模組與賣家實際
 * 擁有的模組不一樣」，沒有任何東西會報錯。
 *
 * 渲染（NavLink、longest-match active 判定、badge）仍留在各自的 shell：兩邊的
 * 連結目標與可操作性本來就不同。
 */

export type NavBadgeKey =
	| "orders"
	| "inventory"
	| "messages"
	| "notifications"
	| "posts";

export type NavBadgeMap = Partial<Record<NavBadgeKey, number>>;

export type NavItem = {
	href: string;
	label: string;
	icon: React.ReactNode;
	badge?: string | number;
	badgeKey?: NavBadgeKey;
	matchPrefix?: string;
};

export type NavGroup = {
	heading: string;
	items: NavItem[];
};

export const SELLER_NAV_GROUPS: NavGroup[] = [
	{
		heading: "營運",
		items: [
			{
				href: "/seller-center/dashboard",
				label: "總覽",
				icon: <SellerSidebarIcon name="chart" />,
			},
			{
				href: "/seller-center/notifications",
				label: "通知中心",
				icon: <SellerSidebarIcon name="notification" />,
				badgeKey: "notifications",
			},
			{
				href: "/seller-center/preview",
				label: "商店預覽",
				icon: <SellerSidebarIcon name="preview" />,
			},
			{
				href: "/seller-center/orders",
				label: "訂單管理",
				icon: <SellerSidebarIcon name="box" />,
				badgeKey: "orders",
			},
			{
				href: "/seller-center/products",
				label: "商品管理",
				icon: <SellerSidebarIcon name="goods" />,
			},
			{
				href: "/seller-center/inventory",
				label: "庫存管理",
				icon: <SellerSidebarIcon name="stock" />,
				badgeKey: "inventory",
			},
		],
	},
	{
		heading: "行銷",
		items: [
			{
				href: "/seller-center/posts",
				label: "動態管理",
				icon: <SellerSidebarIcon name="edit" />,
				badgeKey: "posts",
			},
			{
				href: "/seller-center/coupons",
				label: "優惠券",
				icon: <SellerSidebarIcon name="ticket" />,
			},
			{
				href: "/seller-center/customers",
				label: "客戶管理",
				icon: <SellerSidebarIcon name="users" />,
			},
			{
				href: "/seller-center/messages",
				label: "訊息中心",
				icon: <SellerSidebarIcon name="chat" />,
				badgeKey: "messages",
			},
		],
	},
	{
		heading: "財務與設定",
		items: [
			{
				href: "/seller-center/wallet",
				label: "錢包與認證",
				icon: <SellerSidebarIcon name="wallet" />,
			},
			// 58 號電子發票（58A §B-2.1 #1）。子路由（稅務身分、發票紀錄）不進側欄
			// ——`NavItem` 沒有 `children`、shell 也沒有巢狀渲染，做出五層等於新創一種
			// 側欄互動模式；子節點改為 S01 頁上的卡片入口。`pathMatchesItem` 預設就做
			// 前綴比對，所有 /invoices/* 子路由會自動高亮，不必設 `matchPrefix`。
			{
				href: "/seller-center/invoices",
				label: "發票與稅籍",
				icon: <SellerSidebarIcon name="paper" />,
			},
			{
				href: "/seller-center/me/shipping",
				label: "物流與退貨",
				icon: <SellerSidebarIcon name="truck" />,
			},
			{
				href: "/seller-center/insights",
				label: "市場行情",
				icon: <SellerSidebarIcon name="chart" />,
			},
			{
				href: "/seller/measures",
				label: "措施與申覆",
				icon: <SellerSidebarIcon name="shield" />,
			},
		],
	},
];

export const SELLER_FOOTER_ITEMS: NavItem[] = [
	{
		href: "/seller-center/me",
		label: "商店設定",
		icon: <SellerSidebarIcon name="settings" />,
	},
];
