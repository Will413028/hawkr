import { createNavigation } from "next-intl/navigation";
import { routing } from "@/i18n/routing";
import type { AppLocale } from "./locales";
import {
	isLocalizablePagePath,
	isRootRelativeHref,
	splitLocale,
} from "./paths";

/**
 * 導覽層（ADR 2026-10-17 D1，`as-needed`）：站內所有「去哪裡」都經過這裡，路徑在程式裡一律是不含語系的站內路徑
 * （`/account/orders`），出口才加上讀者的語系前綴（zh-TW 沒有前綴，`en` 是 `/en/…`）。是否加前綴由 next-intl 的
 * `createNavigation(routing)` 決定，這裡沒有第二套規則。
 *
 * 這個檔案 server 與 client 共用，所以沒有 `"use client"`，也不 import 任何只能在 client 用的 hook：
 *   - server：`redirect`（`{ href, locale }`，語系取自路由參數）
 *   - client：`usePathname`（不含語系前綴的路徑）
 *   - 兩邊：`localizeHref`（把站內路徑變成瀏覽器要去的網址；反方向是 `lib/intl/paths.ts` 的 `stripLocale`）
 * 需要 hook 的出口在 `lib/intl/use-router.ts`（`useRouter`）與 `lib/intl/routed-locale.tsx`（`useRoutedLocale`）。
 * 不用 next-intl 的 `Link` 與 `useRouter`：站內連結走 `components/shared/Link.tsx`（View Transition、預取、
 * 導覽觀測），它自己呼叫 `localizeHref`；next-intl 的 `useRouter` 每換一次路徑就回傳新的物件、而且對已經帶前綴的
 * 網址會再加一次前綴，這裡的 `useRouter` 身分穩定、可重複套用。
 *
 * 用法的規則（`tests/navigation-layer.test.ts` 擋）：元件不直接 import `next/link`、`next/navigation` 的
 * `useRouter`／`redirect`／`usePathname`，也不手拼 `/en`。
 */
const navigation = createNavigation(routing);

const { getPathname } = navigation;
export const { redirect, usePathname } = navigation;

/**
 * `localizeHref` for a link target that may be missing or a `UrlObject`: only a string is an app path (nobody in
 * this codebase passes an object), everything else is handed to `next/link` as it is.
 */
export function localizeTarget<T>(target: T, locale: AppLocale): T {
	return (
		typeof target === "string" ? localizeHref(target, locale) : target
	) as T;
}

/**
 * 站內路徑 → 瀏覽器要去的網址，在 `locale` 這個語系下：`/account` → `/account`（zh-TW）或 `/en/account`（en）。
 *
 * 可重複套用：已經帶語系前綴的路徑保留它自己的語系（`/en/account` 不會變成 `/en/en/account`），
 * 所以出口不必知道呼叫端傳來的是哪一種。不是站內頁面路徑的原樣放行：外部網址、`#hash`、`?query`、
 * `mailto:`、`//host`、`/api/…`、`/adm/…`、明列的檔案（`isLocalizablePagePath`）。
 *
 * 絕對網址一律不動，同源的也一樣（`https://hawkr.example/account`）：寫成絕對網址的人已經選好了
 * 確切的位址（付款的回跳、分享連結、`absoluteUrl(path, locale)` 組出來的），這裡只處理站內路徑。
 *
 * `?query` 與 `#hash` 原樣接在加好前綴的路徑後面，不經 next-intl：`/#top` → `/en#top`，`/?q=1` → `/en?q=1`。
 */
export function localizeHref(href: string, locale: AppLocale): string {
	if (!isRootRelativeHref(href)) return href;
	const cut = href.search(/[?#]/);
	const path = cut === -1 ? href : href.slice(0, cut);
	const rest = cut === -1 ? "" : href.slice(cut);
	const explicit = splitLocale(path);
	const base = explicit.locale ? explicit.pathname : path;
	if (!isLocalizablePagePath(base)) return href;
	return `${getPathname({ href: base, locale: explicit.locale ?? locale })}${rest}`;
}
