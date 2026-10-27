/**
 * S02 送審閘門（58 §7.3.3 T02 ＋ §6.1.7 端點 19；裁決 C，2026-08-31）。
 *
 * 🔴 **這一層不是防線，是文案。** 真正的守衛全部在後端（T02 的兩個守衛寫在
 * usecase、狀態與版本寫在 SQL 的 WHERE）。前端提早判斷只為了回答賣家一個問題：
 * 「我現在還缺什麼」——把它做成 `disabled` 而不說原因，賣家會反覆點一顆永遠
 * 不會動的按鈕。
 *
 * 🔴 **每一條判準逐字對應後端的守衛**，不得自己加碼或放寬：
 *
 *   editable          ← `invoiceTaxStatusBanner().editable`（狀態機的性質）
 *   identity 齊全     ← usecase `validateTaxIdentity()`：entityType／legalName／
 *                       responsibleName／address 皆非空；claimedStatus 為
 *                       `REQUIRED` 時另需 taxId；`UNKNOWN` 一律不齊
 *   至少一份佐證      ← T02 守衛②，讀的是**已落庫**的 `evidence_object_keys`
 *                       （投影成 `documents[]`）
 *   已同意條款        ← 裁決 C：勾選框，隨送審一起送（不是另一支端點的前置）
 *
 * ⚠️ 放寬任何一條，賣家會拿到後端的 400／9325 而不是就地的說明；收緊任何一條，
 * 會出現「後端可以送、前端不讓送」的鎖死態。兩種都是回歸。
 */

import { SellerTaxObligation } from "@/lib/codemeta/enums";
import type { SellerTaxProfile } from "@/lib/invoices/seller-types";

/** 送不出去的原因。`null` ＝ 可以送。 */
export type TaxSubmitBlocker =
	/** 這個狀態不允許送審（審核中／已核定／已撤銷）⇒ 整個區塊不渲染。 */
	| "not-editable"
	/** 還沒有任何稅務身分資料 ⇒ 要先建草稿（端點 18）。 */
	| "not-started"
	/** 必填欄位不齊（對應後端 400 `ErrTaxIdentityIncomplete`）。 */
	| "identity-incomplete"
	/** 一份稅籍證明都沒有（對應後端 400「請至少上傳一份稅籍證明文件」）。 */
	| "evidence-missing"
	/** 重新申請冷卻中（對應後端 9324）。 */
	| "cooldown";

export interface TaxSubmitGate {
	/** 送審區塊要不要出現。`false` ＝ 這個狀態根本沒有送審這件事。 */
	visible: boolean;
	/** `null` ＝ 條款勾選之後就能送。 */
	blocker: TaxSubmitBlocker | null;
}

interface GateInput {
	profile: SellerTaxProfile | null;
	/** 端點 17 回 `9320`／`NOT_STARTED`。 */
	notStarted: boolean;
	/** 狀態機說這個狀態可不可以編輯（`invoiceTaxStatusBanner().editable`）。 */
	editable: boolean;
	/** 冷卻是否仍在進行中。呼叫端用同一個 clock 算，避免本檔碰 `Date.now()`。 */
	cooldownActive: boolean;
}

export function taxSubmitGate(input: GateInput): TaxSubmitGate {
	if (!input.editable) return { visible: false, blocker: "not-editable" };
	// 尚未建立任何資料：可編輯，但沒有東西可送 ⇒ 區塊要出現並說明下一步，
	// 不是安靜地消失（安靜消失的話新賣家看不到任何入口）。
	if (input.notStarted || input.profile === null) {
		return { visible: true, blocker: "not-started" };
	}
	// 冷卻優先於欄位檢查：冷卻期間就算資料齊全也送不出去，先講那件事比較有用。
	if (input.cooldownActive) return { visible: true, blocker: "cooldown" };
	if (!isIdentityComplete(input.profile)) {
		return { visible: true, blocker: "identity-incomplete" };
	}
	if (input.profile.documents.length === 0) {
		return { visible: true, blocker: "evidence-missing" };
	}
	return { visible: true, blocker: null };
}

/**
 * 逐字對應 usecase `validateTaxIdentity()`。
 *
 * ⚠️ `addressCity`／`addressDistrict`／`addressDetail` 三欄在後端折成單欄
 * `business_address`，回程只填得出 `addressDetail` ⇒ 判「地址有沒有填」必須看
 * 三欄的**任一**非空，只看 `addressDetail` 會在剛存完草稿時誤判成沒填。
 */
export function isIdentityComplete(profile: SellerTaxProfile): boolean {
	const id = profile.taxIdentity;
	const hasAddress =
		id.addressCity !== "" ||
		id.addressDistrict !== "" ||
		id.addressDetail !== "";
	if (
		id.entityType === "" ||
		id.legalName === "" ||
		id.responsibleName === "" ||
		!hasAddress
	) {
		return false;
	}
	// 自述「應開統一發票」必須有統編；自述值未填（UNKNOWN／空）一律視為不齊。
	if (id.claimedStatus === SellerTaxObligation.REQUIRED) return id.taxId !== "";
	return id.claimedStatus === SellerTaxObligation.EXEMPT;
}
