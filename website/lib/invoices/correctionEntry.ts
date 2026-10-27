/**
 * B06「修改發票資料」入口的顯示判準與送出參數。
 *
 * 🔴 **這裡存在的理由是可測性**：原本這個判斷寫在 `OrderInvoiceSection.tsx` 的 JSX
 * 條件裡，只能靠源碼字串比對來守，而變異驗證證明那種測試是**弱的**——把
 * `invoice.canRequestCorrection` 換成 `invoice.documents.length > 0`，字串仍然出現在
 * 別處，測試照樣全綠。抽成純函式之後，同一個變異會真的轉紅。
 *
 * 🔴 **判準只取後端的 `canRequestCorrection`**，不自己算：它涵蓋開票狀態、法定
 * 作廢窗口、是否已有進行中的申請三件事。前端複製一份必然漂移，而漂移的症狀是
 * 買家按到一顆必定失敗的鍵。
 */

import type { OrderInvoice } from "./types";

export interface CorrectionEntry {
	/** 要不要渲染那顆按鈕。 */
	visible: boolean;
	/**
	 * 送出時的 `expectedSnapshotVersion`。
	 *
	 * 🔴 取**內容軸**的 `snapshotVersion`，不是實體軸的 `issue_attempt_seq`：
	 * 作廢重開會讓後者 +1 而前者不變 ⇒ 拿錯的話，買家在賣家剛作廢重開之後送出的
	 * 修改會被誤判成版本衝突。
	 */
	expectedSnapshotVersion: string;
}

export function correctionEntry(
	invoice: Pick<
		OrderInvoice,
		"canRequestCorrection" | "snapshotVersion"
	> | null,
): CorrectionEntry {
	if (!invoice) return { visible: false, expectedSnapshotVersion: "" };
	return {
		visible: invoice.canRequestCorrection === true,
		expectedSnapshotVersion: invoice.snapshotVersion,
	};
}
