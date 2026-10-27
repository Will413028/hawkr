/**
 * 發票業務碼 → 使用者文案（§B-2.3 (1)）。
 *
 * 🔴 一律讀 envelope 的 `code`，**不讀 HTTP status**（§0.4 ①／§B-2.0 T09）：
 * 碼表裡 7 條標 `preserveData` 的 HTTP 恆為 200，照 status 分流會把「憑證被拒」
 * 當成成功。`ApiClientError` 已帶 `code`，前綴形式的 `[NNNN]` 由
 * `invoiceBizCode()` 一併處理。
 *
 * 回 `null` ＝「這不是發票業務碼」⇒ 呼叫端交回通用錯誤處理，不要在這裡兜底成
 * 一句萬用的「操作失敗」（那會把 5xx 與網路中斷也吃掉）。
 *
 * ⚠️ 本表只收**有路徑到得了**的碼。「寫進來備用」的文案永遠不會被讀到，卻會讓
 * 後來的人以為那條路已經接好了。
 *
 * 2026-08-31：S02 的**送審**（端點 19）接上 ⇒ 補 `9321`／`9322`／`9324`／`9325` 四條。
 * 2026-09-02：端點 62（回報自行開立）接上 ⇒ 補 `9343`／`9349`／`9350`／`9351` 四條。
 * 其餘 mutation 專屬碼（`9323`）仍無路徑，隨各自的 CTA 一起補。
 */

import { invoiceBizCode } from "./buyer-repo";
import { SELLER_INVOICE_BIZ_CODE } from "./seller-repo";

export interface InvoiceErrorCopy {
	title: string;
	description?: string;
	/** 這個錯誤重試有沒有意義。`false` ＝ 就地小字，不給重試鍵。 */
	retryable: boolean;
}

const COPY: Record<number, InvoiceErrorCopy> = {
	[SELLER_INVOICE_BIZ_CODE.SUSPENDED]: {
		title: "帳號停權期間無法使用此功能。",
		retryable: false,
	},
	// Tier A：越權與不存在回同一個碼 ⇒ 文案必須與「真的不存在」完全相同，
	// 不得寫「沒有權限」——那等於把兩者區分出來給探測者看。
	[SELLER_INVOICE_BIZ_CODE.DOCUMENT_NOT_FOUND]: {
		title: "找不到這份發票文件。",
		retryable: false,
	},
	[SELLER_INVOICE_BIZ_CODE.CARRIER_NOT_WON]: {
		title: "載具發票要中獎後才能下載。",
		retryable: false,
	},
	[SELLER_INVOICE_BIZ_CODE.QUERY_WINDOW_EXPIRED]: {
		title: "超過查詢期限，無法下載這份 PDF。",
		retryable: false,
	},
	// 不是錯誤態：S02 收到它要渲染「尚未開始」，不是紅字。文案留著供除錯用。
	[SELLER_INVOICE_BIZ_CODE.TAX_PROFILE_NOT_FOUND]: {
		title: "尚未建立稅務身分資料。",
		retryable: false,
	},
	// 同上：S03 收到它要渲染空白精靈。本波沒有 S03，留在表上是因為端點 16 的
	// providerStatus 投影也走同一個語意。
	[SELLER_INVOICE_BIZ_CODE.PROVIDER_ACCOUNT_NOT_FOUND]: {
		title: "尚未建立 AMEGO 串接資料。",
		retryable: false,
	},
	// 🔴 與 9331（憑證被拒）必須分開：混成一句會讓賣家在 AMEGO 暫時不可用時
	// 把正確的 AppKey 刪掉重打。
	[SELLER_INVOICE_BIZ_CODE.PROVIDER_UNAVAILABLE]: {
		title: "AMEGO 服務暫時無法連線，系統稍後會自動重試。",
		description: "請稍後再試，不需要更改 AppKey。",
		retryable: true,
	},
	// 🔴 S03 的四碼各自有不同的「下一步」，混成一句會把賣家送去做錯的事：
	//   9331 憑證被拒     → 回光貿後台確認 AppKey（**不是**改統編）
	//   9333 尚未通過測試 → 先按「測試連線」（**不是**重打 AppKey）
	//   9334 統編不符     → 回 S02 更正稅籍（**不是**改 AppKey）
	//   9337 憑證被綁走   → 回光貿後台換一組（重打同一把永遠不會成功）
	[SELLER_INVOICE_BIZ_CODE.PROVIDER_VERIFICATION_FAILED]: {
		title: "AMEGO 無法驗證這組 AppKey。",
		description:
			"請回到光貿後台確認 AppKey 是否正確、是否仍在有效期內，再重新測試一次。",
		retryable: true,
	},
	[SELLER_INVOICE_BIZ_CODE.PROVIDER_NOT_VERIFIED]: {
		title: "請先完成連線測試，才能設定啟用時間。",
		retryable: false,
	},
	[SELLER_INVOICE_BIZ_CODE.PROVIDER_TAXID_MISMATCH]: {
		title: "送出的統一編號與平台核定的稅籍資料不符。",
		description:
			"發票一旦開錯抬頭只能作廢重開，所以這一步會先擋下來。請回到「稅務身分」確認核定的統編。",
		retryable: false,
	},
	[SELLER_INVOICE_BIZ_CODE.PROVIDER_CREDENTIAL_TAKEN]: {
		title: "這組 AppKey 已被其他賣場使用。",
		description: "請回到光貿後台為這個賣場另外申請一組，再重新設定。",
		retryable: false,
	},
	// 端點 32 的節流。🔴 **它是 error 不是 `resent:false`**（2026-09-01 dev 實測）
	// ——只 catch 成通用失敗，賣家看到「寄送失敗」而真正的原因是「剛剛才寄過」。
	[SELLER_INVOICE_BIZ_CODE.NOTIFICATION_RESEND_THROTTLED]: {
		title: "剛剛才寄過，暫時不會重複寄送。",
		description: "請稍後再試。買家可能只是還沒收到，先請他確認垃圾郵件匣。",
		retryable: true,
	},
	[SELLER_INVOICE_BIZ_CODE.JOB_NOT_FOUND]: {
		title: "找不到這筆發票工作。",
		retryable: false,
	},
	// 🔴 端點 62 的四碼。四句的「下一步」互不相同：
	//   9343 版本衝突   → 重新整理再送（輸入不清空，賣家沒做錯任何事）
	//   9349 尚未開放   → 等平台通知，重送幾次都一樣
	//   9350 冪等鍵撞號 → 換一把新的重送（呼叫端每次按下本來就換新的）
	//   9351 作業處理中 → 稍等，**不是**填錯
	[SELLER_INVOICE_BIZ_CODE.JOB_VERSION_CONFLICT]: {
		title: "這筆發票工作已被更新。",
		description: "請重新整理後再回報一次，你填的號碼不會有問題。",
		retryable: true,
	},
	[SELLER_INVOICE_BIZ_CODE.EXTERNAL_ISSUANCE_NOT_ALLOWED]: {
		title: "這筆還沒有開放自行開立回報。",
		description: "平台仍在處理這張發票，開放後會通知你。",
		retryable: false,
	},
	[SELLER_INVOICE_BIZ_CODE.IDEMPOTENCY_PAYLOAD_MISMATCH]: {
		title: "這次送出的內容與上一次不一致。",
		description: "請重新整理後再送一次。",
		retryable: true,
	},
	// 不是錯誤態：呼叫端把它從 error 分出來顯示成進度，這一條留著供除錯用。
	[SELLER_INVOICE_BIZ_CODE.OPERATION_IN_PROGRESS]: {
		title: "這張發票有處理中的作業。",
		description: "請稍候幾秒再試，不要重複送出。",
		retryable: true,
	},
	// 🔴 送審四碼（端點 19）。四句的「下一步」互不相同，合併成一句萬用錯誤會讓
	// 賣家在該補件時去等審核、在該等冷卻時反覆重送。
	[SELLER_INVOICE_BIZ_CODE.TAX_STATE_NOT_ALLOWED]: {
		title: "目前的狀態無法送出審核。",
		description: "資料可能已被受理或已更新，請重新整理後再確認。",
		retryable: true,
	},
	// 不清空輸入：賣家沒做錯任何事，重整拿到新版本再送一次即可。
	[SELLER_INVOICE_BIZ_CODE.TAX_VERSION_CONFLICT]: {
		title: "資料已被更新，請重新整理後再送出。",
		retryable: true,
	},
	// preserveData（HTTP 200）：倒數時刻在 data.reapplyCooldownUntil，
	// 呼叫端要自己讀出來接在 description 後面。
	[SELLER_INVOICE_BIZ_CODE.TAX_REAPPLY_COOLDOWN]: {
		title: "還在重新申請的冷卻期間。",
		description: "冷卻結束後才能再次送出審核。",
		retryable: false,
	},
	[SELLER_INVOICE_BIZ_CODE.TAX_SELF_ISSUE_TERMS_NOT_ACCEPTED]: {
		title: "請先閱讀並同意自行開票責任條款，才能送出審核。",
		retryable: false,
	},
};

/** 把任意 error 折成發票文案；不是發票業務碼時回 `null`。 */
export function invoiceErrorCopy(error: unknown): InvoiceErrorCopy | null {
	const code = invoiceBizCode(error);
	if (!code) return null;
	return COPY[code] ?? null;
}

/** 這個 error 是不是某一個特定的發票業務碼。 */
export function isInvoiceBizCode(error: unknown, code: number): boolean {
	return invoiceBizCode(error) === code;
}
