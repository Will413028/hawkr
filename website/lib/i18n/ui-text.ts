import type { Code } from "@connectrpc/connect";
import type { DefaultErrorText } from "@hawkr/api/connect/errors";
import {
	createUiText,
	type Translate,
	type UiMessage,
} from "@hawkr/ui-text/format";
import { commonError, commonUnknown } from "@hawkr/ui-text/keys";
import { createTranslator, useTranslations } from "next-intl";
import { useMemo } from "react";
import { type AppLocale, messagesFor } from "@/lib/intl/config";

export type UiText = (m: UiMessage) => string;

/**
 * 純函式版：明確帶 locale，不讀 header。RSC 與測試用這個，不用 `getTranslations()`（沒有 `[locale]` 段時
 * `requestLocale` 會讀 header，把 ISR 的頁變成動態）。
 */
export function uiTextFor(locale: AppLocale): UiText {
	return createUiText(
		createTranslator({
			locale,
			messages: messagesFor(locale),
			namespace: "uitext",
		}) as unknown as Translate,
	);
}

/** client component 用：語系取自 NextIntlClientProvider。 */
export function useUiText(): UiText {
	const t = useTranslations("uitext") as unknown as Translate;
	return useMemo(() => createUiText(t), [t]);
}

/**
 * 目錄的名詞、變體與句子在「沒有文字」時回 undefined（UNSPECIFIED、不認得的值、該受眾 omit 的值）；有文字就格式化。
 * 要顯示「未知」或用自己的後備，由呼叫端明確寫出（`commonUnknown()`）。
 */
export function optionalText(
	ut: UiText,
	m: UiMessage | undefined,
): string | undefined {
	return m === undefined ? undefined : ut(m);
}

/** Connect code 的預設文字（共用文案目錄）；Canceled 由呼叫端自己處理。 */
export function defaultErrorTextFor(locale: AppLocale): DefaultErrorText {
	const ut = uiTextFor(locale);
	return (code) => ut(commonError(code) ?? commonUnknown());
}

/** client component 用：語系取自 NextIntlClientProvider。 */
export function useDefaultErrorText(): DefaultErrorText {
	const t = useTranslations("uitext") as unknown as Translate;
	return useMemo(() => {
		const ut = createUiText(t);
		return (code: Exclude<Code, Code.Canceled>) =>
			ut(commonError(code) ?? commonUnknown());
	}, [t]);
}
