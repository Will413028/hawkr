import { type AppLocale, defaultLocale } from "./locales";

/**
 * 瀏覽器端「目前這個頁面的語系」，給 React 之外的程式用（Connect 的 `Accept-Language`、硬導覽、`loginPath` 的
 * 呼叫端）。來源是路由的語系：`AppProviders` 每次 render 依 `[locale]` 的參數設一次。
 *
 * 不讀 DOM：`<html lang>` 會被瀏覽器的翻譯功能改寫（Chrome 翻成中文後變 `zh-TW`），讀它送出的語系會把
 * 「看英文版、用瀏覽器翻成中文」的人記成 zh-TW（ADR 2026-10-17 D1，API 收到的語系會更新帳號的 `preferred_locale`）。
 *
 * 只在瀏覽器存在。server 端同一個 JS 程序同時服務很多請求，模組變數會在請求之間共用，所以 server 端不設、
 * 不讀：server 的程式要拿語系，從它渲染的路由參數明確傳下去。
 */
let current: AppLocale = defaultLocale;

const isBrowser = () => typeof window !== "undefined";

/** 設定瀏覽器端目前頁面的語系；在 server 端是空操作。 */
export function setActiveLocale(locale: AppLocale): void {
	if (isBrowser()) current = locale;
}

/** 瀏覽器端目前頁面的語系；server 端沒有「目前頁面」，呼叫它是程式錯誤。 */
export function activeLocale(): AppLocale {
	if (!isBrowser()) {
		throw new Error(
			"activeLocale() is browser-only: server code must pass the locale of the route it renders",
		);
	}
	return current;
}
