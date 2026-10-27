import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	type AuthorityOrderKind,
	AuthorityTargetNoticeKind,
	type CompleteAuthorityOrderRequestSchema,
	type CreateAuthorityOrderRequestSchema,
	type ExtendAuthorityOrderHoldRequestSchema,
	AuthorityOrderKind as Kind,
	type RecordAuthorityTargetNoticeRequestSchema,
	type RevokeAuthorityOrderRequestSchema,
	type TargetNoticeInputSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { parseDate } from "@hawkr/api/proto/date";
import { trimGoSpace } from "@hawkr/api/text/server-text";
import { fromLocalInput } from "@/app/admin/merchants/[id]/measure";

// 主管機關來函的對話框與登記頁：輸入的驗證、請求組裝、失敗句（3c-b 設計書 §5.3、§5.4 (d)(g)）。能不能做來自伺服器
// （AuthorityOrder.can_complete、can_extend_hold）；欄位的長度、範圍與組合由伺服器判斷並回已核准的句子，這裡不重做，
// 只擋有核准文案的「每個對象都要選告知方式」。

/** 來函的版本衝突（L21）。 */
export const AUTHORITY_CONFLICT_TEXT =
	"這筆來函剛被更新，頁面已重新載入；請確認最新狀態後再操作。";
/** 來函的寫入失敗（網路層）：沿用詢問那一句（J15），沒有寫入。 */
export const AUTHORITY_FAILED_TEXT =
	"記下失敗，沒有寫入；請確認網路狀態後再試一次。";
/** 每個對象都要選告知方式（伺服器句子 authority_order.target_notice_required，已核准）。 */
export const TARGET_NOTICE_REQUIRED_TEXT = "請為每個指名的對象選擇告知的方式。";

/** 登記頁對象列的操作（已核准）。 */
export const ADD_TARGET_TEXT = "新增對象";
export const REMOVE_TARGET_TEXT = "移除";

// ---------------------------------------------------------------------------
// 登記來函

export interface TargetRowForm {
	/** 列的穩定鍵（只給畫面用，不送出）。 */
	key: string;
	accountId: string;
	/** 選填：只指名這個店家；空字串＝帳號全部的店家。 */
	storeId: string;
	/** 商品 ID，以逗號分隔（選填）。 */
	productIds: string;
}

export interface AuthorityOrderForm {
	agency: string;
	documentNo: string;
	/** 收文時間，`datetime-local`（台北）。 */
	receiveTime: string;
	kind: AuthorityOrderKind;
	summary: string;
	targets: readonly TargetRowForm[];
	/** 期限，`datetime-local`（台北）；空字串＝不帶，由伺服器依種類取預設（L12）。 */
	dueTime: string;
}

/** 商品 ID 欄的拆法：逗號（半形或全形）分隔，去掉前後空白與空項。 */
export function parseProductIds(text: string): string[] {
	return text
		.split(/[,，]/u)
		.map((id) => trimGoSpace(id))
		.filter((id) => id !== "");
}

/** 輸入內容的鍵：與送出的請求一一對應，request_id 依它換新（同 CreateMeasure）。 */
export function authorityOrderRequestKey(form: AuthorityOrderForm): string {
	return JSON.stringify([
		trimGoSpace(form.agency),
		trimGoSpace(form.documentNo),
		form.receiveTime,
		form.kind,
		trimGoSpace(form.summary),
		form.targets.map((t) => [
			trimGoSpace(t.accountId),
			t.storeId,
			parseProductIds(t.productIds),
		]),
		form.dueTime,
	]);
}

/**
 * 登記的請求。時間讀不出來（沒填）時不帶這個欄位，由伺服器回範圍句子；對象的 ID 去掉前後空白。
 * 期限沒填就不帶，預設由伺服器依種類決定（客戶端不複製這個常數）。
 */
export function buildCreateAuthorityOrder(
	requestId: string,
	form: AuthorityOrderForm,
): MessageInitShape<typeof CreateAuthorityOrderRequestSchema> {
	const receive = fromLocalInput(form.receiveTime);
	const due = fromLocalInput(form.dueTime);
	return {
		requestId,
		agency: trimGoSpace(form.agency),
		documentNo: trimGoSpace(form.documentNo),
		kind: form.kind,
		summary: trimGoSpace(form.summary),
		targets: form.targets.map((t) => ({
			accountId: trimGoSpace(t.accountId),
			storeId: t.storeId,
			productIds: parseProductIds(t.productIds),
		})),
		...(receive ? { receiveTime: timestampFromDate(receive) } : {}),
		...(due ? { dueTime: timestampFromDate(due) } : {}),
	};
}

// ---------------------------------------------------------------------------
// 告知（完成來函、記下告知）

export interface NoticeForm {
	/** UNSPECIFIED＝還沒選。 */
	kind: AuthorityTargetNoticeKind;
	/** 已告知的時間，`datetime-local`（台北）。 */
	notifyTime: string;
	/** 告知的方式或說明。 */
	note: string;
	/** 向機關確認能否告知的日期，`YYYY-MM-DD`（台北）。 */
	recheckDate: string;
}

export const EMPTY_NOTICE_FORM: NoticeForm = {
	kind: AuthorityTargetNoticeKind.UNSPECIFIED,
	notifyTime: "",
	note: "",
	recheckDate: "",
};

/** 一個告知的訊息：只送這個種類對應的欄位（已告知看時間與方式、暫不告知看確認日期與說明、由措施告知都不帶）。 */
export function buildNotice(
	form: NoticeForm,
): MessageInitShape<typeof TargetNoticeInputSchema>["notice"] {
	switch (form.kind) {
		case AuthorityTargetNoticeKind.NOTIFIED: {
			const at = fromLocalInput(form.notifyTime);
			return {
				kind: form.kind,
				note: trimGoSpace(form.note),
				...(at ? { notifyTime: timestampFromDate(at) } : {}),
			};
		}
		case AuthorityTargetNoticeKind.WITHHELD: {
			const recheckDate = parseDate(form.recheckDate);
			return {
				kind: form.kind,
				note: trimGoSpace(form.note),
				...(recheckDate ? { recheckDate } : {}),
			};
		}
		default:
			return { kind: form.kind };
	}
}

// ---------------------------------------------------------------------------
// 完成

export interface CompleteForm {
	resolutionSummary: string;
	/** 調取資料的來函：實際提供資料的時間，`datetime-local`；空字串＝現在（由伺服器補）。 */
	provideTime: string;
	/** 依對象的位置，一個對象一筆。 */
	notices: readonly NoticeForm[];
}

export function emptyCompleteForm(targetCount: number): CompleteForm {
	return {
		resolutionSummary: "",
		provideTime: "",
		notices: Array.from({ length: targetCount }, () => EMPTY_NOTICE_FORM),
	};
}

export function validateComplete(form: CompleteForm): string | undefined {
	if (
		form.notices.some((n) => n.kind === AuthorityTargetNoticeKind.UNSPECIFIED)
	) {
		return TARGET_NOTICE_REQUIRED_TEXT;
	}
	return undefined;
}

/** 完成的請求；`provideTime` 只在調取資料的來函帶（其他種類伺服器會拒絕）。 */
export function buildComplete(
	order: { id: string; etag: string; kind: AuthorityOrderKind },
	form: CompleteForm,
): MessageInitShape<typeof CompleteAuthorityOrderRequestSchema> {
	const provide =
		order.kind === Kind.DATA_REQUEST
			? fromLocalInput(form.provideTime)
			: undefined;
	return {
		id: order.id,
		etag: order.etag,
		resolutionSummary: trimGoSpace(form.resolutionSummary),
		targetNotices: form.notices.map((notice, targetPosition) => ({
			targetPosition,
			notice: buildNotice(notice),
		})),
		...(provide ? { provideTime: timestampFromDate(provide) } : {}),
	};
}

// ---------------------------------------------------------------------------
// 延長保存、記下告知

export interface ExtendForm {
	/** 新的保存期限，`datetime-local`（台北）。 */
	holdUntil: string;
	reason: string;
}

export const EMPTY_EXTEND_FORM: ExtendForm = { holdUntil: "", reason: "" };

export function buildExtend(
	order: { id: string; etag: string },
	form: ExtendForm,
): MessageInitShape<typeof ExtendAuthorityOrderHoldRequestSchema> {
	const until = fromLocalInput(form.holdUntil);
	return {
		id: order.id,
		etag: order.etag,
		reason: trimGoSpace(form.reason),
		...(until ? { holdUntil: timestampFromDate(until) } : {}),
	};
}

export function buildRecordNotice(
	order: { id: string; etag: string },
	targetPosition: number,
	form: NoticeForm,
): MessageInitShape<typeof RecordAuthorityTargetNoticeRequestSchema> {
	return {
		id: order.id,
		etag: order.etag,
		targetPosition,
		notice: buildNotice(form),
	};
}

// ---------------------------------------------------------------------------
// 撤銷登記

/** 撤銷登記的請求：原因去掉前後空白（長度由伺服器判斷並回已核准的句子 authority_order.revoke_reason_length）。 */
export function buildRevoke(
	order: { id: string; etag: string },
	reason: string,
): MessageInitShape<typeof RevokeAuthorityOrderRequestSchema> {
	return { id: order.id, etag: order.etag, reason: trimGoSpace(reason) };
}
