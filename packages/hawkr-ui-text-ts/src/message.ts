import type { UiTextKey } from "./keys.gen";

/**
 * 一則待顯示的文案：key 與參數。建構函式（keys.gen.ts）只回這個，不回字串；格式化由 client 的 i18n 執行期做
 * （website 是 next-intl，admin 是 zh-tw.gen.ts 的 formatStaff）。
 */
export interface UiMessage<K extends UiTextKey = UiTextKey> {
	key: K;
	/** 名詞參數的值是另一則 UiMessage，格式化時以同一語系、同一受眾先解出。 */
	values?: Record<string, string | number | UiMessage>;
}

/** Connect code 的預設文字；型別的唯一定義在 `@hawkr/api/connect/errors`（本套件依賴它，反向不行）。 */
export type { DefaultErrorText } from "@hawkr/api/connect/errors";
