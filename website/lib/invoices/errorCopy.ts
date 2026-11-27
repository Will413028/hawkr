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
	[SELLER_INVOICE_BIZ_CODE.TAX_PROFILE_NOT_FOUND]: {
		title: "尚未建立稅務身分資料。",
		retryable: false,
	},
	[SELLER_INVOICE_BIZ_CODE.PROVIDER_ACCOUNT_NOT_FOUND]: {
		title: "尚未建立 AMEGO 串接資料。",
		retryable: false,
	},
	[SELLER_INVOICE_BIZ_CODE.PROVIDER_UNAVAILABLE]: {
		title: "AMEGO 服務暫時無法連線，系統稍後會自動重試。",
		description: "請稍後再試，不需要更改 AppKey。",
		retryable: true,
	},
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
	[SELLER_INVOICE_BIZ_CODE.NOTIFICATION_RESEND_THROTTLED]: {
		title: "剛剛才寄過，暫時不會重複寄送。",
		description: "請稍後再試。買家可能只是還沒收到，先請他確認垃圾郵件匣。",
		retryable: true,
	},
	[SELLER_INVOICE_BIZ_CODE.JOB_NOT_FOUND]: {
		title: "找不到這筆發票工作。",
		retryable: false,
	},
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
	[SELLER_INVOICE_BIZ_CODE.OPERATION_IN_PROGRESS]: {
		title: "這張發票有處理中的作業。",
		description: "請稍候幾秒再試，不要重複送出。",
		retryable: true,
	},
	[SELLER_INVOICE_BIZ_CODE.TAX_STATE_NOT_ALLOWED]: {
		title: "目前的狀態無法送出審核。",
		description: "資料可能已被受理或已更新，請重新整理後再確認。",
		retryable: true,
	},
	[SELLER_INVOICE_BIZ_CODE.TAX_VERSION_CONFLICT]: {
		title: "資料已被更新，請重新整理後再送出。",
		retryable: true,
	},
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
