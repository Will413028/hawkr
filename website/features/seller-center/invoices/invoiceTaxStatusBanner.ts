/**
 * N6｜`SellerTaxReviewStatus`（10 個 DB 值 ＋ 1 個合成的 `NOT_STARTED`）→ 稅務身分
 * 狀態卡的呈現決策（58A §B-2.3 (8)）。
 *
 * 🔴 **不要複製 `lib/seller/kyc-flow.ts` 的 `kycFlowBanner()` 再改**：那一支
 * 是商家認證流程狀態的文案表，狀態機與本表不同（沒有期限倒數、沒有「已延長一次」旗標、
 * 沒有接單能力這第二維度）。複製之後會有人以為兩者同源而一起改，然後在其中一邊改出
 * 一個對另一邊錯的文案。這裡只沿用它的 `StatusBannerTone` 型別。
 *
 * 🔴 **必須有 default 分支**（§B-2.0 T13）：11 個值逐一有歸屬，值域外落
 * `狀態更新中` 而不是白畫面。default **不得**寫成把已知值一併吃掉的萬用分支
 * ——`unknown` 旗標就是為了讓測試驗得出這件事。
 *
 * 🔴 「賣家自述的 `claimedStatus`」與「平台核定的 `reviewStatus`」是兩個維度，本檔
 * 只吃後者。UI 不得把自述免用寫成「已核定免用統一發票」——那是 ADM 的裁決（§S02 ⑤）。
 */

import { SellerTaxReviewStatus } from "@/lib/codemeta/enums";
import type { StatusBannerTone } from "@/lib/seller/kyc-flow";

export interface InvoiceTaxStatusBanner {
	tone: StatusBannerTone;
	title: string;
	description: string;
	/** 待審類才渲染 `DeadlineCountdown`；其他狀態的 `deadlineAt` 沒有意義。 */
	showDeadline: boolean;
	/**
	 * 這個狀態下賣家是否還能編輯稅務資料。
	 *
	 * ⚠️ 這是**狀態機的性質**，不是某一波的實作細節：端點 18／19／20 與它們的表單
	 * （`tax-status/TaxIdentityForm` 等）都已接上，`editable` 現在同時決定文案與
	 * 表單是否可寫。改動它之前先確認 T02 的 allowed-from，不要為了讓某個畫面能編輯
	 * 而放寬這裡——後端會用 9321 擋下來，而賣家看到的是一句沒有下一步的錯誤。
	 */
	editable: boolean;
	/** 只在目標路由這一波真的存在時才給值——見本檔尾註。 */
	ctaLabel?: string;
	ctaHref?: string;
	/** 落到 default 分支時為 true，供除錯與測試辨識。 */
	unknown: boolean;
}

const TAX_STATUS_HREF = "/seller-center/invoices/tax-status";

/**
 * 審核進行中三個狀態共用的一句提示。
 *
 * DEC-20260905-015836 選 A：端點 29 的 allowed-from 已含審核進行中的四個狀態，
 * 賣家不必等核定就能先簽收據代製授權。少了這句他不會知道現在就可以去簽，而
 * 核定完成到他實際簽署之間的每一筆已付訂單，收據都會卡成待人工處理——那正是這次
 * 放寬要消滅的窗口。
 *
 * 🔴 措辭是**條件式**的：這幾個狀態最後也可能被核定為「應開立統一發票」，那種賣家
 * 永遠用不到收據授權，不得對他寫成「你要去簽」。
 */
const RECEIPT_AUTH_HINT =
	"若你屬於免用統一發票，現在就可以先完成收據代製授權，核定後立即生效。";

interface BannerInput {
	reviewStatus: string;
	/** 自行開票責任是否生效中；生效時在描述後追加一句。 */
	selfIssueEffective?: boolean;
	/** `REJECTED`／`EXPIRED` 的重新申請冷卻終點（已格式化的台北日期字串）。 */
	cooldownUntilText?: string;
	/** 呼叫端是不是 S01（總覽）——只有總覽才需要「前往稅務身分」的導覽 CTA。 */
	withNavigationCta?: boolean;
}

export function invoiceTaxStatusBanner(
	input: BannerInput,
): InvoiceTaxStatusBanner {
	const base = bannerFor(input);
	const description = input.selfIssueEffective
		? `${base.description}審核期間由你自行開立與交付發票。`
		: base.description;
	if (!input.withNavigationCta || base.unknown) {
		return { ...base, description };
	}
	return {
		...base,
		description,
		ctaLabel: "查看稅務身分",
		ctaHref: TAX_STATUS_HREF,
	};
}

function bannerFor(
	input: BannerInput,
): Omit<InvoiceTaxStatusBanner, "ctaLabel" | "ctaHref"> {
	const cooldown = input.cooldownUntilText
		? `可於 ${input.cooldownUntilText} 後重新申請。`
		: "";

	switch (input.reviewStatus) {
		case SellerTaxReviewStatus.NOT_STARTED:
			return {
				tone: "neutral",
				title: "尚未開始稅務身分設定",
				description:
					"完成稅務身分設定後，平台才會依核定結果決定發票的開立方式。",
				showDeadline: false,
				editable: true,
				unknown: false,
			};
		case SellerTaxReviewStatus.DRAFT:
			return {
				tone: "neutral",
				title: "稅務身分資料尚未送出",
				description: "資料還在草稿狀態，送出審核後平台才會開始處理。",
				showDeadline: false,
				editable: true,
				unknown: false,
			};
		case SellerTaxReviewStatus.SUPPLEMENT_REQUIRED:
			return {
				tone: "warning",
				title: "稅務身分需要補件",
				description: `請依平台備註補齊資料後重新送出；審核期限不會重新計算。${RECEIPT_AUTH_HINT}`,
				showDeadline: true,
				editable: true,
				unknown: false,
			};
		case SellerTaxReviewStatus.REJECTED:
			return {
				tone: "danger",
				title: "稅務身分審核未通過",
				description: `請修正資料後重新申請。${cooldown}`,
				showDeadline: false,
				editable: true,
				unknown: false,
			};
		case SellerTaxReviewStatus.EXPIRED:
			return {
				tone: "danger",
				title: "審核期限已過",
				description: `審核期限已過，目前無法接受新訂單。請重新送出申請；完成審核後會自動恢復。${cooldown}`,
				showDeadline: false,
				editable: true,
				unknown: false,
			};
		case SellerTaxReviewStatus.SUBMITTED:
		case SellerTaxReviewStatus.UNDER_REVIEW:
			return {
				tone: "info",
				title: "稅務身分審核中",
				description: `平台審核期間資料無法修改，完成後會通知你。${RECEIPT_AUTH_HINT}`,
				showDeadline: true,
				editable: false,
				unknown: false,
			};
		case SellerTaxReviewStatus.SELF_ISSUE_PENDING:
			return {
				tone: "info",
				title: "已具結，等待平台審核",
				description: `審核完成前，這段期間的發票由你自行開立與交付。${RECEIPT_AUTH_HINT}`,
				showDeadline: true,
				editable: false,
				unknown: false,
			};
		case SellerTaxReviewStatus.APPROVED_INVOICE_REQUIRED:
			return {
				tone: "success",
				title: "已核定應開立統一發票",
				description: "完成 AMEGO 串接後，平台會自動為你開立發票。",
				showDeadline: false,
				editable: false,
				unknown: false,
			};
		case SellerTaxReviewStatus.APPROVED_INVOICE_EXEMPT:
			return {
				tone: "success",
				title: "已核定免用統一發票",
				description: "完成收據代製授權後，平台會依你的授權製作收據。",
				showDeadline: false,
				editable: false,
				unknown: false,
			};
		case SellerTaxReviewStatus.REVOKED:
			return {
				tone: "danger",
				title: "稅務身分已被撤銷",
				description: "請聯繫平台客服了解後續處理方式。",
				showDeadline: false,
				editable: false,
				unknown: false,
			};
		default:
			return {
				tone: "neutral",
				title: "狀態更新中",
				description: "這個稅務身分狀態尚未支援顯示，請稍後重新整理。",
				showDeadline: false,
				editable: false,
				unknown: true,
			};
	}
}
