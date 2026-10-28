"use client";

import { useQuery } from "@connectrpc/connect-query";
import { IdentityAdminService } from "@hawkr/api/gen/hawkr/admin/identity/v1alpha1/identity_pb";
import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { forwardRef } from "react";
import { ToastRegions } from "@/components/UI/ToastRegions";
import { AuthService } from "@/services/auth";
import AdminBrand from "./AdminBrand";

interface SidebarProps {
	open: boolean;
	mobile: boolean;
	onClose: () => void;
}

interface NavItem {
	href: string;
	label: string;
	icon: MaterialSymbolName;
	exact?: boolean;
}

const navGroups: Array<{ section: string; items: NavItem[] }> = [
	{
		section: "總覽",
		items: [
			{ href: "/admin", label: "平台總覽", icon: "apps", exact: true },
			{
				href: "/admin/analytics",
				label: "數據分析",
				icon: "bar_chart",
			},
		],
	},
	{
		section: "內容治理",
		items: [
			{
				href: "/admin/reports",
				label: "內容審核佇列",
				icon: "crisis_alert",
			},
			{ href: "/admin/posts", label: "貼文管理", icon: "article" },
			{
				href: "/admin/disputes",
				label: "售後爭議",
				icon: "support_agent",
			},
		],
	},
	{
		section: "賣家 / 商品",
		items: [
			{
				href: "/admin/queue",
				label: "待辦",
				icon: "task",
			},
			{
				href: "/admin/merchants",
				label: "賣家管理 / 審核",
				icon: "storefront",
			},
			{
				href: "/admin/violation-reports",
				label: "商家通報",
				icon: "gpp_bad",
			},
			{
				href: "/admin/authority-orders",
				label: "主管機關來函",
				icon: "account_balance",
			},
			{
				href: "/admin/measures",
				label: "措施與申覆",
				icon: "balance",
			},
			{
				href: "/admin/delegated-operations",
				label: "代行紀錄",
				icon: "policy",
			},
			{
				// 站外業者主檔（尚未加入平台的目標賣家名單，供業務團隊追蹤入駐進度）。
				// 圖示與同組的「賣家管理／審核」（storefront）刻意錯開，避免兩個
				// 入口在側欄一眼看起來像同一個功能。
				href: "/admin/seller-prospects",
				label: "站外業者",
				icon: "store",
			},
			{ href: "/admin/members", label: "買家管理", icon: "group" },
			{ href: "/admin/products", label: "商品管理", icon: "nutrition" },
			{
				href: "/admin/category-audit",
				label: "分類審核",
				icon: "library_add_check",
			},
		],
	},
	{
		section: "交易",
		items: [
			{
				href: "/admin/transactions",
				label: "交易管理",
				icon: "sync_alt",
			},
			{
				href: "/admin/anomalies",
				label: "異常監控",
				icon: "crisis_alert",
			},
		],
	},
	{
		// 發票 / 稅務（58A §B-3.1.1）。這是**唯一**必須手動登記的地方：ADM 沒有
		// 路由清冊 gate，`src/app/admin/<path>/page.tsx` 存在即成路由 ⇒ 漏了這一段，
		// 頁面存在、路由通、但沒有任何入口找得到它。
		//
		// A03／A06／A08／A09／A11／A13 刻意不進來：詳情頁與表單頁的入口在各自的
		// 佇列頁（同 merchants/[id]、disputes/[id] 的慣例）。
		section: "發票 / 稅務",
		items: [
			{
				// exact 是必要的：選中判定是 prefix 比對（Sidebar 下方 isActive），
				// 少了它，進 /admin/invoices/jobs 時「發票營運總覽」會跟著一起亮。
				href: "/admin/invoices",
				label: "發票營運總覽",
				icon: "receipt_long",
				exact: true,
			},
			{
				href: "/admin/invoices/tax-reviews",
				label: "稅務身分審核",
				icon: "account_balance",
			},
			{
				href: "/admin/invoices/provider-accounts",
				label: "AMEGO 串接監控",
				icon: "electrical_services",
			},
			{
				href: "/admin/invoices/jobs",
				label: "發票工作與異常",
				icon: "crisis_alert",
			},
			{
				href: "/admin/invoices/complaints",
				label: "未收到發票申訴",
				icon: "support_agent",
			},
			{
				href: "/admin/invoices/documents",
				label: "發票文件查詢",
				icon: "search",
			},
			{
				href: "/admin/invoices/audit-logs",
				label: "發票稽核紀錄",
				icon: "policy",
			},
		],
	},
	{
		section: "行銷 / 營運",
		items: [
			{ href: "/admin/banners", label: "管理 Banner", icon: "image" },
			{
				href: "/admin/notification-templates",
				label: "通知模板管理",
				icon: "notifications",
			},
			{ href: "/admin/ai-styles", label: "AI 修圖管理", icon: "wand_stars" },
			{
				href: "/admin/app-versions",
				label: "App 版本管理",
				icon: "mobile",
			},
			{
				href: "/admin/membership-plans",
				label: "會員方案目錄",
				icon: "diamond",
			},
		],
	},
	{
		section: "物流",
		items: [
			{
				href: "/admin/holiday-calendar",
				label: "假日行事曆",
				icon: "event",
			},
		],
	},
];

const Sidebar = forwardRef<HTMLElement, SidebarProps>(function Sidebar(
	{ open, mobile, onClose },
	ref,
) {
	const pathname = usePathname();
	// 登入的管理員，只供顯示；載入中與讀取失敗時顯示預設值。
	const { data: session } = useQuery(
		IdentityAdminService.method.getSession,
		{},
	);
	const admin = session?.admin;

	const handleLogout = () => {
		void AuthService.logout();
	};

	const isActive = (href: string, exact?: boolean) => {
		if (exact) return pathname === href;
		return pathname === href || pathname.startsWith(`${href}/`);
	};

	const sidebarContent = (
		<>
			<div className="sidebar-header">
				<Link href="/admin" className="sidebar-brand" onClick={onClose}>
					<AdminBrand tone="light" priority className="sidebar-brand-lockup" />
					<span className="sidebar-title">平台營運管理</span>
				</Link>
				<button
					type="button"
					className="sidebar-close"
					onClick={onClose}
					aria-label="關閉主選單"
				>
					<MaterialSymbol name="close" />
				</button>
			</div>

			<div className="sidebar-nav">
				{navGroups.map((group) => (
					<div key={group.section} className="sidebar-section">
						<div className="sidebar-section-title">{group.section}</div>
						{group.items.map((item) => (
							<Link
								key={item.href}
								href={item.href}
								className={`nav-item ${isActive(item.href, item.exact) ? "active" : ""}`}
								aria-current={
									isActive(item.href, item.exact) ? "page" : undefined
								}
								onClick={onClose}
							>
								<MaterialSymbol name={item.icon} />
								<span>{item.label}</span>
							</Link>
						))}
					</div>
				))}
			</div>

			<div className="sidebar-footer">
				<div className="user-profile">
					<div className="user-avatar">
						{admin?.email?.[0]?.toUpperCase() || "A"}
					</div>
					<div className="user-info">
						<div className="user-name">{admin?.email || "管理員"}</div>
						<div className="user-role">平台管理員</div>
					</div>
					<button
						type="button"
						className="btn-reset"
						onClick={handleLogout}
						title="登出"
						aria-label="登出"
					>
						<MaterialSymbol name="logout" />
					</button>
				</div>
			</div>
		</>
	);

	const className = `sidebar${open ? " open" : ""}`;
	if (mobile) {
		return (
			<aside
				ref={ref}
				className={className}
				aria-label="平台管理主選單"
				aria-hidden={!open ? true : undefined}
				aria-modal={open ? true : undefined}
				inert={!open ? true : undefined}
				role="dialog"
				tabIndex={-1}
			>
				{sidebarContent}
				{/* 抽屜是最上層的 modal 時，toast 寫進這裡（aria-modal 面板之外的內容可能不進無障礙樹）。 */}
				{open && <ToastRegions />}
			</aside>
		);
	}

	return (
		<aside ref={ref} className={className} aria-label="平台管理主選單">
			{sidebarContent}
		</aside>
	);
});

export default Sidebar;
