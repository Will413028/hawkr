import { render } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactElement } from "react";
import { defaultLocale, messagesFor } from "@/lib/intl/config";

/** 和 Storybook 的 IntlDecorator 相同的語系 provider（lib/intl/navigation.ts 的 hook 需要它）。 */
export function renderWithIntl(
	ui: ReactElement,
	options: { theme?: "light" | "dark" } = {},
) {
	const root = document.documentElement;
	root.lang = "zh-Hant";
	root.classList.add("hk-font-rounded");
	root.dataset.theme = options.theme ?? "light";
	return render(
		<NextIntlClientProvider
			locale={defaultLocale}
			messages={messagesFor(defaultLocale)}
			timeZone="Asia/Taipei"
		>
			{ui}
		</NextIntlClientProvider>,
	);
}
