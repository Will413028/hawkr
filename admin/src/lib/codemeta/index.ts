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
