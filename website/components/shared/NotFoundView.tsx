"use client";

import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import StatusPageLayout, {
	STATUS_PAGE_GHOST_ACTION_CLASS,
	STATUS_PAGE_PRIMARY_ACTION_CLASS,
} from "@/components/shared/StatusPageLayout";

/**
 * Body of the app-wide 404, split out of app/[locale]/not-found.tsx so that route file
 * can be a server component and therefore export `metadata` — a 404 that
 * inherits the root title claims to be the home page in the tab, the history
 * entry and the share card.
 *
 * Shares `StatusPageLayout` with ErrorState so dead links and crashed segments
 * read as the same family, but without the retry button — there is nothing to
 * retry on a missing page, so both actions route the buyer onward.
 */
export default function NotFoundView() {
	return (
		<StatusPageLayout
			data-hk-route-terminal-ready="not-found"
			discClassName="p-10"
			media={
				<AppIcon
					name="search"
					size={216}
					className="size-full"
					style={{ color: "var(--not-found-icon-color)" }}
				/>
			}
			eyebrow="404"
			title="這頁不在貨架上"
			description={
				<>
					可能是連結過期，或商品已經下架。
					<br />
					回市集逛逛，看今天有什麼新鮮的。
				</>
			}
			actions={
				<>
					<Link
						href="/market/products"
						className={STATUS_PAGE_PRIMARY_ACTION_CLASS}
					>
						逛逛市集
					</Link>
					<Link href="/" className={STATUS_PAGE_GHOST_ACTION_CLASS}>
						回到首頁
					</Link>
				</>
			}
		/>
	);
}
