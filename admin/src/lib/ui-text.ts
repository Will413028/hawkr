import type { UiMessage } from "@hawkr/ui-text/format";
import { commonUnknown } from "@hawkr/ui-text/keys";
import { formatStaff } from "@hawkr/ui-text/zh-TW";

/** 不認得的值與 UNSPECIFIED 的顯示文字（共用文案目錄的 common.unknown：「未知」）。 */
export function unknownText(): string {
	return formatStaff(commonUnknown());
}

/**
 * 目錄的名詞或句子的顯示文字（後台員工受眾）。目錄對 UNSPECIFIED 與不認得的值回 undefined；
 * 這個畫面選擇顯示「未知」，所以在這裡明確寫出來。
 */
export function staffTextOrUnknown(message: UiMessage | undefined): string {
	return message === undefined ? unknownText() : formatStaff(message);
}
