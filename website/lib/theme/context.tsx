"use client";

import { useTheme as useNextTheme } from "next-themes";
import * as React from "react";

/**
 * Site-wide light/dark theme.
 *
 * Compatibility layer over next-themes.
 *
 * One source of truth: next-themes reads/writes the `data-theme` attribute on `<html>`.
 * - Buyer + seller surfaces both consume the same attribute, so the toggle
 *   in the top navbar pill flips everything that scopes off
 *   `[data-theme="dark"]`.
 * - The initial paint is handled by an inline beforeInteractive script in
 *   `components/shared/RootDocument.tsx` (THEME_BOOT_SCRIPT below) so the page never flashes
 *   the wrong theme on first load.
 */

export type Theme = "light" | "dark";
export const STORAGE_KEY = "hk:theme";

interface ThemeContextValue {
	theme: Theme;
	toggle: () => void;
	setTheme: (t: Theme) => void;
}

/**
 * Deprecated compatibility wrapper. AppProviders owns the actual next-themes
 * provider; this remains only so older imports don't crash while callers are
 * migrated.
 */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
	return <>{children}</>;
}

export function useTheme(): ThemeContextValue {
	const { resolvedTheme, setTheme: setNextTheme, theme } = useNextTheme();
	const currentTheme =
		theme === "dark" || resolvedTheme === "dark" ? "dark" : "light";

	const setTheme = React.useCallback(
		(next: Theme) => {
			setNextTheme(next);
		},
		[setNextTheme],
	);

	const toggle = React.useCallback(() => {
		setTheme(currentTheme === "dark" ? "light" : "dark");
	}, [currentTheme, setTheme]);

	return {
		theme: currentTheme,
		toggle,
		setTheme,
	};
}

/**
 * Inline script string for `<Script strategy="beforeInteractive">`.
 *
 * Reads localStorage synchronously and writes `data-theme` to <html>
 * before React hydrates, so the cream → primary swap doesn't flash.
 */
export const THEME_BOOT_SCRIPT = `try {
  var v = localStorage.getItem(${JSON.stringify(STORAGE_KEY)});
  var theme = v === 'dark' ? 'dark' : 'light';
  document.documentElement.setAttribute('data-theme', theme);
  document.documentElement.style.colorScheme = theme;
} catch (e) {}`;
