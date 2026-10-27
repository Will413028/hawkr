import { stripLocale } from "@/lib/intl/paths";

/**
 * Match a Next.js pathname against an internal link target.
 *
 * `usePathname()` never includes the query string or hash, while navigation
 * links commonly do (for example `/login?role=buyer`). Compare pathname parts
 * only so those links can still own the active navigation state.
 */
export function isNavPathActive(rawPathname: string, href: string): boolean {
	const pathname = stripLocale(rawPathname);
	const hrefPathname = stripLocale(href.split(/[?#]/, 1)[0] || "/");
	if (hrefPathname === "/") return pathname === "/";
	return pathname === hrefPathname || pathname.startsWith(`${hrefPathname}/`);
}
