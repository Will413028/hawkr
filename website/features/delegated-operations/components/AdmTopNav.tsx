"use client";

import BrandLogo from "@/components/shared/BrandLogo";
import Link from "@/components/shared/Link";
import ThemeToggle from "@/components/shared/ThemeToggle";
import { usePathname } from "@/lib/intl/navigation";
import { useDelegatedSession } from "../state/DelegatedSessionContext";

/**
 * 頂部白色導覽（mockup §4.4.2）。
 *
 * 與賣家中心的 `SellerHeader` 刻意**不共用**：那一支讀 `useSellerIdentity()`／
 * `useAuth()`，還掛著通知鈴鐺與聊天入口 —— 在代行工作區這三件事分別是「拿不到
 * 資料」「不在授權範圍」「會打到管理員自己的帳號」。
 *
 * `＋ 上架新商品` 是**動作按鈕**不是模組入口，渲染條件因此是 capability 而不是
 * 「這一面有沒有開放」：缺 `products.write` 時**不渲染**（不是 disabled，L3857）——
 * 這個工作階段從頭到尾都建不了商品，一顆停用的按鈕仍在宣稱「這件事本來做得到」。
 *
 * ⚠️ 它與三樣東西是同一個集合，任一缺席就會變成「按下去說沒授權」的死按鈕，而
 * 那正是它上一輪被整顆移除的原因：① `delegated-repos.ts` 的 `createProduct` 真的
 * 接上 `POST .../products` ② `/adm/{storeId}/products/new` 有具名路由（否則落進
 * `[...area]` 空狀態頁）③ `ProductsPage` 的同一顆按鈕用同一個 capability 條件。
 * 三者由 `tests/delegated-workspace-faces.test.ts` 綁在一起。
 *
 * ⚠️ 上架能力（`products.publish`）**不是**這顆按鈕的條件：建立一律是未上架，沒有
 * 上架能力的工作階段照樣可以把商品建起來讓賣家自己上架。建立頁的「儲存並上架」
 * 才看 `products.publish`。
 * ⚠️ `數據分析` 渲染但不可操作（點入是空狀態頁）。
 */
const TOP_NAV_ITEMS = [
	{ segment: "dashboard", label: "賣家總覽" },
	{ segment: "products", label: "商品管理" },
	{ segment: "orders", label: "訂單管理" },
	{ segment: "customers", label: "客戶管理" },
	{ segment: "posts", label: "動態" },
	{ segment: "analytics", label: "數據分析" },
];

export default function AdmTopNav({ storeId }: { storeId: string }) {
	const pathname = usePathname() ?? "";
	const { hasCapability } = useDelegatedSession();
	const base = `/adm/${encodeURIComponent(storeId)}`;

	return (
		<header className="adm-topnav">
			<BrandLogo variant="horizontal" size="xs" />

			<nav className="adm-topnav__nav" aria-label="賣家工作區導覽">
				{TOP_NAV_ITEMS.map((item) => {
					const href = `${base}/${item.segment}`;
					const active = pathname === href || pathname.startsWith(`${href}/`);
					return (
						<Link
							key={item.segment}
							href={href}
							className={
								active
									? "adm-topnav__item adm-topnav__item--active"
									: "adm-topnav__item"
							}
							aria-current={active ? "page" : undefined}
						>
							{item.label}
						</Link>
					);
				})}
			</nav>

			<div className="adm-topnav__actions">
				{hasCapability("products.write") ? (
					<Link
						href={`${base}/products/new`}
						// ⚠️ `.adm-topnav__cta` 自帶實心森林綠底＋白字（0-1-0），但
						// `[data-theme="dark"] body a:not([data-hkui-button])`（0-2-2）會壓過它，
						// 把白字換成 `--color-ink-200`。`data-hkui-button` 是「這是按鈕、前景色由自己的
						// 表面決定」的既有宣告（`01-tokens-theme.css:761`）。
						// 少了它，這顆與 `ProductsPage` 那顆同名同義的「＋ 上架新商品」會在
						// **同一個畫面上**呈現兩種字色（那一顆已帶標記）。
						data-hkui-button=""
						className="adm-topnav__cta"
					>
						＋ 上架新商品
					</Link>
				) : null}
				<ThemeToggle />
			</div>
		</header>
	);
}
