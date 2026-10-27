"use client";

import NextLink from "next/link";
import type * as React from "react";
import { localizeTarget } from "./navigation";
import { useRoutedLocale } from "./routed-locale";

/**
 * `next/link` with the page's language in its href, and nothing else changed.
 *
 * The shared link (`components/shared/Link.tsx`) is a different component, not a faster `next/link`: it
 * navigates through a View Transition, prefetches on hover and focus only (`prefetch={false}` underneath),
 * declares the navigation to the observability layer and warms the destination's data first. A link that has
 * to keep `next/link`'s own behavior — its viewport prefetch above all, or no transition (the chat sidebar
 * rows, see tests/conversation-sidebar-navigation.test.ts) — is this one. The only thing it adds is the
 * prefix, so the zh-TW site renders and behaves exactly as `next/link` did.
 */
export default function NativeLink({
	href,
	as,
	...rest
}: React.ComponentProps<typeof NextLink>) {
	const locale = useRoutedLocale();
	return (
		<NextLink
			{...rest}
			href={localizeTarget(href, locale)}
			as={localizeTarget(as, locale)}
		/>
	);
}
