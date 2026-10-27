"use client";

import Link from "@/components/shared/Link";
import SellerAvatarImage from "@/components/shared/SellerAvatarImage";
import SellerSidebarIcon from "@/components/shared/SellerSidebarIcon";
import Button from "@/components/ui/Button";
import {
	type NavItem,
	SELLER_FOOTER_ITEMS,
	SELLER_NAV_GROUPS,
} from "@/features/seller-center/components/sellerNav";
import { usePathname } from "@/lib/intl/navigation";
import { delegatedHref, isDelegatedHiddenHref } from "../nav-policy";
import { useDelegatedSession } from "../state/DelegatedSessionContext";

/**
 * 代行工作區的內側欄：**沿用賣家中心既有的選單資料**，只做三件事
 * （§6.1.14／定案第 3 條）：
 * 1. 濾掉禁區項（`nav-policy.ts` 的 `DELEGATED_HIDDEN_HREFS`，唯一要手動維護的清單）
 * 2. 連結改指向 `/adm/{storeId}/*`
 * 3. 「登出」改為「結束代行」
 *
 * 刻意**不渲染任何 badge**：badge 的資料來源是 dashboard／products／posts 三支
 * 輪詢，代行下不但沒有對應端點，打下去還會在稽核表製造一批 `*_READ` ＋ FAILED
 * 的列，把「有人試圖跨店讀取」這個安全訊號淹掉。
 *
 * 身分卡的頭像刻意是空 slot：`GET current` 的 target 沒有頭像，而為了畫一顆頭像
 * 去打 storefront 端點會在每一頁載入時多落一筆 `STOREFRONT_READ` 稽核列，而且
 * `storefront.read` 是可以不核發的 capability（沒給就只有側欄壞掉）。
 */
export default function DelegatedSellerSidebar({
	storeId,
	onRequestEnd,
}: {
	storeId: string;
	onRequestEnd: () => void;
}) {
	const { summary, ending } = useDelegatedSession();
	const pathname = usePathname() ?? "";
	const groups = SELLER_NAV_GROUPS.map((group) => ({
		heading: group.heading,
		items: group.items.filter((item) => !isDelegatedHiddenHref(item.href)),
	})).filter((group) => group.items.length > 0);
	const footerItems = SELLER_FOOTER_ITEMS.filter(
		(item) => !isDelegatedHiddenHref(item.href),
	);
	const allItems = [...groups.flatMap((group) => group.items), ...footerItems];

	return (
		<aside className="seller-sidebar" aria-hidden={false}>
			<div className="seller-sidebar__profile">
				<div className="seller-sidebar__avatar" aria-hidden="true">
					<SellerAvatarImage src={null} alt="" variant="identity" />
				</div>
				<div className="seller-sidebar__profile-meta">
					<strong title={summary?.target.storeName ?? undefined}>
						{summary?.target.storeName ?? ""}
					</strong>
					<small>賣家 ID：{storeId}</small>
				</div>
			</div>

			<nav className="seller-sidebar__nav" aria-label="代行工作區導覽">
				{groups.map((group) => (
					<div key={group.heading} className="seller-sidebar__group">
						<div className="seller-sidebar__heading">{group.heading}</div>
						{group.items.map((item) => (
							<DelegatedNavLink
								key={item.href}
								item={item}
								storeId={storeId}
								pathname={pathname}
								allItems={allItems}
							/>
						))}
					</div>
				))}
			</nav>

			<div className="seller-sidebar__footer">
				{footerItems.map((item) => (
					<DelegatedNavLink
						key={item.href}
						item={item}
						storeId={storeId}
						pathname={pathname}
						allItems={allItems}
					/>
				))}
				{/* 「登出」＝結束本次代行（B16）。⚠️ 不得接任何 ADM 登出端點：那會污染
				    稽核的 revoke_reason 判讀，也會把操作者踢回登入頁。 */}
				<Button
					type="button"
					variant="light"
					color="danger"
					className="seller-sidebar__item seller-sidebar__item--signout h-auto justify-start"
					disabled={ending}
					onClick={onRequestEnd}
				>
					<span className="seller-sidebar__icon" aria-hidden="true">
						<SellerSidebarIcon name="logout" />
					</span>
					<span className="seller-sidebar__label">結束代行</span>
				</Button>
			</div>
		</aside>
	);
}

function DelegatedNavLink({
	item,
	storeId,
	pathname,
	allItems,
}: {
	item: NavItem;
	storeId: string;
	pathname: string;
	allItems: NavItem[];
}) {
	const href = delegatedHref(item.href, storeId);
	const isActive = isActiveDelegatedItem(item, storeId, pathname, allItems);
	return (
		<Link
			href={href}
			className={
				isActive
					? "seller-sidebar__item seller-sidebar__item--active"
					: "seller-sidebar__item"
			}
			aria-current={isActive ? "page" : undefined}
		>
			<span className="seller-sidebar__icon" aria-hidden="true">
				{item.icon}
			</span>
			<span className="seller-sidebar__label">{item.label}</span>
		</Link>
	);
}

/** Longest-match：明細路由高亮它的父項，巢狀兄弟項不會雙重高亮。 */
function isActiveDelegatedItem(
	item: NavItem,
	storeId: string,
	pathname: string,
	allItems: NavItem[],
): boolean {
	const target = delegatedHref(item.href, storeId);
	if (!matches(target, pathname)) return false;
	return !allItems.some((other) => {
		if (other === item) return false;
		const otherTarget = delegatedHref(other.href, storeId);
		return matches(otherTarget, pathname) && otherTarget.length > target.length;
	});
}

function matches(target: string, pathname: string): boolean {
	return pathname === target || pathname.startsWith(`${target}/`);
}
