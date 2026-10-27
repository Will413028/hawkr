import type { BadgeVariant } from "@/components/UI/StatusBadge";
import {
	InvoiceComplaintStatus,
	InvoiceDocumentStatus,
	InvoiceFulfillmentMode,
	InvoiceOperationStatus,
	InvoiceProviderAccountStatus,
	SellerTaxReviewStatus,
} from "@/lib/codemeta";

/**
 * 發票模組的「值 → badge 顏色」對照。
 *
 * **這裡只放顏色，一個中文字都沒有**：標籤一律 `codeLabel(XXX_LABEL, value)` 查
 * CodeMeta 生成物。顏色是後端不下發的呈現層決定，所以它是前端的責任；中文不是。
 *
 * ⚠️ **鍵一律寫成 `[XxxEnum.MEMBER]` 的計算鍵**，不寫字面值。理由是機械的：
 * CodeMeta 的值域改名或刪值時，計算鍵會**編譯錯**，字面值只會靜靜落進 `?? "gray"`
 * ——一個沒有顏色分級的狀態看起來就跟「一切正常」一樣。
 * （`tests/codemeta-no-second-copy.test.ts` 的字串鍵偵測器也要求這個形式。）
 *
 * ⚠️ 58A §A02 把稅務審核那張表寫在 `tax-reviews/page.tsx` 內並 export 給 A03。
 * 這裡改放共用模組：兩張表各有兩個以上的消費者（A01＋A05／A02＋A03），而從
 * `page.tsx` import 常數會把整個頁面模組（含 "use client" 邊界）拖進 importer。
 * 值與語意未變。
 */

/**
 * `InvoiceOperationStatus`（20 值）。A01 優先工作清單與 A05 佇列共用。
 *
 * 三個失敗狀態全部 danger 而語意不同（永久失敗／重試用盡／**結果未確認**）；
 * 最後一個特別危險——它代表我們不知道 AMEGO 那邊到底開了沒，貿然重試可能開出
 * 第二張發票。顏色不承載這個差異，中文與 A06 的處置說明才承載。
 */
export const INVOICE_OPERATION_STATUS_BADGE_VARIANT: Record<
	string,
	BadgeVariant
> = {
	[InvoiceOperationStatus.CREATED]: "gray",
	[InvoiceOperationStatus.AWAITING_TRIGGER]: "gray",
	[InvoiceOperationStatus.READY]: "gray",
	[InvoiceOperationStatus.HELD]: "warning",
	[InvoiceOperationStatus.SENDING]: "info",
	[InvoiceOperationStatus.RETRY_SCHEDULED]: "warning",
	[InvoiceOperationStatus.WAITING_CONFIRMATION]: "warning",
	[InvoiceOperationStatus.PROVIDER_STATUS_RECHECK]: "info",
	[InvoiceOperationStatus.SUCCEEDED]: "success",
	[InvoiceOperationStatus.FAILED_PERMANENT]: "danger",
	[InvoiceOperationStatus.FAILED_RETRY_EXHAUSTED]: "danger",
	[InvoiceOperationStatus.FAILED_UNKNOWN]: "danger",
	[InvoiceOperationStatus.SELLER_ACTION_REQUIRED]: "warning",
	[InvoiceOperationStatus.CANCEL_REQUESTED]: "warning",
	[InvoiceOperationStatus.CANCEL_BLOCKED_UNKNOWN]: "danger",
	[InvoiceOperationStatus.CANCELLED_BY_ADMIN]: "gray",
	[InvoiceOperationStatus.MANUAL_PROCESS_ALLOWED]: "warning",
	[InvoiceOperationStatus.RESOLVED_EXTERNALLY]: "success",
	[InvoiceOperationStatus.NOT_APPLICABLE]: "gray",
	[InvoiceOperationStatus.CANCELLED_BEFORE_ISSUE]: "gray",
};

/**
 * 端點 17／39 的合成值：`seller_tax_profiles` 查無該賣家的列時回這個。
 *
 * 🔴 它**不進 DB**（不在 `chk_stp_review_status` 的 10 值裡），但**會出現在回應中**。
 * 照 CHECK 抄 10 個的症狀是：上線當天全體尚未申請的賣家 badge 落 gray。
 *
 * 2026-08-30 起它**已在 CodeMeta 的 `SellerTaxReviewStatus` 內**（label「尚未開始」），
 * `codeLabel` 查得到中文。先前的缺口已補，不要再在前端補第二份碼表。
 */
export const TAX_REVIEW_STATUS_NOT_STARTED = "NOT_STARTED";

/**
 * `SellerTaxReviewStatus`（**11 值，全部來自 CodeMeta**）。
 * A02 佇列與 A03 詳情共用。
 */
export const TAX_REVIEW_STATUS_BADGE_VARIANT: Record<string, BadgeVariant> = {
	[TAX_REVIEW_STATUS_NOT_STARTED]: "gray",
	[SellerTaxReviewStatus.DRAFT]: "gray",
	[SellerTaxReviewStatus.SUBMITTED]: "info",
	[SellerTaxReviewStatus.UNDER_REVIEW]: "info",
	[SellerTaxReviewStatus.SUPPLEMENT_REQUIRED]: "warning",
	[SellerTaxReviewStatus.SELF_ISSUE_PENDING]: "warning",
	[SellerTaxReviewStatus.APPROVED_INVOICE_REQUIRED]: "success",
	[SellerTaxReviewStatus.APPROVED_INVOICE_EXEMPT]: "success",
	[SellerTaxReviewStatus.REJECTED]: "danger",
	[SellerTaxReviewStatus.EXPIRED]: "danger",
	[SellerTaxReviewStatus.REVOKED]: "danger",
};

/**
 * `InvoiceProviderAccountStatus`（**13 值 ＝ 11 個 DB 值 ＋ 2 個投影值**）。
 * A04 列表與詳情共用。
 *
 * 🔴 **`PROVIDER_UNAVAILABLE` 是 `warning` 不是 `danger`，這不是配色偏好**：它代表
 * 「AMEGO 現在連不上」，不是這個賣家的設定問題。畫成 danger 會讓 ADM 打電話去跟
 * 賣家要 AppKey，而賣家那邊什麼都沒壞。
 *
 * 🔴 **沒有 `SUSPENDING`**（CodeMeta 已於 2026-08-25 依 58 §7.3.4 刪除，DRAIN 整條分支
 * 不存在）。這裡故意不補那一格：補了就是一個永遠不會命中的鍵，而下一個維護者會
 * 以為那個狀態存在。收到它一律落 default。
 *
 * ⚠️ 三個「還沒設定好」的值刻意分開，`??` fallback 一律不得把它們壓成同一個
 * （陷阱 T16）：`NOT_STARTED`（上線當天的全體賣家）／`NOT_CONFIGURED`（已建立未填憑證）
 * ／`CONFIG_ERROR`（真正的配置錯誤，會進 A01 的 KPI）。
 */
export const PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT: Record<
	string,
	BadgeVariant
> = {
	[InvoiceProviderAccountStatus.NOT_STARTED]: "gray",
	[InvoiceProviderAccountStatus.NOT_CONFIGURED]: "warning",
	[InvoiceProviderAccountStatus.CONFIG_ERROR]: "danger",
	[InvoiceProviderAccountStatus.CREDENTIAL_SUBMITTED]: "warning",
	[InvoiceProviderAccountStatus.CREDENTIAL_VERIFYING]: "info",
	[InvoiceProviderAccountStatus.ACTIVE]: "success",
	[InvoiceProviderAccountStatus.ROTATION_PENDING]: "info",
	[InvoiceProviderAccountStatus.INVALID_CREDENTIAL]: "danger",
	[InvoiceProviderAccountStatus.PROVIDER_REJECTED]: "danger",
	[InvoiceProviderAccountStatus.PROVIDER_UNAVAILABLE]: "warning",
	[InvoiceProviderAccountStatus.DEGRADED]: "warning",
	[InvoiceProviderAccountStatus.SUSPENDED]: "gray",
	[InvoiceProviderAccountStatus.REVOKED]: "gray",
};

/**
 * `InvoiceComplaintStatus`（**7 值，一個都不能少也不能多**）。A07 佇列與 A08 詳情共用。
 *
 * ⚠️ **不要收 `PLATFORM_ASSIST_REQUESTED` 與 `REOPENED`**：它們不在 `chk_invcomp_status`
 * 的值域裡（CodeMeta 也沒有），多收就是兩個永遠不會命中的鍵——不會出錯，但會誤導
 * 下一個維護者以為那兩個狀態存在。重開案是 `reopenCount > 0`，狀態仍是 `OPEN`。
 *
 * ⚠️ `OVERDUE` 是 danger 而 `ESCALATED_TO_ADM` 是 warning：前者是 SLA 已經破了
 * （會計違規），後者只是輪到平台處理。
 */
export const COMPLAINT_STATUS_BADGE_VARIANT: Record<string, BadgeVariant> = {
	[InvoiceComplaintStatus.OPEN]: "info",
	[InvoiceComplaintStatus.SELLER_RESPONDED]: "info",
	[InvoiceComplaintStatus.OVERDUE]: "danger",
	[InvoiceComplaintStatus.ESCALATED_TO_ADM]: "warning",
	[InvoiceComplaintStatus.RESOLVED]: "success",
	[InvoiceComplaintStatus.VIOLATION_CONFIRMED]: "danger",
	[InvoiceComplaintStatus.WITHDRAWN]: "gray",
};

/**
 * `InvoiceDocumentStatus`（**11 值，一個都不能少**）。A12 列表與 A13 詳情共用同一份。
 *
 * 🔴 `VOIDING` 不能省：「作廢動作已受理」與「憑證真的變成作廢」之間必須有中間態，
 * 少了它，作廢中的發票會落 `?? "gray"` 而看起來就像已經作廢了。
 *
 * 🔴 `RESOLVED_EXTERNALLY` 是 success 但語意不是「平台開好了」：它是「賣家自行開立
 * 並回報」的終態。中文由 CodeMeta 下發（「賣家自行開立並回報」），顏色只說「這件事
 * 結束了、不必再追」。
 *
 * ⚠️ **不收 `SELF_ISSUED_REPORTED` / `DELEGATED_TO_ISSUER` / `CANCELLED`**：三者在
 * `chk_invdoc_status` 的值域內零命中，補進來就是三個永遠不會命中的鍵，而下一個
 * 維護者會以為那三個狀態存在。
 */
export const DOCUMENT_STATUS_BADGE_VARIANT: Record<string, BadgeVariant> = {
	[InvoiceDocumentStatus.PENDING]: "info",
	[InvoiceDocumentStatus.ISSUING]: "info",
	[InvoiceDocumentStatus.ISSUED]: "success",
	[InvoiceDocumentStatus.ISSUE_FAILED]: "danger",
	[InvoiceDocumentStatus.AWAITING_MANUAL]: "warning",
	[InvoiceDocumentStatus.VOIDING]: "info",
	[InvoiceDocumentStatus.VOIDED]: "gray",
	[InvoiceDocumentStatus.SUPERSEDED]: "gray",
	[InvoiceDocumentStatus.CANCELLED_BEFORE_ISSUE]: "gray",
	[InvoiceDocumentStatus.NOT_APPLICABLE]: "gray",
	[InvoiceDocumentStatus.RESOLVED_EXTERNALLY]: "success",
};

/**
 * `InvoiceFulfillmentMode`（4 值）。A12 的「模式」欄。
 *
 * `LEGACY_MANUAL` 是 gray 不是 danger：切點之前付款的訂單本來就走人工，那是設計而不是
 * 故障。畫成紅色會讓 ADM 去追一批根本沒有問題的舊訂單。
 */
export const FULFILLMENT_MODE_BADGE_VARIANT: Record<string, BadgeVariant> = {
	[InvoiceFulfillmentMode.AMEGO_AUTO]: "success",
	[InvoiceFulfillmentMode.SELLER_SELF_ISSUE]: "gray",
	[InvoiceFulfillmentMode.PLATFORM_RECEIPT]: "info",
	[InvoiceFulfillmentMode.LEGACY_MANUAL]: "gray",
};
