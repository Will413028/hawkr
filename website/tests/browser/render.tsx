import { render } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactElement } from "react";
import { defaultLocale, messagesFor } from "@/lib/intl/config";

/**
 * 掛語系 provider（lib/intl/navigation.ts 的 hook 需要它），並設定 html 的主題與字級倍率。
 * `frameMaxWidth` 以 100% 寬、上限該值的外框包住被測元件。
 * 字級倍率在 `tests/browser/setup.ts` 的 afterEach 還原。
 */
export function renderWithIntl(
	ui: ReactElement,
	options: {
		theme?: "light" | "dark";
		textScale?: number;
		frameMaxWidth?: number;
	} = {},
) {
	const root = document.documentElement;
	root.lang = "zh-Hant";
	root.classList.add("hk-font-rounded");
	root.dataset.theme = options.theme ?? "light";
	if (options.textScale !== undefined) {
		root.style.fontSize = `${Number((options.textScale * 100).toFixed(4))}%`;
		root.style.setProperty("--hk-text-scale", String(options.textScale));
	}
	return render(
		<NextIntlClientProvider
			locale={defaultLocale}
			messages={messagesFor(defaultLocale)}
			timeZone="Asia/Taipei"
		>
			{options.frameMaxWidth === undefined ? (
				ui
			) : (
				<div style={{ width: "100%", maxWidth: options.frameMaxWidth }}>
					{ui}
				</div>
			)}
		</NextIntlClientProvider>,
	);
}
