import { hasLocale } from "next-intl";
import { getRequestConfig } from "next-intl/server";
import { defaultLocale, messagesFor } from "@/lib/intl/config";
import { routing } from "./routing";

// 目前沒有 server component 呼叫 next-intl 的 server API（`getTranslations`、`getLocale`）：RSC 一律用
// `uiTextFor(locale)`（lib/i18n/ui-text.ts），因為 `requestLocale` 在沒有 `setRequestLocale` 時會讀 header，
// 把 ISR 的頁變成動態。這份設定給 plugin 與日後的 server API 用，語系取自 middleware 傳下來的值。
export default getRequestConfig(async ({ requestLocale }) => {
	const requested = await requestLocale;
	const locale = hasLocale(routing.locales, requested)
		? requested
		: defaultLocale;
	return { locale, messages: messagesFor(locale) };
});
