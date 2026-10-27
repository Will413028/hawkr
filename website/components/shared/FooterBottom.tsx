import {
	FOOTER_POLICY_LINK,
	FOOTER_SHELL,
} from "@/components/shared/footerChrome";
import Link from "@/components/shared/Link";

/**
 * Copyright + policy links. Rendered verbatim by both site footers; it used to
 * be a duplicated 27-line block in each.
 *
 * The inner `<nav aria-label="政策">` stays a landmark on purpose — the policy
 * row IS navigation, and the enclosing `<footer aria-label="頁尾">` names the
 * outer landmark so a screen-reader landmark list reads 頁尾 › 政策 instead of
 * two unnamed regions.
 */
export default function FooterBottom() {
	return (
		<div
			className={`${FOOTER_SHELL} flex items-center justify-between pt-2 border-t border-hk-cream/15 max-[960px]:flex-col max-[960px]:items-start max-[960px]:gap-2 max-[560px]:flex-row max-[560px]:items-center max-[560px]:justify-between max-[560px]:gap-3`}
		>
			<div className="flex items-center gap-2.5 text-hk-cream/85 text-[length:var(--text-base)] leading-6 max-[960px]:flex-wrap max-[960px]:text-[0.8125rem] max-[560px]:flex-1 max-[560px]:min-w-0 max-[560px]:gap-2 max-[560px]:text-xs max-[560px]:leading-normal max-[560px]:text-hk-cream/50">
				<span>&copy; Hawkr 示範專案 2026 All rights reserved.</span>
			</div>
			<nav
				aria-label="政策"
				className="flex items-center gap-3 max-[560px]:gap-2"
			>
				<Link href="/user-policy" className={FOOTER_POLICY_LINK}>
					使用者政策
				</Link>
				<span
					aria-hidden="true"
					className="h-3 w-px bg-hk-cream/25 max-[560px]:h-2.5"
				/>
				<Link href="/privacy" className={FOOTER_POLICY_LINK}>
					隱私權政策
				</Link>
				<span
					aria-hidden="true"
					className="h-3 w-px bg-hk-cream/25 max-[560px]:h-2.5"
				/>
				<Link href="/terms" className={FOOTER_POLICY_LINK}>
					服務條款
				</Link>
			</nav>
		</div>
	);
}
