// CodeMeta 的穩定公開入口。既有 import 路徑 `@/lib/codemeta` 不變。
//
// 碼值本身**不在本檔維護**——它們在 ./generated.ts，由
// `backend/docs/代碼管理（CodeMeta）/scripts/generate.sh --admin` 從 CodeMeta Markdown 直接生成
// （CI 檢查 drift）。要改代碼值或中文標籤，改 backend 的 `.md` 再重新生成，
// 不要改 ./generated.ts（會被下次生成覆蓋，且與 SSOT 分歧）。
//
// 本檔只放**不是碼值**的東西：查表 helper 與顯示順序。這條界線是 CodeMeta 規範
// 明訂的（生成 enum 只承載代碼資料，業務 helper 另放），也是讓 ./generated.ts
// 可以被整檔重新生成而不損失任何手寫邏輯的前提。
//
// 歷史：本檔前身是 208 行手抄的 backend 鏡像。集中到單一檔案解決了「各頁自定義
// enum」那次漂移（transactions 頁 status 假想 0-6 / payType 假想 1-4 全數錯位），
// 但集中之後它仍然是手抄的，所以又漂了 5 處——包括把 CodeMeta 的 Description 欄
// 當成 Label 欄抄（AppType 抄成「買家版／賣家版」，正確是「買家／賣家」）。
// 那種錯誤任何「統一措辭」都治不了，只有吃生成物才治得到（backend #2536）。
export * from "./generated";

export type CodeLabelMap = Record<string, string>;

/** value→label 查表；未命中回傳原值，null/空回傳 "—"（避免顯示空白或誤導成假標籤）。 */
export function codeLabel(
	map: CodeLabelMap,
	value: string | number | null | undefined,
): string {
	if (value === null || value === undefined || value === "") return "—";
	const key = String(value);
	return map[key] ?? key;
}

/**
 * 供 <select>/tab 建構有序選項用（[value, label][]）。
 *
 * `order` 省略時沿用 map 的宣告順序，也就是 CodeMeta Markdown 的列序。⚠️ 那個順序是
 * **SSOT 的敘述順序，不是 UI 的呈現順序**，兩者沒有必然關係——例如 ShipmentAnomalySeverity
 * 在 SSOT 是 LOW→MEDIUM→HIGH（由輕到重的定義順序），但異常列表的篩選下拉要「高」在最前面
 * （管理員最先想看的是最嚴重的）。這種時候明確傳 `order`，不要為了排版去動 SSOT 的列序。
 *
 * `order` 內找不到的 key 會被忽略；map 內有而 `order` 沒列到的 key 接在後面，
 * 所以未來 SSOT 新增值域不會從下拉裡靜默消失。
 */
export function toOptions(
	map: CodeLabelMap,
	order?: readonly string[],
): Array<[string, string]> {
	if (!order) return Object.entries(map);
	const listed = order.filter((k) => k in map);
	const rest = Object.keys(map).filter((k) => !listed.includes(k));
	return [...listed, ...rest].map((k) => [k, map[k]] as [string, string]);
}

/** 配送異常嚴重度的呈現順序：重的在前。值域本身見 ./generated 的 ShipmentAnomalySeverity。 */
export const SHIPMENT_ANOMALY_SEVERITY_ORDER = [
	"HIGH",
	"MEDIUM",
	"LOW",
] as const;
