"use client";

import { RouterProvider } from "@heroui/react";
import { useLocalizedHref } from "@/lib/intl/use-router";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

/**
 * react-aria's router bridge for the HeroUI components that take an `href`: `navigate` is called with the
 * path the caller wrote (the observed router puts the page's language on it), and `useHref` is what the
 * component renders into its `href` attribute, so the anchor already points at the right language before
 * hydration and for "open in new tab".
 */
export function UIProvider({ children }: { children: React.ReactNode }) {
	const router = useObservedRouter("heroui_provider");
	return (
		<RouterProvider navigate={router.push} useHref={useLocalizedHref}>
			{children}
		</RouterProvider>
	);
}

/** The file is named after the provider; make that name importable too, so a
 *  call site does not have to know the export is spelled differently. */
export const HeroUIProvider = UIProvider;
export default UIProvider;
