import AppDownloadCTA from "@/components/shared/AppDownloadCTA";
import AppIcon from "@/components/shared/AppIcon";
import BrandLogo from "@/components/shared/BrandLogo";
import FooterBottom from "@/components/shared/FooterBottom";
import FooterLoginLink from "@/components/shared/FooterLoginLink";
import {
	FOOTER_BRAND_LOGO,
	FOOTER_BRAND_TAGLINE,
	FOOTER_COLUMN,
	FOOTER_CONTACT_ICON,
	FOOTER_CONTACT_LINK,
	FOOTER_HEADING,
	FOOTER_HEADING_DOT,
	FOOTER_ICON_LINK,
	FOOTER_LINK_LIST,
	FOOTER_ROOT,
	FOOTER_SHELL,
	FOOTER_SUMMARY,
	FOOTER_SUMMARY_CHEVRON,
	FOOTER_TEXT_LINK,
} from "@/components/shared/footerChrome";
import Link from "@/components/shared/Link";
import SiteBackgroundToggle from "@/components/shared/SiteBackgroundToggle";
import TextScaleToggle from "@/components/shared/TextScaleToggle";

/**
 * Buyer footer. Server Component: only the toggles, the login link and the two
 * data-driven columns (`FooterCategoryColumns`) are client islands.
 *
 * Column order — 探索 · 商品分類 · 熱門話題 · 關於與服務 · 聯絡 — puts the two
 * shopping entry points next to the navigation column they extend. Each column
 * is a native `<details open>`; ≤560px it collapses into an accordion (five
 * columns do not fit a phone side by side), above that the disclosure is inert
 * and the column renders as a plain block. Brand actions share one compact
 * row of circular controls.
 */
export default function Footer({
	categoryColumns,
}: {
	/** Server-rendered 商品分類 / 熱門話題 columns (see RoleAwareFooter). */
	categoryColumns: React.ReactNode;
}) {
	return (
		<footer className={FOOTER_ROOT} role="contentinfo" aria-label="頁尾">
			<div
				className={`${FOOTER_SHELL} grid grid-cols-[minmax(200px,1.5fr)_repeat(3,minmax(0,1fr))_minmax(112px,1fr)_minmax(220px,1.5fr)] items-start gap-x-6 gap-y-6 max-[1200px]:grid-cols-4 max-[768px]:grid-cols-2 max-[560px]:grid-cols-1 max-[560px]:gap-x-3 max-[560px]:gap-y-1`}
			>
				{/* Brand column */}
				<div className="min-w-0 max-w-[300px] max-[1200px]:max-w-none max-[1200px]:col-span-full max-[560px]:pb-4">
					<BrandLogo tone="light" className={FOOTER_BRAND_LOGO} />
					<p className={FOOTER_BRAND_TAGLINE}>台灣小農與在地職人的線上市集。</p>
					<div className="flex items-center gap-2">
						<a
							href="https://www.facebook.com/profile.php?id=61582379055464"
							target="_blank"
							rel="noopener noreferrer"
							className={FOOTER_ICON_LINK}
							aria-label="Facebook"
							title="Facebook"
						>
							<svg
								width="20"
								height="20"
								viewBox="0 0 24 24"
								fill="currentColor"
								aria-hidden="true"
								className="w-5 h-5"
							>
								<path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z" />
							</svg>
							<span className="sr-only">Facebook</span>
						</a>
						<AppDownloadCTA variant="icon" />
						<SiteBackgroundToggle variant="icon" />
						<TextScaleToggle variant="footer" />
					</div>
				</div>

				{/* Quick links column */}
				<details className={FOOTER_COLUMN} open>
					<summary className={FOOTER_SUMMARY}>
						<h2 className={FOOTER_HEADING}>
							<span className={FOOTER_HEADING_DOT} />
							快速連結
						</h2>
						<AppIcon
							name="chevron-down"
							size={18}
							className={FOOTER_SUMMARY_CHEVRON}
						/>
					</summary>
					<ul className={FOOTER_LINK_LIST}>
						<li>
							<Link href="/" className={FOOTER_TEXT_LINK}>
								首頁
							</Link>
						</li>
						<li>
							<Link href="/feed" className={FOOTER_TEXT_LINK}>
								Hawkr 動態
							</Link>
						</li>
						<li>
							<Link href="/market/products" className={FOOTER_TEXT_LINK}>
								Hawkr 市集
							</Link>
						</li>
						<li>
							<Link href="/search" className={FOOTER_TEXT_LINK}>
								搜尋
							</Link>
						</li>
					</ul>
				</details>

				{/* 商品分類 + 熱門話題 — the two data-driven columns. */}
				{categoryColumns}

				{/* Customer service column */}
				<details className={FOOTER_COLUMN} open>
					<summary className={FOOTER_SUMMARY}>
						<h2 className={FOOTER_HEADING}>
							<span className={FOOTER_HEADING_DOT} />
							客戶服務
						</h2>
						<AppIcon
							name="chevron-down"
							size={18}
							className={FOOTER_SUMMARY_CHEVRON}
						/>
					</summary>
					<ul className={FOOTER_LINK_LIST}>
						<li>
							<Link href="/return-policy" className={FOOTER_TEXT_LINK}>
								退換貨須知
							</Link>
						</li>
						<li>
							<Link href="/faq" className={FOOTER_TEXT_LINK}>
								常見問題
							</Link>
						</li>
						<FooterLoginLink className={FOOTER_TEXT_LINK} />
						<li>
							<Link href="/about" className={FOOTER_TEXT_LINK}>
								關於我們
							</Link>
						</li>
						<li>
							<Link href="/seller" className={FOOTER_TEXT_LINK}>
								招商合作
							</Link>
						</li>
						<li>
							<Link href="/seller/pricing" className={FOOTER_TEXT_LINK}>
								合作方案
							</Link>
						</li>
					</ul>
				</details>

				{/* Contact info column */}
				<details
					className={`${FOOTER_COLUMN} max-[1200px]:col-span-full max-[560px]:col-span-1`}
					open
				>
					<summary className={FOOTER_SUMMARY}>
						<h2 className={FOOTER_HEADING}>
							<span className={FOOTER_HEADING_DOT} />
							聯絡資訊
						</h2>
						<AppIcon
							name="chevron-down"
							size={18}
							className={FOOTER_SUMMARY_CHEVRON}
						/>
					</summary>
					<div className="flex flex-col gap-0">
						<a
							href="mailto:supports@example.com"
							className={FOOTER_CONTACT_LINK}
						>
							<span className={FOOTER_CONTACT_ICON}>
								<svg
									width="16"
									height="16"
									viewBox="0 0 24 24"
									fill="none"
									stroke="var(--color-neutral-cream)"
									strokeWidth="2"
									aria-hidden="true"
								>
									<path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
									<polyline points="22,6 12,13 2,6" />
								</svg>
							</span>
							<span className="max-[960px]:min-w-0">supports@example.com</span>
						</a>
						<Link href="/contact" className={FOOTER_CONTACT_LINK}>
							<span className={FOOTER_CONTACT_ICON}>
								<svg
									width="16"
									height="16"
									viewBox="0 0 24 24"
									fill="none"
									stroke="var(--color-neutral-cream)"
									strokeWidth="2"
									strokeLinecap="round"
									strokeLinejoin="round"
									aria-hidden="true"
								>
									<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
								</svg>
							</span>
							<span className="max-[960px]:min-w-0">聯絡我們</span>
						</Link>
					</div>
				</details>
			</div>

			<FooterBottom />
		</footer>
	);
}
