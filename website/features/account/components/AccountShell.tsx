"use client";

import type * as React from "react";
import { usePathname } from "@/lib/intl/navigation";
import AccountSidebar from "./AccountSidebar";

/**
 * Layout switcher for `/account/*`.
 *
 * Default: 2-col shell with the persistent left sidebar mirroring the
 * navbar's BuyerMenu (see `AccountSidebar.tsx`).
 *
 * Exception: `/account/messages*` — the chat page owns its own
 * conversation-list rail on the left, so the outer account sidebar is
 * suppressed to avoid stacking two sidebars side-by-side. Same
 * treatment as Telegram/LINE: a chat thread fills the page.
 */
const SIDEBAR_HIDDEN_PREFIXES = ["/account/messages"];

function shouldHideSidebar(pathname: string): boolean {
	return SIDEBAR_HIDDEN_PREFIXES.some(
		(prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
	);
}

export default function AccountShell({
	children,
}: {
	children: React.ReactNode;
}) {
	const pathname = usePathname() ?? "";
	if (!pathname) return null;
	if (shouldHideSidebar(pathname)) {
		// Chat owns its own layout end-to-end; render children directly so
		// `.chat-page` can claim the full content width.
		return <>{children}</>;
	}
	return (
		<div className="account-shell grid grid-cols-[256px_1fr] max-[960px]:grid-cols-[minmax(0,1fr)] gap-6 max-[960px]:gap-0 max-w-[1440px] mx-auto p-[6px_20px_48px] max-[960px]:p-[16px_clamp(12px,3vw,24px)_32px] items-stretch min-h-[calc(100dvh-40px)]">
			<AccountSidebar />
			{/* pt-6 gives every account inner page the same comfortable
			 * title breathing room as the order-detail page; individual
			 * screens should not re-add their own top padding. */}
			<div className="min-w-0 pt-6">{children}</div>
		</div>
	);
}
