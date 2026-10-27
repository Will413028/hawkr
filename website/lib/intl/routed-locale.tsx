"use client";

import * as React from "react";
import { type AppLocale, defaultLocale } from "./locales";

const RoutedLocaleContext = React.createContext<AppLocale>(defaultLocale);

/**
 * 這棵樹正在渲染的路由語系，給導覽層用（`components/shared/Link.tsx`、`lib/intl/use-router.ts`）。
 * `AppProviders` 依 `[locale]` 的參數提供；server 端 render 與瀏覽器端都讀得到，不經模組變數
 * （server 端的模組變數會在請求之間共用，見 `lib/intl/active-locale.ts`）。
 *
 * 沒有 provider 的獨立渲染（單元測試、預覽、之後的 server 擁有的外殼）退回預設語系：
 * 共用的連結本來就要能在沒有 AppProviders 時退化成普通連結（`components/shared/Link.tsx`）。
 */
export const RoutedLocaleProvider = RoutedLocaleContext.Provider;

export function useRoutedLocale(): AppLocale {
	return React.useContext(RoutedLocaleContext);
}
