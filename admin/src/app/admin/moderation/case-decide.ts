import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	type Case,
	CaseAction,
	CaseDecision,
	CaseState,
	type DecideCaseRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { VIOLATION_SEVERITY_OPTIONS } from "@/app/admin/merchants/[id]/violation-measure";
import { actionsFor, categoriesFor } from "./moderation-text";

export const REASON_MAX_LENGTH = 500;

export const DECIDE_FAILED_TEXT =
	"決定失敗，案件沒有變動；請確認網路狀態後再試一次。";
export const CASE_CONFLICT_TEXT =
	"案件剛被更新（有新的檢舉或已被處理），頁面已重新載入，請確認後再決定。";

/** 只有進行中的案件可以決定；結案是終態，沒有重開。 */
export function canDecide(kase: Case): boolean {
	return kase.state === CaseState.OPEN;
}

export const TAKEDOWN_NO_STORE_TEXT =
	"案件沒有內容快照的店家（快照已過保留期或內容不是店家寫的），無法建立下架措施。";

export interface DecideForm {
	decision: CaseDecision;
	action: CaseAction;
	category: PolicyCategory;
	reason: string;
	severity: ViolationSeverity;
}

export const EMPTY_DECIDE_FORM: DecideForm = {
	decision: CaseDecision.UNSPECIFIED,
	action: CaseAction.UNSPECIFIED,
	category: 0,
	reason: "",
	severity: ViolationSeverity.UNSPECIFIED,
};

/** 送出前的檢查，與伺服器的規則一致（DecideCaseRequest）；回傳第一個錯誤，沒有錯誤回 undefined。 */
export function validateDecide(
	kase: Case,
	form: DecideForm,
): string | undefined {
	if (form.decision === CaseDecision.NO_VIOLATION) return undefined;
	if (form.decision !== CaseDecision.VIOLATION) return "請選擇決定。";
	const kind = kase.target?.kind ?? 0;
	if (!actionsFor(kind).includes(form.action)) return "請選擇處置。";
	if (!categoriesFor(form.action).includes(form.category)) {
		return "請選擇類別。";
	}
	const reason = trimGoSpace(form.reason);
	if (reason.length === 0) return "請填寫給作者的理由。";
	if (goTrimmedRuneCount(reason) > REASON_MAX_LENGTH) {
		return `給作者的理由最多 ${REASON_MAX_LENGTH} 字。`;
	}
	if (form.action === CaseAction.TAKEDOWN) {
		if (!kase.storeId) return TAKEDOWN_NO_STORE_TEXT;
		if (!VIOLATION_SEVERITY_OPTIONS.includes(form.severity))
			return "請選擇嚴重度。";
	}
	return undefined;
}

/** DecideCase 的請求：帶讀到的案件的 etag；不違規不帶其他欄位，下架才帶嚴重度（店家由伺服器取自案件快照）。呼叫前須先通過 validateDecide。 */
export function buildDecide(
	kase: Case,
	form: DecideForm,
): MessageInitShape<typeof DecideCaseRequestSchema> {
	const base = { id: kase.id, etag: kase.etag, decision: form.decision };
	if (form.decision !== CaseDecision.VIOLATION) return base;
	const violation = {
		...base,
		category: form.category,
		action: form.action,
		reason: trimGoSpace(form.reason),
	};
	return form.action === CaseAction.TAKEDOWN
		? { ...violation, severity: form.severity }
		: violation;
}
