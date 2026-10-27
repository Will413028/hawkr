/**
 * `carrierSummary` 那一列該標成什麼（backend#3084）。
 *
 * - `carrier` — summary 裝的是載具（已依 viewer 遮罩）
 * - `recipientTaxId` — summary 裝的是買受人統編
 * - `none` — 沒有東西可顯示
 */
export type CarrierRowKind = "carrier" | "recipientTaxId" | "none";

/**
 * 依後端的 `carrierType` 判斷 `carrierSummary` 裝的是哪一種。
 *
 * 🔴 **不要用「值看起來像不像統編」去猜**：8 碼數字既可能是統編、也可能是遮罩後的
 * 載具尾碼，猜錯就是在法定憑證上標錯欄位。
 *
 * 後端在沒有載具時**刻意**把買受人統編放進 `carrierSummary`
 * （B2B 列表要看得到開給哪個統編，那是 #3084 裁決 B 保留的行為），
 * 所以只看 `carrierSummary` 分不出兩者 —— 判別欄是 `carrierType`。
 *
 * | carrierType | carrierSummary | 回傳 |
 * |---|---|---|
 * | 非空 | 有值 | `carrier` |
 * | 空 | 有值 | `recipientTaxId` |
 * | 空 | 空 | `none` |
 *
 * dev 實測的原缺陷：document 7501121593846792192 的 `carrier_type` 與
 * `carrier_id1_ciphertext` 皆為 NULL（真的沒有載具），而詳情頁同時出現
 * 「買受人統編 22223223」與「載具 22223223」兩列、值一模一樣。
 *
 * ⚠️ 回 `carrier` 但 summary 為空是**矛盾輸入**（後端保證兩者同進同出，
 * 見 `carrierPresentationOf`）。這裡仍歸到 `none`，寧可少顯示也不要標錯。
 */
export function carrierRowKindOf(
	carrierSummary: string,
	carrierType: string,
): CarrierRowKind {
	if (carrierType && carrierSummary) return "carrier";
	if (carrierSummary) return "recipientTaxId";
	return "none";
}
