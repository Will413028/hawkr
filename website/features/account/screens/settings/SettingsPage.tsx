import * as React from "react";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import TextScaleControl from "@/components/shared/TextScaleControl";
import ThemeToggle from "@/components/shared/ThemeToggle";
import DeleteAccountSection from "./DeleteAccountSection";
import SignInMethodsSection from "./SignInMethodsSection";
import { DISPLAY_ROW } from "./settings-row";

export const metadata = { title: "設定" };

/**
 * 顯示 rows（`DISPLAY_ROW`）carry a control instead of a link. This page stays a Server Component; the two
 * controls, the sign-in methods and the delete-account section are the only client islands on it.
 */

const SECTIONS = [
	{
		group: "帳號安全",
		items: [
			{
				label: "個人資料",
				href: "/account/profile",
				desc: "暱稱、大頭貼、生日、性別",
			},
			{
				label: "聯絡資訊",
				href: "/account/profile",
				desc: "手機 / Email 綁定",
			},
		],
	},
	{
		group: "通知",
		items: [
			{
				label: "通知偏好",
				href: "/account/notifications/preferences",
				desc: "訂單 / 優惠 / 訊息 / 系統 共 6 類事件",
			},
		],
	},
	{
		group: "購物與訂單",
		items: [
			{ label: "收件地址", href: "/account/addresses", desc: "最多 5 筆" },
			{ label: "我的訂單", href: "/account/orders", desc: "全部狀態" },
		],
	},
	{
		group: "我的活動",
		items: [
			{
				label: "我的優惠券",
				href: "/account/coupons",
				desc: "可用 / 已使用 / 已過期",
			},
			{ label: "會員卡", href: "/account/wallet", desc: "各商家累計消費" },
			{
				label: "收藏 / 追蹤",
				href: "/account/favorites",
				desc: "商品收藏、追蹤的小農",
			},
			{
				label: "瀏覽紀錄",
				href: "/account/recent",
				desc: "最近看過的商品 / 小農 / 貼文",
			},
		],
	},
	{
		group: "其他",
		items: [
			{ label: "我的評價", href: "/account/reviews", desc: "已寫 + 待評" },
			{ label: "客服中心", href: "/account/help", desc: "常見問題 + 聯絡客服" },
		],
	},
];

export default function Page() {
	return (
		<main className="settings-page">
			<SellerPageShell
				title="設定"
				subtitle="一站管理你的帳號、購物偏好、通知設定。"
			>
				<div className="flex flex-col gap-4">
					<section className="seller-card">
						<header className="seller-card__head">
							<h2>顯示</h2>
						</header>
						<ul className="list-none m-0 p-0 flex flex-col gap-2">
							<li className={DISPLAY_ROW}>
								<div className="flex flex-col gap-0.5">
									<strong className="text-sm font-semibold text-hk-heading">
										主題
									</strong>
									<small className="text-xs text-foreground-soft">
										日間 / 夜間外觀
									</small>
								</div>
								<ThemeToggle verbose />
							</li>
							<li className={DISPLAY_ROW}>
								<div className="flex flex-col gap-0.5">
									<strong className="text-sm font-semibold text-hk-heading">
										字體大小
									</strong>
									<small className="text-xs text-foreground-soft">
										小 / 中 / 大，只放大文字，版面不變
									</small>
								</div>
								<TextScaleControl />
							</li>
						</ul>
					</section>
					{SECTIONS.map((section) => (
						<React.Fragment key={section.group}>
							<section className="seller-card">
								<header className="seller-card__head">
									<h2>{section.group}</h2>
								</header>
								<ul className="list-none m-0 p-0 flex flex-col gap-2">
									{section.items.map((item) => (
										<li key={`${item.href}:${item.label}`}>
											<Link
												href={item.href}
												className="flex flex-col gap-0.5 p-3 px-4 bg-hk-surface-muted rounded-hk-md text-inherit no-underline transition-colors duration-200 ease-in-out hover:bg-hk-surface-panel"
											>
												<strong className="text-sm font-semibold text-hk-heading">
													{item.label}
												</strong>
												<small className="text-xs text-foreground-soft">
													{item.desc}
												</small>
											</Link>
										</li>
									))}
								</ul>
							</section>
							{section.group === "帳號安全" ? (
								<React.Suspense fallback={null}>
									<SignInMethodsSection />
								</React.Suspense>
							) : null}
						</React.Fragment>
					))}
					<DeleteAccountSection />
				</div>
			</SellerPageShell>
		</main>
	);
}
