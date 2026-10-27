"use client";

import type { LinkProps } from "next/link";
import type { ComponentPropsWithoutRef, MouseEvent, ReactNode } from "react";
import Link from "@/components/shared/Link";
import { canGoBackInApp } from "@/lib/navigation/history-marker";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

type AnchorProps = Omit<
	ComponentPropsWithoutRef<"a">,
	keyof LinkProps | "onClick" | "children"
>;

interface BackLinkProps extends AnchorProps {
	fallbackHref: LinkProps["href"];
	children?: ReactNode;
	prefetch?: LinkProps["prefetch"];
}

/**
 * Anchor that pops the browser history on plain click and only navigates to
 * `fallbackHref` when there is no in-app history to return to (direct deep-link
 * entry, opened in a new tab, etc.) or the user explicitly opens it in a new
 * tab/window via modifier keys.
 *
 * "In-app history" is decided by `lib/navigation/history-marker.ts`, not by
 * `history.length > 1` — that counted redirects and deep-link `replace()`s as
 * app pages and walked the user off the site. The marker also refuses to pop
 * back onto /login, /register and the checkout result pages, which redirect
 * arrivals straight back out.
 *
 * Stays an `<a>` so existing `body a.foo` CSS selectors keep matching and
 * right-click / ⌘-click / middle-click still get a real URL to open.
 */
export function BackLink({
	fallbackHref,
	children,
	prefetch,
	...rest
}: BackLinkProps) {
	const router = useObservedRouter("back_link");
	const handleClick = (e: MouseEvent<HTMLAnchorElement>) => {
		if (e.defaultPrevented) return;
		if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
		if (e.button !== 0) return;
		if (typeof window === "undefined") return;
		if (canGoBackInApp()) {
			e.preventDefault();
			router.back();
		}
	};
	return (
		<Link
			href={fallbackHref}
			prefetch={prefetch}
			onClick={handleClick}
			{...rest}
		>
			{children}
		</Link>
	);
}
