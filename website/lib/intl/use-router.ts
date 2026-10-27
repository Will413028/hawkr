"use client";

import { useRouter as useNextRouter } from "next/navigation";
import * as React from "react";
import { localizeHref } from "./navigation";
import { useRoutedLocale } from "./routed-locale";

/** 站內路徑 → 這棵樹的語系下的網址，給沒有經過 router／Link 的出口用（`<form action>`）。 */
export function useLocalizedHref(href: string): string {
	return localizeHref(href, useRoutedLocale());
}

/**
 * App Router 的 router，`push`／`replace`／`prefetch` 收站內路徑（不含語系），出口加上讀者的語系前綴
 * （`localizeHref`）。已經帶前綴的網址與絕對網址原樣通過，所以重複套用沒有副作用。
 *
 * 不用 next-intl 的 `useRouter`：它依賴目前路徑，每次換頁都回傳新的物件，而這個專案有上百處把 router
 * 放進 effect／memo 的依賴；這裡回傳的物件只在 Next 的 router 或語系變時才換（Next 的 router 本身是穩定的）。
 */
export function useRouter() {
	const router = useNextRouter();
	const locale = useRoutedLocale();
	return React.useMemo(
		() => ({
			...router,
			push: (...[href, ...rest]: Parameters<typeof router.push>) =>
				router.push(localizeHref(href, locale), ...rest),
			replace: (...[href, ...rest]: Parameters<typeof router.replace>) =>
				router.replace(localizeHref(href, locale), ...rest),
			prefetch: (...[href, ...rest]: Parameters<typeof router.prefetch>) =>
				router.prefetch(localizeHref(href, locale), ...rest),
		}),
		[router, locale],
	);
}
