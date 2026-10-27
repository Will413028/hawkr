import type { UiMessage } from "./message";

export type { DefaultErrorText, UiMessage } from "./message";

/** 翻譯函式：以 key 與（名詞參數已解成字串的）參數回文字，例如 use-intl 的 `t`。 */
export type Translate = (
	key: string,
	values?: Record<string, string | number>,
) => string;

/**
 * 把 UiMessage 格式化成字串：值是 UiMessage 的參數（名詞參數）先以同一個 translate 解成字串，再格式化外層訊息。
 * 這不是拼接：外層整句的模式是一則訊息，語序由它決定。
 */
export function formatUiMessage(translate: Translate, m: UiMessage): string {
	const values: Record<string, string | number> = {};
	for (const [name, value] of Object.entries(m.values ?? {})) {
		values[name] =
			typeof value === "object" ? formatUiMessage(translate, value) : value;
	}
	return translate(m.key, values);
}

/** 由翻譯函式建立 `(m: UiMessage) => string`。 */
export function createUiText(translate: Translate): (m: UiMessage) => string {
	return (m) => formatUiMessage(translate, m);
}
