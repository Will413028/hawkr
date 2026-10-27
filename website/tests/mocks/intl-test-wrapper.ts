import { NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { defaultLocale, messagesFor } from "@/lib/intl/config";

/** 單元測試用：讓 `useDefaultErrorText()` 等 next-intl 的 hook 拿得到與 app 相同的語系與訊息。 */
export function IntlTestWrapper({ children }: { children: React.ReactNode }) {
	return React.createElement(NextIntlClientProvider, {
		locale: defaultLocale,
		messages: messagesFor(defaultLocale),
		children,
	});
}
