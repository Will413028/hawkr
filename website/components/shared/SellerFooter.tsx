import BrandLogo from "@/components/shared/BrandLogo";
import FooterBottom from "@/components/shared/FooterBottom";
import {
	FOOTER_BRAND_LOGO,
	FOOTER_BRAND_TAGLINE,
	FOOTER_HEADING,
	FOOTER_HEADING_DOT,
	FOOTER_LINK_LIST,
	FOOTER_ROOT,
	FOOTER_SHELL,
	FOOTER_TEXT_LINK,
} from "@/components/shared/footerChrome";
import Link from "@/components/shared/Link";
import SiteBackgroundToggle from "@/components/shared/SiteBackgroundToggle";
import TextScaleToggle from "@/components/shared/TextScaleToggle";

/**
 * Footer for the seller workspace (/seller-center/*). Deliberately distinct
 * from the buyer Footer: SellerWorkspaceLock confines sellers to the
 * workspace, so this only links to seller-center destinations plus the
 * universal legal/help pages (allowlisted in SellerWorkspaceLock). No buyer
 * storefront links — those would just bounce back to the dashboard.
 *
 * Shared chrome (classes + footer bottom) lives in footerChrome.ts /
 * FooterBottom.tsx so the two footers cannot drift apart again.
 */
const workspaceLinks = [
	{ label: "賣家總覽", href: "/seller-center/dashboard" },
	{ label: "商品管理", href: "/seller-center/products" },
	{ label: "訂單管理", href: "/seller-center/orders" },
	{ label: "動態管理", href: "/seller-center/posts" },
	{ label: "商店設定", href: "/seller-center/me" },
];

const supportLinks = [
	{ label: "賣家操作說明", href: "/seller-guide" },
	{ label: "合作方案", href: "/seller/pricing" },
	{ label: "常見問題", href: "/faq" },
	{ label: "賣家契約", href: "/seller-terms" },
	{ label: "退換貨須知", href: "/return-policy" },
	{ label: "聯絡我們", href: "/contact" },
	{ label: "關於我們", href: "/about" },
];

export default function SellerFooter() {
	return (
		<footer className={FOOTER_ROOT} role="contentinfo" aria-label="頁尾">
			<div
				className={`${FOOTER_SHELL} flex justify-between gap-x-20 gap-y-8 max-[960px]:flex-col max-[960px]:gap-y-8`}
			>
				{/* Brand column */}
				<div className="max-[960px]:min-w-0 max-w-[320px] max-[960px]:max-w-none">
					<BrandLogo tone="light" className={FOOTER_BRAND_LOGO} />
					<p className={FOOTER_BRAND_TAGLINE}>賣家中心 · 經營你的線上商店。</p>
					<a
						href="mailto:supports@example.com"
						className="inline-flex min-h-11 items-center !text-hk-cream/70 text-[length:var(--text-sm)] leading-[22.75px] tracking-[-0.15px] no-underline transition-colors duration-200 motion-reduce:transition-none hover:!text-hk-cream"
					>
						supports@example.com
					</a>
					<SiteBackgroundToggle />
					{/* mt-2 keeps the 8px minimum between adjacent touch targets —
					    `.site-background-control` above it declares the same gap. */}
					<TextScaleToggle className="mt-2" />
				</div>

				{/* Link columns grouped together on the right */}
				<div className="flex gap-20 max-[960px]:gap-12 max-[560px]:grid max-[560px]:grid-cols-2 max-[560px]:gap-x-3 max-[560px]:gap-y-[18px]">
					{/* Workspace column */}
					<div className="max-[960px]:min-w-0">
						<h2 className={FOOTER_HEADING}>
							<span className={FOOTER_HEADING_DOT} />
							賣家中心
						</h2>
						<ul className={FOOTER_LINK_LIST}>
							{workspaceLinks.map((item) => (
								<li key={item.href}>
									<Link href={item.href} className={FOOTER_TEXT_LINK}>
										{item.label}
									</Link>
								</li>
							))}
						</ul>
					</div>

					{/* Support column */}
					<div className="max-[960px]:min-w-0">
						<h2 className={FOOTER_HEADING}>
							<span className={FOOTER_HEADING_DOT} />
							協助與支援
						</h2>
						<ul className={FOOTER_LINK_LIST}>
							{supportLinks.map((item) => (
								<li key={item.href}>
									<Link href={item.href} className={FOOTER_TEXT_LINK}>
										{item.label}
									</Link>
								</li>
							))}
						</ul>
					</div>
				</div>
			</div>

			<FooterBottom />
		</footer>
	);
}
