import { activeLocale } from "@/lib/intl/active-locale";
import { localizeHref } from "@/lib/intl/navigation";

/**
 * A navigation target as the browser will show it: an app path (`/account`) gets the page's
 * language (`/en/account` on an English page), an absolute URL or an already-prefixed path stays
 * as it is. Browser-only (it reads the page's active locale).
 */
export function toBrowserHref(target: string): string {
	return localizeHref(target, activeLocale());
}

/** `toBrowserHref` as a URL; null when `target` is not a URL at all. Browser-only. */
export function resolveNavigationUrl(
	target: string,
	base: string = window.location.href,
): URL | null {
	try {
		return new URL(toBrowserHref(target), base);
	} catch {
		return null;
	}
}
