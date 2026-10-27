"use client";

import * as React from "react";
import Link from "@/components/shared/Link";
import { usePathname } from "@/lib/intl/navigation";

type TabKey = "products" | "posts" | "messages" | "reviews";

const TABS: { key: TabKey; label: string; suffix: string }[] = [
	{ key: "products", label: "商品", suffix: "" },
	{ key: "posts", label: "貼文", suffix: "/posts" },
	{ key: "messages", label: "留言", suffix: "/messages" },
	{ key: "reviews", label: "評價", suffix: "/reviews" },
];

function tabFromPathname(pathname: string | null): TabKey {
	if (!pathname) return "products";
	if (/\/posts\/?$/.test(pathname)) return "posts";
	if (/\/messages\/?$/.test(pathname)) return "messages";
	if (/\/reviews\/?$/.test(pathname)) return "reviews";
	return "products";
}

/**
 * Storefront tab bar. Client-side so the active highlight (a) derives from
 * the URL while the header itself lives in the persistent (storefront)
 * layout, and (b) moves the instant a tab is clicked — optimistic, not
 * waiting for the route commit — so slow content never makes the tab bar
 * feel stuck.
 */
export default function SellerProfileTabs({
	sellerId,
	basePath,
}: {
	sellerId: string;
	/** Override only for the seller-center buyer-view preview. Public storefronts
	 * keep their canonical `/sellers/:id` routes. */
	basePath?: string;
}) {
	const pathname = usePathname() ?? "";
	const tabRoot = (basePath ?? `/sellers/${sellerId}`).replace(/\/$/, "");
	const routeTab = tabFromPathname(pathname);
	// Optimistic highlight: remember which tab was clicked AND on which URL.
	// The override only applies while we're still on that URL — the moment
	// the navigation commits (or back/forward fires) pathname moves and the
	// URL becomes the single source of truth again, no effect needed.
	const [pending, setPending] = React.useState<{
		tab: TabKey;
		from: string;
	} | null>(null);

	const activeTab =
		pending && pending.from === pathname ? pending.tab : routeTab;

	return (
		<div
			className="hkui-seller-profile__tabs"
			role="tablist"
			aria-label="商家頁切換"
		>
			{TABS.map((t) => (
				<Link
					key={t.key}
					href={`${tabRoot}${t.suffix}`}
					role="tab"
					aria-selected={activeTab === t.key}
					className={`hkui-seller-profile__tab${activeTab === t.key ? " is-active" : ""}`}
					onClick={() => setPending({ tab: t.key, from: pathname })}
				>
					{t.label}
				</Link>
			))}
		</div>
	);
}
