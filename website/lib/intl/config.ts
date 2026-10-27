import uiTextEn from "@hawkr/ui-text/messages/en";
import uiTextZhTW from "@hawkr/ui-text/messages/zh-TW";
import websiteMessagesEn from "@/messages/en.json";
import websiteMessagesZhTW from "@/messages/zh-TW.json";
import { type AppLocale, defaultLocale } from "./locales";

export { type AppLocale, defaultLocale };

/** 每個語系的訊息。共用文案目錄掛在 `uitext` namespace，不和 website 自己的 key 撞。 */
const messages = {
	"zh-TW": { ...websiteMessagesZhTW, uitext: uiTextZhTW },
	en: { ...websiteMessagesEn, uitext: uiTextEn },
} as const satisfies Record<AppLocale, Record<string, unknown>>;

export function messagesFor(locale: AppLocale) {
	return messages[locale];
}
