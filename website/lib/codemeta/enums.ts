// CodeMeta 生成部分：re-export 自 ./generated。
//
// ./generated.ts 由 `backend/docs/代碼管理（CodeMeta）/scripts/generate.sh --web`
// 直接從 CodeMeta Markdown 生成（CI 檢查 drift；buyer/seller app 的 code_meta.dart 同）。
// DO NOT 手動修改 ./generated.ts；要修改代碼值請改 backend 的
// 業務狀態碼.md / 業務類型碼.md / 內部技術碼.md 後重新生成。
//
// 本檔（enums.ts）是穩定的公開入口：既有 import 路徑不需變動，CodeMeta 生成內容
// re-export 於此，手寫的工具函式（非 CodeMeta 範疇）也放在本檔繼續維護。
export * from "./generated";

import {
	SHIPPING_METHOD_LABEL,
	ShippingMethod,
	type ShippingMethodValue,
	WEIGHT_UNIT_LABEL,
	WeightUnit,
	type WeightUnitValue,
} from "./generated";

const WEIGHT_LABEL_TO_CODE: Record<string, WeightUnitValue> = {
	公斤: "3301",
	台斤: "3302",
	公克: "3303",
};

/** 顯示用：把 SKU unit 值轉成人類可讀標籤。已是 WeightUnit code（"3301"…）→ 中文；
 *  其餘（舊自由標籤如「件」）原樣保留。買家端渲染 unit 時一律過這個。 */
export function formatUnitLabel(raw?: string | null): string {
	const v = (raw ?? "").trim();
	return WEIGHT_UNIT_LABEL[v as WeightUnitValue] ?? v;
}

/** 送後端用：把任意 unit 值正規化成 WeightUnit code（3301/3302/3303）。後端 SKU `unit`
 *  欄位嚴格驗證為 WeightUnit。已是 code 直接用；舊中文標籤映射回 code；其餘退回 KG。 */
export function coerceWeightUnitCode(raw?: string | null): WeightUnitValue {
	const v = (raw ?? "").trim();
	if (v in WEIGHT_UNIT_LABEL) return v as WeightUnitValue;
	return WEIGHT_LABEL_TO_CODE[v] ?? WeightUnit.KG;
}

/**
 * 賣家取消出貨原因（CancelOrderDialog 使用）。
 *
 * 手寫，非 CodeMeta 生成——backend CodeMeta 沒有獨立的 `SellerCancelReason` enum，
 * 這組值是後端 `OrderCancelReason`（見 ./generated）92xx 賣家觸發子集的複本。
 *
 * 曾有 `PROCESSING_TIMEOUT: "9204"`（UI 語意「賣家逾期未處理」）dead 常數——
 * 從未進 CancelOrderDialog 的選單、從未實際送出，且 9204 在後端實為
 * `SUGGEST_SPLIT`（建議買家拆單重下），語意衝突。#1513 裁決：「賣家逾期未處理」
 * 由後端系統自動取消 9102 `SHIPMENT_TIMEOUT` 涵蓋（09 號設計書 §5.9.11 早已
 * 廢棄賣家自選的 PROCESSING_TIMEOUT），不新增碼值，該 dead 常數直接移除。
 */
export const SellerCancelReason = {
	FORCE_MAJEURE: "9201",
	OUT_OF_STOCK: "9202",
	QUALITY_ISSUE: "9203",
	OTHER: "9299",
} as const;
export type SellerCancelReasonCode = keyof typeof SellerCancelReason;
export type SellerCancelReasonValue =
	(typeof SellerCancelReason)[SellerCancelReasonCode];

export const SELLER_CANCEL_REASON_LABEL: Record<
	SellerCancelReasonValue,
	string
> = {
	"9201": "天災因素無法配送",
	"9202": "商品缺貨",
	"9203": "商品品質問題",
	"9299": "其他原因",
};

// 賣家停權原因（46 §7.2 停權公告頁顯示用）— issue #1514 後端傳輸值已統一改送
// CodeMeta Value 數字碼（"9601" 等），與 ./generated 的數字碼鍵版本一致，移除
// 先前為了 shadow 舊 Name 傳輸值而手寫的覆蓋版本，改用 `export * from "./generated"`
// （檔案頂部）帶出的 SellerSuspensionReason / SELLER_SUSPENSION_REASON_LABEL。

/**
 * 售後證據類型（CodeMeta `AfterSaleEvidenceType`，backend `內部技術碼.md`）。
 *
 * 手寫，非 CodeMeta 生成 —— 該 enum 在 CodeMeta 標「前端可見: No」，`generate.sh --web`
 * 只輸出 Yes 的項目，所以 ./generated 裡沒有它。但 Gateway 的售後詳情回應（買家
 * `GET /app/after_sales/{caseNo}`、賣家 `GET /app/seller/after_sales/{caseNo}`）在
 * `evidence[].evidenceType` 實際把這個碼值送到前端，前端必須靠它把「買家舉證」與
 * 「賣家驗貨照片」分流，因此在此鏡射一份（同 SellerCancelReason 的手寫慣例）。
 *
 * ⚠️ 值是數字碼字串，**不是** `"BUYER"` / `"SELLER"` 這類角色字面值 —— 後者在全鏈路
 * 從未存在過，曾因 Gateway 一則過時註解誤導而寫進 `splitEvidence`，導致驗貨照片
 * 永遠 0 張（website#430 A3；後端註解已由 backend PR #2203 更正）。
 */
export const AfterSaleEvidenceType = {
	/** 8601 — 買家申請 / 補件時上傳的證據照片。 */
	PHOTO: "8601",
	/** 8602 — 賣家確認收貨時上傳的驗貨照片（支持驗貨扣款）。 */
	INSPECTION_PHOTO: "8602",
} as const;
export type AfterSaleEvidenceTypeCode = keyof typeof AfterSaleEvidenceType;
export type AfterSaleEvidenceTypeValue =
	(typeof AfterSaleEvidenceType)[AfterSaleEvidenceTypeCode];

/* ============================================================
   ShippingMethod：正規化與顯示（全站唯一入口）
   ============================================================ */

/**
 * 把後端送來的配送方式值正規化成 CodeMeta `ShippingMethod` value。
 *
 * 🔴 **未知值回 `undefined`，呼叫端必須自己 fail-safe，不得 fallback 成任何既有
 * provider**（65A §1.10）。`3003` Uber（已刪除的功能）、`3004` 7-11 大宗寄倉、`3005` 商家自送，
 * 以及後端未來新增的任何 provider，都會走到 `undefined` 這條路——舊寫法
 * `isTaqbin ? 黑貓 : 7-11` 會把它們全部靜默畫成 7-11，型別正確、不拋例外、
 * 測試全綠，只有使用者看得到。
 *
 * ⚠️ 下面那張英文／中文別名表是「後端還沒統一只回 CodeMeta value」的殘留
 * （65A §12.4 D5／D6）。它是**正規化**不是 fallback——每一條都指向一個明確的
 * 已知 provider，沒有任何一條會吃掉未列舉的值。後端統一之後整張表可刪，
 * 刪除順序是後端先、前端後。
 */
export function normalizeShippingMethod(
	raw?: string | null,
): ShippingMethodValue | undefined {
	const v = (raw ?? "").trim();
	if (!v) return undefined;
	const allowed = new Set<string>(Object.values(ShippingMethod));
	if (allowed.has(v)) return v as ShippingMethodValue;
	switch (v.toUpperCase().replace(/[\s-]/g, "_")) {
		// 黑貓宅配的歷史別名（含賣家端 fixture 用的 "home"／"宅配"）
		case "TAQBIN":
		case "TCAT":
		case "HOME_DELIVERY":
		case "HOME":
		case "宅配":
			return ShippingMethod.TAQBIN;
		case "SEVEN_ELEVEN_STORE_PICKUP":
		case "SEVEN_ELEVEN":
		case "CVS_7_11":
		case "STORE_PICKUP":
			return ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP;
		default:
			return undefined;
	}
}

/**
 * 配送方式的顯示文字。空值 → `"—"`；認得的碼 → CodeMeta 中文標籤；
 * **不認得的碼 → 原碼照顯**（65A §1.3 第 4 條：不猜測、不隱藏）。
 *
 * 🔴 一律不得回傳任何 provider 名稱當作預設值——賣家訂單列表曾在 `raw` 為空時
 * 回「宅配」，等於憑空捏造一個物流商。
 */
export function formatShippingMethodLabel(raw?: string | null): string {
	const v = (raw ?? "").trim();
	if (!v) return "—";
	const code = normalizeShippingMethod(v);
	if (code) return SHIPPING_METHOD_LABEL[code] ?? v;
	return SHIPPING_METHOD_LABEL[v] ?? v;
}
