/**
 * S05 的資格 gate 文案（`ineligibleReason` → 標題／說明）。
 *
 * 🔴 **必須有 default 分支**：後端先上線一個前端還不知道的理由值時，落一句保守文案
 * 而不是白畫面。`unknown` 旗標讓測試驗得出「已知值真的有進 map」。
 *
 * 🔴 三種理由的**處置相同（都導向 S02）但描述不同**，不可合併成一句：
 * `NOT_APPROVED` 要賣家先把稅務身分送出審核，「已撤銷」要他知道既有收據不受影響，
 * 「已到期」要他重新提交。合併之後賣家不知道自己是哪一種。
 *
 * ⚠️ `NOT_APPROVED` 的母體在 DEC-20260905-015836 之後**縮小了**：端點 29 的
 * allowed-from 已含四個審核進行中的狀態（`SUBMITTED`／`UNDER_REVIEW`／
 * `SUPPLEMENT_REQUIRED`／`SELF_ISSUE_PENDING`），它們現在拿到的是 `eligible=true`。
 * 落到這裡的只剩「尚未送出審核或還在草稿」「審核未通過」「已逾期」「核定為應開立
 * 統一發票」四種人 ⇒ 文案不得再寫成「只開放給已核定免用統一發票的商店」，
 * 那句話對現在真正看到它的人全部是錯的。
 */

export interface ReceiptAuthGateCopy {
	title: string;
	description: string;
	ctaLabel: string;
	ctaHref: string;
	unknown: boolean;
}

const TAX_STATUS_HREF = "/seller-center/invoices/tax-status";

export const RECEIPT_INELIGIBLE_REASON = {
	NOT_APPROVED: "NOT_APPROVED",
	APPROVAL_REVOKED: "APPROVAL_REVOKED",
	APPROVAL_EXPIRED: "APPROVAL_EXPIRED",
} as const;

export function receiptAuthGateCopy(reason: string): ReceiptAuthGateCopy {
	const cta = { ctaLabel: "前往稅務身分", ctaHref: TAX_STATUS_HREF };
	switch (reason) {
		case RECEIPT_INELIGIBLE_REASON.NOT_APPROVED:
			return {
				title: "目前還不能授權平台代製收據",
				description:
					"送出稅務身分審核之後就可以先簽這份授權，不必等核定結果。你目前尚未送出審核、審核未通過或已逾期，或已核定為應開立統一發票——請到「稅務身分」確認狀態與下一步。",
				...cta,
				unknown: false,
			};
		case RECEIPT_INELIGIBLE_REASON.APPROVAL_REVOKED:
			return {
				title: "免用統一發票資格已撤銷",
				description:
					"平台已撤銷這家商店的免用統一發票核定。既有的收據不受影響，但目前無法新增授權。",
				...cta,
				unknown: false,
			};
		case RECEIPT_INELIGIBLE_REASON.APPROVAL_EXPIRED:
			return {
				title: "免用統一發票核定已到期",
				description: "核定已超過有效期限，請重新提交核定證明以恢復。",
				...cta,
				unknown: false,
			};
		default:
			return {
				title: "目前無法使用收據代製授權",
				description: "資格狀態更新中，請稍後再試或洽平台客服。",
				...cta,
				unknown: true,
			};
	}
}

/**
 * 資格已失效（撤銷／到期）⇒ 整頁唯讀 ＋ 引導重新審核，但**歷史核定資料仍要顯示**。
 * 與 `NOT_APPROVED`（從來沒核定過、沒有歷史資料可看）是不同的處置。
 */
export function isRevokedOrExpired(reason: string): boolean {
	return (
		reason === RECEIPT_INELIGIBLE_REASON.APPROVAL_REVOKED ||
		reason === RECEIPT_INELIGIBLE_REASON.APPROVAL_EXPIRED
	);
}
