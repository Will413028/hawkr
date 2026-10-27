import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, type Transport } from "@connectrpc/connect";
import { createQueryOptions } from "@connectrpc/connect-query";
import {
	type GetKYCApplicationResponse,
	type KYCApplication,
	type KYCDocumentView,
	type KYCRegistration,
	type KYCRegistrationCheck,
	KYCRegistrationOutcome,
	KYCRegistrationStatus,
	type KYCReview,
	KYCReviewAction,
	type PayUniRegistration,
	type RevealKYCSensitiveResponse,
	SellerAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	KYCField,
	type KYCIssue,
	KYCIssueCode,
	PayUniBindStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { formatDate as formatCalendarDate } from "@hawkr/api/proto/date";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { sellerKYCField } from "@hawkr/ui-text/keys";
import { type QueryClient, skipToken } from "@tanstack/react-query";
import type { ActionResult } from "@/lib/connect-action";
import { formatDateTimeShort } from "@/lib/format";
import { staffTextOrUnknown, unknownText } from "@/lib/ui-text";
import { firstVisibleLabel } from "@/lib/unavailability";
import { addressText } from "./store-detail";

// 商家詳情的商家認證（KYC）審核與 PayUni 的顯示判斷與文案（SellerAdminService.GetKYCApplication、RevealKYCSensitive、
// ApproveKYC、RejectKYC、RequeryKYCRegistration、GetPayUniRegistration、RebindPayUni）。畫面只管呈現；哪個狀態
// 可以核准、退回的逐項問題怎麼組成與驗證、顯示全文何時收回、GCIS 並列怎麼標記、背景工作怎麼輪詢都在這裡，才能不靠畫面測試
// （決策簡報 K7、K8、K9、K15、K16、D4 第 4、5、6 點；2b 設計簡報題 1 (b′)、題 2）。

// ---------------------------------------------------------------------------
// D4 的文案（已核准，照原文；標註「實作者補」的是 D4 沒有原文的部分）

export const REVEAL_DIALOG_TEXT =
	"顯示全文會記錄你的帳號、時間與理由；15 分鐘內可以檢視完整號碼與證件影像。";
export const DOCUMENT_NEEDS_REVEAL_TEXT = "需先顯示全文";
export const GCIS_PENDING_TEXT = "登記資料查詢中…";
/** 實作者補：查詢超過伺服器保證的期限仍沒有終態（伺服器的錯誤，或這台電腦的時鐘比伺服器快）時的說明。 */
export const GCIS_OVERDUE_TEXT = "登記資料查詢超過預定時間仍未完成";
export const GCIS_OVERDUE_DETAIL = "請按「重新查詢」重新送出。";
export const GCIS_NONE_TEXT = "沒有登記資料";
export const GCIS_NOT_FOUND_TEXT = "查無此統編";
export const GCIS_UNAVAILABLE_TEXT = "查詢失敗";

/** 實作者補：核准與退回遇到 VERSION_CONFLICT 時的說明（輸入保留，畫面已重讀）。 */
export const KYC_CONFLICT_TEXT =
	"申請的內容已經變動（可能別的管理員先處理了，或登記資料有了新的查詢結果）。畫面已重新讀取，你輸入的內容還在，請確認後再送出。";
/** 實作者補：核准成功後的頁首提示（PayUni 綁定由背景程序完成）。 */
export const APPROVE_SUCCESS_TEXT =
	"已通過審核。系統會在背景向 PayUni 註冊，結果會顯示在下方的 PayUni 卡片。";
export const REJECT_SUCCESS_TEXT =
	"已退回申請，賣家可依你指出的問題修改後重新送審。";
/** 實作者補：重新綁定與重新查詢的成功提示。 */
export const REBIND_SUCCESS_TEXT =
	"已重新排入 PayUni 綁定，結果會更新在 PayUni 卡片上。";
export const REQUERY_SUCCESS_TEXT =
	"已排入重新查詢，結果出來後會更新在商業登記欄。";

// ---------------------------------------------------------------------------
// 申請的讀取狀態

/** 申請內容的讀取狀態：沒有送出過的申請（GetStore 沒有 kyc 摘要）時是 none，畫面不顯示 KYC 區塊。 */
export type KycApplicationState =
	| { state: "none" }
	| { state: "loading" }
	| { state: "error" }
	| { state: "ready"; application: KYCApplication };

/**
 * GetKYCApplication 的結果轉成讀取狀態。摘要（GetStore.kyc）沒有、或 application_id 是空字串時是 none（不發請求）；
 * 申請讀不到（not_found：草稿或已被取消）也是 none；其他錯誤是 error（可重試）；成功卻沒有申請是 error，不當成沒有申請。
 */
export function kycApplicationOf(
	applicationId: string | undefined,
	result: {
		error: unknown;
		data: { application?: KYCApplication } | undefined;
	},
): KycApplicationState {
	if (!applicationId) return { state: "none" };
	if (result.error != null) {
		return result.error instanceof ConnectError &&
			result.error.code === Code.NotFound
			? { state: "none" }
			: { state: "error" };
	}
	if (result.data === undefined) return { state: "loading" };
	if (!result.data.application) return { state: "error" };
	return { state: "ready", application: result.data.application };
}

// ---------------------------------------------------------------------------
// 背景工作的輪詢（簡報題 1 (b′)）：進行中的查詢與綁定都是有期限、伺服器保證終態的資源，client 以退避輪詢到終態或期限。
// 頁籤隱藏時不輪詢（TanStack 的 refetchIntervalInBackground 預設為 false）。

/** 退避的下限與上限（毫秒）：從要求的時間起，間隔隨已等待的時間成長（已等待時間的 1/4），上限 60 秒。 */
export const POLL_MIN_MS = 3000;
export const POLL_MAX_MS = 60_000;
const PAYUNI_POLL_MIN_MS = 5000;

function backoffMs(elapsedMs: number, minMs: number, maxMs: number): number {
	return Math.min(maxMs, Math.max(minMs, Math.floor(elapsedMs / 4)));
}

/**
 * 進行中的登記資料查詢該多久之後再讀一次申請；沒有進行中的查詢、或已超過期限（伺服器保證不會發生；沒有期限的當成超過）時不輪詢
 * （false）。下一次讀取不晚於期限之後一秒，才看得到期限那一刻的終態。
 */
export function registrationPollInterval(
	pending: KYCRegistrationCheck | undefined,
	now: Date,
): number | false {
	if (!pending) return false;
	if (!pending.requestTime || !pending.deadlineTime) return false;
	const remaining =
		timestampDate(pending.deadlineTime).getTime() - now.getTime();
	if (remaining <= 0) return false;
	const elapsed = Math.max(
		0,
		now.getTime() - timestampDate(pending.requestTime).getTime(),
	);
	return Math.min(
		backoffMs(elapsed, POLL_MIN_MS, POLL_MAX_MS),
		remaining + 1000,
	);
}

/** 進行中的查詢已超過伺服器保證的期限（還沒有終態）。 */
export function registrationOverdue(
	pending: KYCRegistrationCheck | undefined,
	now: Date,
): boolean {
	if (!pending) return false;
	if (!pending.deadlineTime) return true;
	return now.getTime() >= timestampDate(pending.deadlineTime).getTime();
}

/**
 * PayUni 綁定（bind_status = PENDING）的輪詢間隔：伺服器的 payuni_sweep 保證會到終態（BOUND 或 FAILED），所以沒有次數上限，
 * 間隔以距離上次更新的時間退避、上限 60 秒；其他綁定狀態不輪詢。
 */
export function payUniPollInterval(
	registration: PayUniRegistration | undefined,
	now: Date,
): number | false {
	if (registration?.bindStatus !== PayUniBindStatus.PENDING) return false;
	const updated = registration.updateTime ?? registration.createTime;
	const elapsed = updated
		? Math.max(0, now.getTime() - timestampDate(updated).getTime())
		: 0;
	return backoffMs(elapsed, PAYUNI_POLL_MIN_MS, POLL_MAX_MS);
}

// ---------------------------------------------------------------------------
// 核准、退回與重新查詢的可用性

export interface ReviewActions {
	/** 核准：伺服器說能核准（`can_approve`：審核中、店家營運中、帳號沒有停權），而且帶得出 id 與 etag（資料完整性，不是業務規則）。 */
	canApprove: boolean;
	/** 退回：伺服器說能退回（`can_reject`），而且帶得出 id 與 etag。 */
	canReject: boolean;
	/** 重新查詢登記資料：伺服器說能重新查詢（`can_requery_registration`）。 */
	canRequery: boolean;
}

/**
 * 核准、退回與重新查詢的可用性只讀伺服器的旗標（前提 2026-10-15）；client 不從申請狀態、賣家型態或店家狀態推導。
 * 申請還沒讀到、沒有 ID 或 etag 時不能核准與退回：空字串不是有效值，送出去的請求沒有意義。
 */
export function reviewActions(
	application: KYCApplication | undefined,
): ReviewActions {
	const complete =
		application !== undefined &&
		application.id !== "" &&
		application.etag !== "";
	return {
		canApprove: complete && application.canApprove,
		canReject: complete && application.canReject,
		canRequery: application?.canRequeryRegistration === true,
	};
}

/**
 * 不能核准時詳情頁顯示的那一句，只讀伺服器：伺服器列出全部原因，畫面只顯示第一個（終止合作關店時伺服器列出 [已關閉, 停權]，只說已關閉那句）。
 * 核准的清單空（可以核准）時不說；核准的第一個原因是結構性的（申請不在審核中）時沒有入口、不說明。
 */
export function reviewBlockedLabel(
	application: KYCApplication | undefined,
): string | undefined {
	return application
		? firstVisibleLabel(application.approveUnavailabilities)
		: undefined;
}

// ---------------------------------------------------------------------------
// 欄位

/** 目錄認得的欄位（KYCField 的非 UNSPECIFIED 值）；新版伺服器才有的值不列。 */
function isKnownKycField(field: KYCField): boolean {
	return (
		field !== KYCField.KYC_FIELD_UNSPECIFIED && KYCField[field] !== undefined
	);
}

/** 欄位的顯示文字，取自共用文案目錄（賣家端同名）；UNSPECIFIED 或不認得的值是「未知」。 */
export function kycFieldLabel(field: KYCField): string {
	return staffTextOrUnknown(sellerKYCField(field));
}

/** 身分證字號對應的退回欄位：個人是賣家本人的，企業是負責人的。 */
export function idCardNoField(application: KYCApplication): KYCField {
	return application.subject.case === "business"
		? KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO
		: KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO;
}

/**
 * 文件類的欄位：契約的 KYCIssue.document_asset_id 只有這四個欄位能指到某一張文件；申請上的文件（KYCDocument.field、
 * KYCDocumentView.field）也用這幾個欄位表示，後台契約沒有另一套證件位置的列舉。這是畫面上唯一一張「欄位是不是文件」的表。
 */
const DOCUMENT_FIELDS: ReadonlySet<KYCField> = new Set([
	KYCField.KYC_FIELD_ID_CARD_FRONT,
	KYCField.KYC_FIELD_ID_CARD_BACK,
	KYCField.KYC_FIELD_BANK_BOOK,
	KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
]);

export function isDocumentField(field: KYCField): boolean {
	return DOCUMENT_FIELDS.has(field);
}

// ---------------------------------------------------------------------------
// 退回的逐項問題（2b-0 契約：RejectKYCRequest.issues）

/** 原因碼的呈現文字（封閉集合；UNSPECIFIED 不是有效值）。 */
export const KYC_ISSUE_CODE_LABEL: Record<
	Exclude<KYCIssueCode, KYCIssueCode.KYC_ISSUE_CODE_UNSPECIFIED>,
	string
> = {
	[KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE]: "看不清楚",
	[KYCIssueCode.KYC_ISSUE_CODE_MISMATCH]: "與其他資料不一致",
	[KYCIssueCode.KYC_ISSUE_CODE_EXPIRED]: "已過期",
	[KYCIssueCode.KYC_ISSUE_CODE_MISSING]: "缺少",
	[KYCIssueCode.KYC_ISSUE_CODE_INVALID]: "格式或內容不符規定",
	[KYCIssueCode.KYC_ISSUE_CODE_OTHER]: "其他",
};

/** 原因碼下拉的選項順序（不含 UNSPECIFIED）。 */
export const KYC_ISSUE_CODE_OPTIONS: readonly KYCIssueCode[] = [
	KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
	KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
	KYCIssueCode.KYC_ISSUE_CODE_EXPIRED,
	KYCIssueCode.KYC_ISSUE_CODE_MISSING,
	KYCIssueCode.KYC_ISSUE_CODE_INVALID,
	KYCIssueCode.KYC_ISSUE_CODE_OTHER,
];

/** 原因碼的顯示文字；UNSPECIFIED 或不認得的值是「未知」。 */
export function kycIssueCodeLabel(code: KYCIssueCode): string {
	return (
		(KYC_ISSUE_CODE_LABEL as Partial<Record<KYCIssueCode, string>>)[code] ??
		unknownText()
	);
}

/**
 * 文件類欄位的原因碼規則（伺服器 RejectKYC 驗證的同一條，proto 註解 KYCIssue.document_asset_id）：UNREADABLE、EXPIRED、MISMATCH、
 * INVALID 必須指定是哪一張文件；MISSING 不可以指定（缺的是整個位置）；OTHER 兩者皆可。
 */
export function issueAssetRequired(code: KYCIssueCode): boolean {
	return (
		code === KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE ||
		code === KYCIssueCode.KYC_ISSUE_CODE_EXPIRED ||
		code === KYCIssueCode.KYC_ISSUE_CODE_MISMATCH ||
		code === KYCIssueCode.KYC_ISSUE_CODE_INVALID
	);
}

export function issueAssetForbidden(code: KYCIssueCode): boolean {
	return code === KYCIssueCode.KYC_ISSUE_CODE_MISSING;
}

/** 實作者補：違反文件規則的說明。 */
export const ISSUE_ASSET_REQUIRED_TEXT = "這個原因要指定是哪一張文件";
export const ISSUE_ASSET_FORBIDDEN_TEXT =
	"缺少文件指的是整個位置，不能指定某一張文件。";

/** 問題說明的長度上限（契約：1 到 500 字）。 */
export const ISSUE_MESSAGE_MAX = 500;
/** 退回總理由與內部備註的長度上限（契約：1000 字）。 */
export const REJECT_TEXT_MAX = 1000;

/** 這份申請上可以指出問題的欄位；document 表示這是文件類的欄位（可以指到某一張）。 */
export interface IssueFieldOption {
	field: KYCField;
	label: string;
	document: boolean;
}

/**
 * 可以指出問題的欄位選項：由伺服器給（GetKYCApplication 的 issuable_fields，與 RejectKYC 驗證 KYCIssue.field 同一份規則），
 * 這裡只加上呈現文字；不自己寫「個人有哪些欄位、企業有哪些欄位」的表。UNSPECIFIED 與不認得的值不列。
 */
export function issuableFieldOptions(
	application: Pick<KYCApplication, "issuableFields">,
): IssueFieldOption[] {
	return application.issuableFields.filter(isKnownKycField).map((field) => ({
		field,
		label: kycFieldLabel(field),
		document: isDocumentField(field),
	}));
}

/** 顯示全文之後才有的、可以指定的一張文件（asset_id 來自 RevealKYCSensitive 的 KYCDocumentView）。 */
export interface DocumentChoice {
	field: KYCField;
	assetId: string;
	label: string;
}

/** 文件的顯示名稱：補充文件依順序編號，其他是欄位名稱。 */
function documentLabel(field: KYCField, position: number): string {
	return field === KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS
		? `${kycFieldLabel(field)} ${position + 1}`
		: kycFieldLabel(field);
}

/** 此刻有效的顯示全文裡、有 asset_id 的文件；沒有顯示全文（或已到期）時是空的：不能指定某一張。 */
export function documentChoices(
	reveal: KycReveal | undefined,
): DocumentChoice[] {
	if (!reveal) return [];
	const out: DocumentChoice[] = [];
	for (const d of reveal.documents) {
		if (!isDocumentField(d.field) || !d.assetId) continue;
		out.push({
			field: d.field,
			assetId: d.assetId,
			label: documentLabel(d.field, d.position),
		});
	}
	return out;
}

/** 審核員已加入的一項問題（還沒送出）。key 只用來在清單裡定位，不送出。 */
export interface KycIssueDraft {
	key: string;
	field: KYCField;
	/** 文件類欄位指定的那一張；整個位置（例如缺件）不指定時是空字串。 */
	documentAssetId: string;
	/** 指定的那一張文件的顯示名稱（加入當下記下，之後顯示全文到期也看得到）。 */
	documentLabel: string;
	code: KYCIssueCode;
	message: string;
}

/** 審核員正在填的內容：逐項問題、總理由與內部備註。送出失敗（含版本衝突）時整份保留，成功才清空。 */
export interface ReviewDraft {
	issues: readonly KycIssueDraft[];
	rejectReason: string;
	internalNote: string;
}

export const EMPTY_REVIEW_DRAFT: ReviewDraft = {
	issues: [],
	rejectReason: "",
	internalNote: "",
};

/**
 * 加入一項問題前的檢查：欄位要是這份申請可以指出問題的欄位；原因碼不可以是 UNSPECIFIED；說明去掉前後空白後 1 到 500 字；
 * 只有文件類欄位能指定文件，而且那一張要屬於這個欄位；同一個欄位（同一張文件）不重複。回傳錯誤文字，沒有錯誤回 undefined。
 */
export function validateIssueDraft(
	issue: Omit<KycIssueDraft, "key" | "documentLabel">,
	options: readonly IssueFieldOption[],
	choices: readonly DocumentChoice[],
	existing: readonly KycIssueDraft[],
): string | undefined {
	const option = options.find((o) => o.field === issue.field);
	if (!option) return "請選擇有問題的欄位。";
	if (!KYC_ISSUE_CODE_OPTIONS.includes(issue.code)) return "請選擇原因。";
	const message = trimGoSpace(issue.message);
	if (message.length === 0) return "請填寫給賣家看的說明。";
	if (goTrimmedRuneCount(message) > ISSUE_MESSAGE_MAX) {
		return `說明最多 ${ISSUE_MESSAGE_MAX} 字。`;
	}
	if (issue.documentAssetId) {
		if (!option.document) return "這個欄位不是文件，不能指定文件。";
		if (
			!choices.some(
				(c) => c.field === issue.field && c.assetId === issue.documentAssetId,
			)
		) {
			return "找不到指定的文件，請重新選擇。";
		}
	}
	// 文件類欄位依原因碼（與伺服器同一條規則）：問題出在某一張文件上的原因必須指定是哪一張，缺少則不可以指定。
	if (option.document) {
		if (issue.documentAssetId && issueAssetForbidden(issue.code)) {
			return ISSUE_ASSET_FORBIDDEN_TEXT;
		}
		if (!issue.documentAssetId && issueAssetRequired(issue.code)) {
			return choices.some((c) => c.field === issue.field)
				? ISSUE_ASSET_REQUIRED_TEXT
				: `${ISSUE_ASSET_REQUIRED_TEXT}；請先在頁面上「顯示全文」，才選得到文件。`;
		}
	}
	if (
		existing.some(
			(e) =>
				e.field === issue.field && e.documentAssetId === issue.documentAssetId,
		)
	) {
		return "這個欄位（文件）已經有一項問題，請先移除或修改它。";
	}
	return undefined;
}

/** 退回前的檢查：至少一項問題、總理由必填、總理由與內部備註各最多 1000 字。 */
export function validateRejectDraft(draft: ReviewDraft): string | undefined {
	if (draft.issues.length === 0)
		return "請至少指出一項問題，賣家才知道要修改什麼。";
	const reason = trimGoSpace(draft.rejectReason);
	if (reason.length === 0) return "請填寫退回原因，它會回傳給賣家。";
	if (goTrimmedRuneCount(reason) > REJECT_TEXT_MAX) {
		return `退回原因最多 ${REJECT_TEXT_MAX} 字。`;
	}
	if (goTrimmedRuneCount(trimGoSpace(draft.internalNote)) > REJECT_TEXT_MAX) {
		return `內部備註最多 ${REJECT_TEXT_MAX} 字。`;
	}
	return undefined;
}

/** RejectKYC 的請求：問題依加入的順序，說明與理由去掉前後空白，內部備註選填。 */
export function buildRejectRequest(
	application: Pick<KYCApplication, "id" | "etag">,
	draft: ReviewDraft,
) {
	return {
		id: application.id,
		etag: application.etag,
		reason: trimGoSpace(draft.rejectReason),
		issues: draft.issues.map((i) => ({
			field: i.field,
			documentAssetId: i.documentAssetId,
			code: i.code,
			message: trimGoSpace(i.message),
		})),
		internalNote: trimGoSpace(draft.internalNote),
	};
}

/** 一項問題在清單上的文字（審核員的草稿、賣家尚未處理的問題、審核紀錄共用）。 */
export function issueView(issue: {
	field: KYCField;
	code: KYCIssueCode;
	message: string;
	documentLabel?: string;
	hasDocument?: boolean;
}): { field: string; code: string; message: string; document?: string } {
	return {
		field: kycFieldLabel(issue.field),
		code: kycIssueCodeLabel(issue.code),
		message: issue.message,
		document:
			issue.documentLabel ?? (issue.hasDocument ? "指定的文件" : undefined),
	};
}

/** 伺服器回的問題（outstanding_issues、審核紀錄的 issues）轉成清單文字。 */
export function serverIssueView(issue: KYCIssue) {
	return issueView({
		field: issue.field,
		code: issue.code,
		message: issue.message,
		hasDocument: issue.documentAssetId !== "",
	});
}

/** 送出的結果對草稿的影響：成功才清空；任何失敗（含版本衝突）原封不動，整份草稿（問題、理由、備註）都不能丟。 */
export function draftAfter(
	draft: ReviewDraft,
	result: ActionResult,
): ReviewDraft {
	return result.ok ? EMPTY_REVIEW_DRAFT : draft;
}

// ---------------------------------------------------------------------------
// 審核紀錄

/** 審核紀錄的動作文字。 */
export function reviewActionLabel(action: KYCReviewAction): string {
	switch (action) {
		case KYCReviewAction.KYC_REVIEW_ACTION_SUBMIT:
			return "賣家送出";
		case KYCReviewAction.KYC_REVIEW_ACTION_APPROVE:
			return "核准";
		case KYCReviewAction.KYC_REVIEW_ACTION_REJECT:
			return "退回";
		case KYCReviewAction.KYC_REVIEW_ACTION_CANCEL:
			return "賣家取消";
		case KYCReviewAction.KYC_REVIEW_ACTION_REBIND:
			return "重新綁定 PayUni";
		default:
			return "-";
	}
}

/** 審核者：只顯示 admin ID（K16）。賣家自己的動作沒有審核者。 */
export function reviewerLabel(review: KYCReview): string {
	return review.reviewerAdminId
		? `管理員 ${review.reviewerAdminId}`
		: review.action === KYCReviewAction.KYC_REVIEW_ACTION_SUBMIT ||
				review.action === KYCReviewAction.KYC_REVIEW_ACTION_CANCEL
			? "賣家"
			: "-";
}

/** 最近一次退回（reviews 是舊到新）；沒有時 undefined。 */
export function lastRejection(
	reviews: readonly KYCReview[],
): KYCReview | undefined {
	for (let i = reviews.length - 1; i >= 0; i--) {
		if (reviews[i].action === KYCReviewAction.KYC_REVIEW_ACTION_REJECT) {
			return reviews[i];
		}
	}
	return undefined;
}

// ---------------------------------------------------------------------------
// 顯示全文（K15、D4 第 5 點）

/** 一次顯示全文的結果（只存在瀏覽器記憶體，不寫進任何儲存）。 */
export interface KycReveal {
	applicationId: string;
	idCardNo: string;
	repIdCardNo: string;
	bankAccount: string;
	/** 授權到期的時間（伺服器給的）；到期後要收回成遮蔽值。 */
	expireTime: Date;
	documents: readonly KYCDocumentView[];
}

/** 回應沒有到期時間時不當成可顯示（沒有期限的明文不存在）：回 undefined。 */
export function revealFrom(
	applicationId: string,
	res: RevealKYCSensitiveResponse,
): KycReveal | undefined {
	if (!applicationId || !res.expireTime) return undefined;
	return {
		applicationId,
		idCardNo: res.idCardNo,
		repIdCardNo: res.repIdCardNo,
		bankAccount: res.bankAccount,
		expireTime: timestampDate(res.expireTime),
		documents: res.documents,
	};
}

/** 此刻還有效的顯示全文：申請不同、或到期（含剛好到期的那一刻）都是 undefined。 */
export function activeReveal(
	reveal: KycReveal | null | undefined,
	applicationId: string | undefined,
	now: Date,
): KycReveal | undefined {
	if (!reveal || !applicationId || reveal.applicationId !== applicationId) {
		return undefined;
	}
	return now.getTime() < reveal.expireTime.getTime() ? reveal : undefined;
}

/** 距離到期還有幾毫秒；已到期是 0。 */
export function revealRemainingMs(reveal: KycReveal, now: Date): number {
	return Math.max(0, reveal.expireTime.getTime() - now.getTime());
}

/** 「全文顯示到 HH:mm」（台北時間）。 */
export function revealUntilText(reveal: KycReveal): string {
	return `全文顯示到 ${formatDateTimeShort(reveal.expireTime).slice(-5)}`;
}

/**
 * 敏感欄位的顯示值：顯示全文且有明文時用明文，否則用遮蔽值；兩者都沒有回 `-`。
 * 明文是空字串（這個型態沒有該欄位）時退回遮蔽值，不顯示空白。
 */
export function sensitiveValue(
	masked: string,
	full: string | undefined,
	revealed: boolean,
): string {
	if (revealed && full) return full;
	return masked || "-";
}

/** 這個位置的檢視網址（只在顯示全文之後有）。 */
export function documentViewOf(
	reveal: KycReveal | undefined,
	field: KYCField,
	position: number,
): KYCDocumentView | undefined {
	return reveal?.documents.find(
		(d) => d.field === field && d.position === position,
	);
}

const ADMIN_FILES_PREFIX = "/api/files/admin/";

/**
 * 證件影像的瀏覽器網址：只收伺服器給的相對路徑（`/api/files/admin/…`），經同源 `/api/bff` 讀取；
 * 絕對網址、協定相對網址、帶 `..` 的路徑一律不收（回 undefined），不直連後端。
 */
export function documentSrc(viewUri: string): string | undefined {
	if (!viewUri.startsWith(ADMIN_FILES_PREFIX)) return undefined;
	if (viewUri.includes("//") || viewUri.includes("..")) return undefined;
	if (/[\s\\?#]/u.test(viewUri)) return undefined;
	return `/api/bff${viewUri}`;
}

/** 申請上某個文件欄位的文件（只有順序，從 0）。 */
export function documentsOf(
	application: KYCApplication,
	field: KYCField,
): number[] {
	return application.documents
		.filter((d) => d.field === field)
		.map((d) => d.position)
		.sort((a, b) => a - b);
}

// ---------------------------------------------------------------------------
// 申請內容的文字

export function sellerTypeLabel(application: KYCApplication): string {
	switch (application.subject.case) {
		case "business":
			return "企業賣家";
		case "individual":
			return "個人賣家";
		default:
			return "-";
	}
}

/** 日期（google.type.Date）→ `YYYY-MM-DD`；沒有回 `-`。 */
export function dateText(
	date: { year: number; month: number; day: number } | undefined,
): string {
	return formatCalendarDate(date) || "-";
}

// ---------------------------------------------------------------------------
// 商業登記（GCIS）並列（B4、D4 第 4 點）

export type GcisMark = "mismatch" | "unverifiable";

export interface GcisRow {
	key: string;
	label: string;
	/** 申請填的值。 */
	applicant: string;
	/** 登記資料（來源）的值；沒有查到時 `-`。 */
	registry: string;
	/** 不一致或無法比對；一致或沒有比對的列沒有標記。 */
	mark?: GcisMark;
}

export type GcisHeaderKind =
	| "pending"
	| "overdue"
	| "none"
	| "found"
	| "notFound"
	| "unavailable";

export interface GcisView {
	header: {
		kind: GcisHeaderKind;
		text: string;
		/** 查詢中：「最晚 HH:mm 會有結果」；超過期限：請重新查詢。 */
		detail?: string;
		/** 查詢中、仍顯示上一筆已完成的查詢時，上一筆的摘要。 */
		previous?: string;
	};
	rows: GcisRow[];
	/** 登記現況不是核准設立（或查詢當時在停業期間）時的警示文字；沒有警示是 undefined。 */
	warning?: string;
}

function outcomeText(check: KYCRegistrationCheck): string | undefined {
	switch (check.outcome) {
		case KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_FOUND:
			return "查到";
		case KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_NOT_FOUND:
			return GCIS_NOT_FOUND_TEXT;
		case KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_UNAVAILABLE:
			return GCIS_UNAVAILABLE_TEXT;
		default:
			return undefined;
	}
}

/** `查詢時間 {時間}・{結果}`；結果不認得（UNSPECIFIED）或沒有查詢時間時沒有這句。 */
function checkSummary(check: KYCRegistrationCheck): string | undefined {
	const result = outcomeText(check);
	if (!result || !check.queryTime) return undefined;
	return `查詢時間 ${formatDateTimeShort(timestampDate(check.queryTime))}・${result}`;
}

/** 進行中的查詢最晚什麼時候有結果（伺服器保證的期限，台北時間 HH:mm）。 */
export function pendingDeadlineText(
	pending: KYCRegistrationCheck,
): string | undefined {
	if (!pending.deadlineTime) return undefined;
	return `最晚 ${formatDateTimeShort(timestampDate(pending.deadlineTime)).slice(-5)} 會有結果`;
}

function headerOf(application: KYCApplication, now: Date): GcisView["header"] {
	const check = application.registrationCheck;
	const summary = check ? checkSummary(check) : undefined;
	const pending = application.pendingRegistrationCheck;
	if (pending) {
		if (registrationOverdue(pending, now)) {
			return {
				kind: "overdue",
				text: GCIS_OVERDUE_TEXT,
				detail: GCIS_OVERDUE_DETAIL,
				previous: summary,
			};
		}
		return {
			kind: "pending",
			text: GCIS_PENDING_TEXT,
			detail: pendingDeadlineText(pending),
			previous: summary,
		};
	}
	if (!check || !summary) return { kind: "none", text: GCIS_NONE_TEXT };
	switch (check.outcome) {
		case KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_FOUND:
			return { kind: "found", text: summary };
		case KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_NOT_FOUND:
			return { kind: "notFound", text: summary };
		default:
			return { kind: "unavailable", text: summary };
	}
}

function registrationStatusLabel(status: KYCRegistrationStatus): string {
	switch (status) {
		case KYCRegistrationStatus.KYC_REGISTRATION_STATUS_ACTIVE:
			return "核准設立";
		case KYCRegistrationStatus.KYC_REGISTRATION_STATUS_ORDERED_TO_DISSOLVE:
			return "命令解散";
		case KYCRegistrationStatus.KYC_REGISTRATION_STATUS_TERMINATED:
			return "已終止";
		case KYCRegistrationStatus.KYC_REGISTRATION_STATUS_UNKNOWN:
			return "現況不明";
		default:
			return "-";
	}
}

/** 停業期間（`YYYY-MM-DD`）；起迄都沒有時 undefined。 */
function suspendPeriod(
	registration: KYCRegistration,
): { start: string; end: string } | undefined {
	const start = formatCalendarDate(registration.suspendStartDate);
	const end = formatCalendarDate(registration.suspendEndDate);
	return start || end ? { start, end } : undefined;
}

function periodText(period: { start: string; end: string }): string {
	if (period.start && period.end) return `${period.start} 至 ${period.end}`;
	return period.start ? `${period.start} 起` : `至 ${period.end}`;
}

/**
 * 登記現況的警示：現況不是核准設立，或查詢當天落在停業期間內時顯示「{現況原文}（停業期間 …）」；
 * 核准設立而且沒有在停業的不顯示。沒有查到登記資料（沒有現況）時 undefined。
 * 「查詢當天是否在停業期間」由伺服器算好（KYCRegistration.suspended_at_query），這裡只讀，不自己比對日期。
 */
export function registrationWarning(
	check: KYCRegistrationCheck | undefined,
): string | undefined {
	if (
		check?.outcome !== KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_FOUND ||
		!check.registration
	) {
		return undefined;
	}
	const registration = check.registration;
	if (
		registration.status ===
			KYCRegistrationStatus.KYC_REGISTRATION_STATUS_ACTIVE &&
		!registration.suspendedAtQuery
	) {
		return undefined;
	}
	const period = suspendPeriod(registration);
	const base =
		registration.statusText || registrationStatusLabel(registration.status);
	return period ? `${base}（停業期間 ${periodText(period)}）` : base;
}

function markOf(
	check: KYCRegistrationCheck | undefined,
	field: KYCField,
): GcisMark | undefined {
	if (
		check?.outcome !== KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_FOUND
	) {
		return undefined;
	}
	if (check.mismatchedFields.includes(field)) return "mismatch";
	if (check.unverifiableFields.includes(field)) return "unverifiable";
	return undefined;
}

/**
 * 公司型申請的商業登記並列：申請的值與登記資料逐列對照，不一致與無法比對各有標記。個人型申請沒有（undefined）。
 * 只有查到（FOUND）的查詢才有登記值與標記；沒有查到或查詢中時登記欄是 `-`。
 */
export function gcisView(
	application: KYCApplication,
	now: Date,
): GcisView | undefined {
	if (application.subject.case !== "business") return undefined;
	const business = application.subject.value;
	const check = application.registrationCheck;
	const registration =
		check?.outcome === KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_FOUND
			? check.registration
			: undefined;
	const registry = (value: string | undefined) => value || "-";
	const rows: GcisRow[] = [
		{
			key: "company_name",
			label: "公司名稱",
			applicant: business.companyName || "-",
			registry: registry(registration?.name),
			mark: markOf(check, KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME),
		},
		{
			key: "tax_id",
			label: "統一編號",
			applicant: business.taxId || "-",
			registry: registry(registration?.taxId),
			mark: markOf(check, KYCField.KYC_FIELD_BUSINESS_TAX_ID),
		},
		{
			key: "company_address",
			label: "公司地址",
			applicant: addressText(business.companyAddress),
			registry: registry(registration?.address),
			mark: markOf(check, KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS),
		},
		{
			key: "company_register_date",
			label: "設立日期",
			applicant: dateText(business.companyRegisterDate),
			registry: dateText(registration?.setupDate),
			mark: markOf(check, KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE),
		},
		{
			// K8：組織型態不是申請的欄位，只顯示登記資料的。
			key: "organization_type",
			label: "組織型態",
			applicant: "-",
			registry: registry(registration?.organizationType),
		},
		{
			key: "registration_status",
			label: "登記現況",
			applicant: "-",
			registry: registry(
				registration
					? registration.statusText ||
							registrationStatusLabel(registration.status)
					: undefined,
			),
		},
	];
	return {
		header: headerOf(application, now),
		rows,
		warning: registrationWarning(check),
	};
}

export function gcisMarkLabel(mark: GcisMark): string {
	return mark === "mismatch" ? "不一致" : "無法比對";
}

// ---------------------------------------------------------------------------
// PayUni 註冊（K9、B18、D4 第 6 點）

/** PayUni 註冊的讀取狀態；notFound 是店家還沒有核准過申請（還沒有註冊）。 */
export type PayUniState =
	| { state: "loading" }
	| { state: "error" }
	| { state: "notFound" }
	| { state: "ready"; registration: PayUniRegistration };

export function payUniOf(result: {
	error: unknown;
	data: { registration?: PayUniRegistration } | undefined;
}): PayUniState {
	if (result.error != null) {
		return result.error instanceof ConnectError &&
			result.error.code === Code.NotFound
			? { state: "notFound" }
			: { state: "error" };
	}
	if (result.data === undefined) return { state: "loading" };
	if (!result.data.registration) return { state: "error" };
	return { state: "ready", registration: result.data.registration };
}

/**
 * PayUni 卡上重新綁定的呈現，只讀伺服器（`can_rebind`、`rebind_unavailabilities`）：能做時顯示「重新綁定」；不能做時第一個原因是結構性的
 * （綁定中、已綁定）就什麼都不說，否則顯示第一個原因的伺服器句子（終止、店家已關閉、等待刪除、申請不是已核准）。
 */
export function rebindPresentation(registration: PayUniRegistration): {
	canRebind: boolean;
	reason: string | undefined;
} {
	if (registration.canRebind) return { canRebind: true, reason: undefined };
	return {
		canRebind: false,
		reason: firstVisibleLabel(registration.rebindUnavailabilities),
	};
}

/** 問題要補的申請欄位文字（沒有時空陣列）。 */
export function payUniProblemFields(
	registration: PayUniRegistration,
): string[] {
	return (registration.problem?.fields ?? [])
		.filter((f) => f !== KYCField.KYC_FIELD_UNSPECIFIED)
		.map(kycFieldLabel);
}

// ---------------------------------------------------------------------------
// 評分（公開的 StoreStats.rating_average）

/**
 * 評分統計卡的值：只有讀到而且大於 0 的平均分才顯示（一位小數）；讀不到、還沒有評價、已關閉或未公開的店都是 undefined，
 * 那張卡不渲染（算不出來的不編一個數字）。
 */
export function ratingText(
	stats: { ratingAverage?: number } | undefined,
): string | undefined {
	const rating = stats?.ratingAverage;
	if (rating === undefined || !Number.isFinite(rating) || rating <= 0) {
		return undefined;
	}
	return rating.toFixed(1);
}

/**
 * 重新查詢成功後，把回應的那筆進行中的查詢寫進 GetKYCApplication 的快取（不整筆重讀申請）：只換 pending_registration_check，
 * 其餘（含 etag、審核紀錄）照舊；快取還沒有資料時原樣回傳（交給正常的讀取）。輪詢隨後以伺服器的期限接手。
 */
export function applyRequeryResult<T extends { application?: KYCApplication }>(
	cached: T | undefined,
	check: KYCRegistrationCheck | undefined,
): T | undefined {
	if (!cached?.application || !check) return cached;
	return {
		...cached,
		application: { ...cached.application, pendingRegistrationCheck: check },
	};
}

/**
 * GetKYCApplication 的 query options（connect-query 的 createQueryOptions）。讀（useKycApplication）直接用這份 options，
 * 寫（writeRequeryResult）用同一份的 queryKey，所以寫入一定落在畫面讀的那筆快取上。申請 ID 為空時不發請求。
 */
export function kycApplicationQueryOptions(
	transport: Transport,
	applicationId: string | undefined,
) {
	return createQueryOptions(
		SellerAdminService.method.getKYCApplication,
		applicationId ? { id: applicationId } : skipToken,
		{ transport },
	);
}

export function kycApplicationQueryKey(
	transport: Transport,
	applicationId: string,
) {
	return kycApplicationQueryOptions(transport, applicationId).queryKey;
}

/** 重新查詢成功後，把回應的那筆查詢寫進該申請在 useQuery 的快取（applyRequeryResult）。 */
export function writeRequeryResult(
	queryClient: QueryClient,
	transport: Transport,
	applicationId: string,
	check: KYCRegistrationCheck | undefined,
): void {
	queryClient.setQueryData(
		kycApplicationQueryKey(transport, applicationId),
		(cached: GetKYCApplicationResponse | undefined) =>
			applyRequeryResult(cached, check),
	);
}
