/**
 * 評價者沒有名字（匿名）時顯示的文字，所有評價畫面共用這一處。
 * 目錄的匿名評價者 key 隨 A6-5f 加入，到時改成 `@hawkr/ui-text` 的文案；資料層一律傳 undefined，不寫死文字。
 */
const ANONYMOUS_REVIEWER = "匿名";

export function reviewerName(name: string | null | undefined): string {
	return name?.trim() || ANONYMOUS_REVIEWER;
}
