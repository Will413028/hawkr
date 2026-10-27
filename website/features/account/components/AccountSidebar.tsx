"use client";

import * as React from "react";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import AccountMenuIcon from "@/features/account/components/AccountMenuIcon";
import {
	BUYER_MENU_GROUPS,
	isAccountMenuActive,
} from "@/features/account/menu";
import { useAuth } from "@/lib/auth/context";
import { usePathname } from "@/lib/intl/navigation";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";

/**
 * Persistent left-rail navigation for `/account/*` pages.
 *
 * Mirrors the seller-center sidebar (.seller-sidebar) — same glass card
 * chrome, grouped nav with uppercase eyebrow headings, signature 44px
 * pill items with lift-on-hover. The two surfaces share the visual
 * grammar so users switching between buyer + seller modes feel like
 * they're in the same product family.
 *
 * Menu data comes from `BUYER_MENU_GROUPS` (groups) — same items as the
 * flat `BUYER_MENU_ITEMS` consumed by the navbar dropdown, just bucketed
 * into 購物 / 優惠與會員 / 互動 / 個人設定 sections.
 */
export default function AccountSidebar() {
	const pathname = usePathname() ?? "";
	const { identity, signOut } = useAuth();
	const [signingOut, setSigningOut] = React.useState(false);

	// TODO(B-189): profile endpoint — buyerName not on Identity; use nickname fallback.
	const displayName = identity?.nickname || "會員";
	const avatar = identity?.avatarKey;

	const handleSignOut = async () => {
		if (signingOut) return;
		setSigningOut(true);
		try {
			await signOut();
		} catch {
			// 撤銷失敗＝session 仍有效（signOut 已跳 toast），維持登入畫面。
			setSigningOut(false);
			return;
		}
		// 整頁重載回首頁，避免部署後 soft 導航去要舊 chunk 卡骨架。
		commitDocumentNavigation("/");
	};

	const baseItemClass =
		"group flex items-center gap-3 min-h-[44px] py-[9px] px-3 rounded-hk-lg border border-transparent bg-transparent text-foreground-muted dark:text-hk-ink-100/78 text-sm font-medium leading-tight cursor-pointer w-full text-left transition-all duration-160 ease-out hover:bg-white/46 dark:hover:bg-[#f5dea5]/8 hover:border-white/58 dark:hover:border-[#f5dea5]/16 hover:shadow-[inset_0_1px_0_rgba(255,255,255,0.7),0_10px_24px_rgb(var(--color-ink-800-rgb)/0.08)] hover:text-foreground dark:hover:text-[#f5e9c3] hover:-translate-y-px";

	const activeItemClass =
		"font-bold bg-[linear-gradient(135deg,rgb(var(--color-ink-100-rgb)/0.78),rgba(255,255,255,0.46))] bg-white/36 dark:bg-[linear-gradient(135deg,rgba(241,205,112,0.22),rgba(245,222,165,0.08))] border-hk-ink-500/16 dark:border-[#f1cd70]/32 shadow-[inset_0_1px_0_rgba(255,255,255,0.76),0_12px_26px_rgb(var(--color-ink-600-rgb)/0.12)] text-hkui-primary-700 dark:text-[#f1cd70]";

	return (
		<aside
			className="sticky top-5 isolate bg-gradient-to-br from-white/70 to-[#fffdf4]/46 bg-white/36 dark:bg-gradient-to-br dark:from-hk-ink-950/86 dark:to-hk-ink-900/72 dark:bg-hk-ink-950/60 border border-white/72 dark:border-[#f5dea5]/14 rounded-[36px] shadow-[inset_0_1px_0_rgba(255,255,255,0.78),inset_0_-1px_0_rgba(255,255,255,0.22),0_28px_70px_rgb(var(--color-ink-800-rgb)/0.12),0_10px_30px_rgb(var(--color-ink-900-rgb)/0.08)] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06),0_28px_70px_rgba(0,0,0,0.5),0_10px_30px_rgba(0,0,0,0.32)] backdrop-blur-[26px] backdrop-saturate-[158%] p-[22px] flex flex-col gap-4 h-fit min-h-[min(calc(100dvh-72px),940px)] font-sans max-[960px]:hidden before:absolute before:inset-[1px] before:z-0 before:pointer-events-none before:content-[''] before:rounded-[35px] before:bg-[linear-gradient(135deg,rgba(255,255,255,0.68),rgba(255,255,255,0)_42%),radial-gradient(circle_at_16%_4%,rgb(var(--color-status-success-rgb)/0.18),rgb(var(--color-status-success-rgb)/0)_24%),radial-gradient(circle_at_94%_28%,rgba(223,176,70,0.13),rgba(223,176,70,0)_32%)] dark:before:bg-[linear-gradient(135deg,rgba(245,222,165,0.08),rgba(245,222,165,0)_42%),radial-gradient(circle_at_16%_4%,rgb(var(--color-ink-600-rgb)/0.35),rgb(var(--color-ink-600-rgb)/0)_26%),radial-gradient(circle_at_94%_28%,rgba(223,176,70,0.16),rgba(223,176,70,0)_32%)]"
			aria-label="會員中心導覽"
		>
			<div className="relative z-1 flex items-center gap-3 pb-[14px] border-b border-b-[rgb(var(--color-ink-700-rgb)/0.1)] dark:border-b-[#f5dea5]/12 shadow-[0_1px_0_rgba(255,255,255,0.55)] dark:shadow-none">
				<Avatar
					src={avatar}
					alt={displayName}
					entity="buyer"
					className="size-11! shrink-0 rounded-[50%] object-cover bg-hk-ink-50"
				/>
				<div className="flex flex-col min-w-0">
					<strong className="text-base font-bold text-foreground dark:text-[#f5e9c3] truncate">
						{displayName}
					</strong>
					<small className="text-xs text-foreground-soft dark:text-hk-ink-100/60">
						{/* TODO(B-189): profile endpoint — phone not on Identity */}
						{"會員中心"}
					</small>
				</div>
			</div>

			<nav
				className="relative z-1 flex flex-col gap-3"
				aria-label="會員中心導覽"
			>
				{BUYER_MENU_GROUPS.map((group) => (
					<div key={group.heading} className="flex flex-col gap-[1px]">
						<div className="text-xs font-bold text-foreground-soft dark:text-hk-ink-100/50 tracking-[0.04em] px-2">
							{group.heading}
						</div>
						{group.items.map((item) => {
							const active = isAccountMenuActive(pathname, item.href);
							return (
								<Link
									key={item.href}
									href={item.href}
									aria-current={active ? "page" : undefined}
									className={
										active
											? `${baseItemClass} ${activeItemClass}`
											: baseItemClass
									}
								>
									<span
										className="inline-flex items-center justify-center w-[22px] h-[22px] shrink-0 text-current transition-colors duration-150"
										aria-hidden="true"
									>
										<AccountMenuIcon name={item.icon} />
									</span>
									<span className="flex-1 min-w-0 truncate">{item.label}</span>
								</Link>
							);
						})}
					</div>
				))}
			</nav>

			<div className="relative z-1 mt-auto pt-3 border-t border-t-[rgb(var(--color-ink-700-rgb)/0.1)] dark:border-t-[#f5dea5]/12">
				<Button
					type="button"
					className="group flex items-center justify-start gap-3 min-h-[44px] py-[9px] px-3 rounded-hk-lg border border-transparent bg-transparent text-danger dark:text-[#f5b97a] text-sm font-medium leading-tight cursor-pointer w-full text-left transition-all duration-160 ease-out hover:bg-white/52 dark:hover:bg-[#f5b97a]/12 hover:border-[#ff5c2b]/16 dark:hover:border-[#f5b97a]/22 hover:shadow-[inset_0_1px_0_rgba(255,255,255,0.68),0_10px_22px_rgba(193,56,56,0.1)] hover:text-danger dark:hover:text-[#f5b97a] hover:-translate-y-px"
					disabled={signingOut}
					isLoading={signingOut}
					loadingLabel="登出帳號中…"
					onClick={() => void handleSignOut()}
				>
					<span
						className="inline-flex items-center justify-center w-[22px] h-[22px] shrink-0 text-current transition-colors duration-150"
						aria-hidden="true"
					>
						<IconSignOut />
					</span>
					<span className="flex-1 min-w-0 truncate">登出</span>
				</Button>
			</div>
		</aside>
	);
}

function IconSignOut() {
	return (
		<svg
			viewBox="0 0 24 24"
			width="22"
			height="22"
			fill="none"
			stroke="currentColor"
			strokeWidth="1.8"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
			<path d="m16 17 5-5-5-5" />
			<path d="M21 12H9" />
		</svg>
	);
}
