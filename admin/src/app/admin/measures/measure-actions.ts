import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	AdminRevokeReason,
	type CreateMeasureNoteRequestSchema,
	type DecideAppealRequestSchema,
	type DecideRemediationRequestSchema,
	type RevokeMeasureRequestSchema,
	type ScheduleMeasureReviewRequestSchema,
	type UpdateMeasureRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import {
	type Evidence,
	type Measure,
	type MeasureActionUnavailability,
	MeasureSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { MeasureRequestState } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { unavailableLabels } from "@/lib/unavailability";
import { documentSrc } from "../merchants/[id]/kyc-application";
import {
	fromLocalInput,
	NOTE_MAX_LENGTH,
	reviewByError,
	STATEMENT_MAX_LENGTH,
	toLocalInput,
} from "../merchants/[id]/measure";

// 措施詳情（/admin/measures/[id]）的純邏輯：哪些動作顯示、停用與原因文字、各對話框的驗證與請求組裝。
// 前提（Will 2026-10-15）：能不能做與為什麼不能，都來自伺服器（Measure.can_* 與 *_unavailabilities，每個原因帶伺服器的文字），client 只呈現。
// 時間一律由呼叫端傳入（now）。

// ---------------------------------------------------------------------------
// 操作列

export interface ActionState {
	/** 隱藏的動作（伺服器說不能做且不需要說明）不渲染。 */
	visible: boolean;
	enabled: boolean;
	/** 停用時並排顯示的原因；可以做時是空陣列。 */
	reasons: string[];
}

function hidden(): ActionState {
	return { visible: false, enabled: false, reasons: [] };
}

function shown(
	can: boolean,
	...lists: ReadonlyArray<readonly MeasureActionUnavailability[]>
): ActionState {
	return can
		? { visible: true, enabled: true, reasons: [] }
		: { visible: true, enabled: false, reasons: unavailableLabels(...lists) };
}

function visibleWhen(can: boolean): ActionState {
	return can ? { visible: true, enabled: true, reasons: [] } : hidden();
}

export interface ActionBar {
	edit: ActionState;
	end: ActionState;
	revoke: ActionState;
	scheduleReview: ActionState;
	decideAppeal: ActionState;
	decideRemediation: ActionState;
}

/**
 * 操作列：修改與撤銷在不能做時停用並列出伺服器的原因（不隱藏）；結束、延後複審與兩種裁定的入口本身由狀態決定，不能做時隱藏。
 * 修改只要改理由、延長、縮短其中一個能做就可以開啟。
 */
export function actionBar(measure: Measure): ActionBar {
	const canEdit =
		measure.canUpdateStatement || measure.canExtend || measure.canShorten;
	return {
		edit: shown(
			canEdit,
			measure.updateStatementUnavailabilities,
			measure.extendUnavailabilities,
			measure.shortenUnavailabilities,
		),
		end: visibleWhen(measure.canEnd),
		revoke: shown(measure.canRevoke, measure.revokeUnavailabilities),
		scheduleReview: visibleWhen(measure.canScheduleReview),
		decideAppeal: visibleWhen(measure.canDecideAppeal),
		decideRemediation: visibleWhen(measure.canDecideRemediation),
	};
}

// ---------------------------------------------------------------------------
// 修改措施（UpdateMeasure）

export interface UpdateForm {
	/** 給賣家的理由。 */
	statement: string;
	/** 結束時間，`datetime-local` 的值（台北時間）；空字串＝沒填。 */
	endTime: string;
}

export function initialUpdateForm(
	measure: Pick<Measure, "statement" | "endTime">,
): UpdateForm {
	return {
		statement: measure.statement,
		endTime: measure.endTime
			? toLocalInput(timestampDate(measure.endTime))
			: "",
	};
}

/** 理由欄不能編輯時，旁邊顯示的原因；可以編輯時是空陣列。 */
export function statementLockReasons(measure: Measure): string[] {
	return measure.canUpdateStatement
		? []
		: unavailableLabels(measure.updateStatementUnavailabilities);
}

/**
 * 結束時間欄的限制與說明。延長與縮短都可以：不設上限，下限是現在；只能縮短：上限是目前的結束時間、下限是現在；
 * 只能延長：下限是目前的結束時間；都不行：停用。限制之外的方向，旁邊顯示伺服器給的原因。
 */
export function endTimeField(
	measure: Measure,
	now: Date,
): { disabled: boolean; min?: string; max?: string; reasons: string[] } {
	const current = measure.endTime
		? toLocalInput(timestampDate(measure.endTime))
		: undefined;
	const nowInput = toLocalInput(now);
	if (measure.canExtend && measure.canShorten) {
		return { disabled: false, min: nowInput, reasons: [] };
	}
	if (measure.canShorten) {
		return {
			disabled: false,
			min: nowInput,
			max: current,
			reasons: unavailableLabels(measure.extendUnavailabilities),
		};
	}
	if (measure.canExtend) {
		return {
			disabled: false,
			min: current ?? nowInput,
			reasons: unavailableLabels(measure.shortenUnavailabilities),
		};
	}
	return {
		disabled: true,
		reasons: unavailableLabels(
			measure.extendUnavailabilities,
			measure.shortenUnavailabilities,
		),
	};
}

export type UpdatePath = "statement" | "end_time";

/**
 * 管理員自己改過的欄位：表單的值與開啟（或上次重讀時尚未動過）的基準不同。版本衝突重讀之後，沒動過的欄位不算改，
 * 不會因為它們與新版本不同就進 update_mask 而蓋掉別人的修改。改回基準的值不算改。
 */
export function editedFields(
	baseline: UpdateForm,
	form: UpdateForm,
): UpdatePath[] {
	const edited: UpdatePath[] = [];
	if (trimGoSpace(form.statement) !== trimGoSpace(baseline.statement)) {
		edited.push("statement");
	}
	if (form.endTime !== baseline.endTime) edited.push("end_time");
	return edited;
}

/**
 * 重讀措施（版本衝突）之後把基準與表單對到新的措施：沒動過的欄位換成新措施的值，動過的欄位保留管理員的輸入與原本的基準；
 * 但新措施已經不許改的欄位（理由：can_update_statement 為 false；結束時間：can_extend 與 can_shorten 都是 false），
 * 欄位會停用，管理員改過的輸入也還原成新措施的值，不留一個看得到、卻不會送出的修改。
 */
export function rebaseForm(
	measure: Pick<
		Measure,
		"statement" | "endTime" | "canUpdateStatement" | "canExtend" | "canShorten"
	>,
	baseline: UpdateForm,
	form: UpdateForm,
): { baseline: UpdateForm; form: UpdateForm } {
	const fresh = initialUpdateForm(measure);
	const edited = editedFields(baseline, form);
	const next = { baseline: { ...baseline }, form: { ...form } };
	if (!edited.includes("statement") || !measure.canUpdateStatement) {
		next.baseline.statement = fresh.statement;
		next.form.statement = fresh.statement;
	}
	if (
		!edited.includes("end_time") ||
		!(measure.canExtend || measure.canShorten)
	) {
		next.baseline.endTime = fresh.endTime;
		next.form.endTime = fresh.endTime;
	}
	return next;
}

/**
 * 實際要改的欄位（update_mask 的路徑）：管理員自己改過（editedFields）、而且伺服器此刻允許改的欄位。不能編輯的欄位不算；
 * 結束時間清空不算改（不支援清除，伺服器的 VIOLATION 依據也不許）；與目前相同的值（到分鐘）不算改。
 */
export function updatePaths(
	measure: Measure,
	form: UpdateForm,
	baseline: UpdateForm,
): UpdatePath[] {
	const edited = editedFields(baseline, form);
	const paths: UpdatePath[] = [];
	if (
		edited.includes("statement") &&
		measure.canUpdateStatement &&
		trimGoSpace(form.statement) !== measure.statement
	) {
		paths.push("statement");
	}
	if (
		edited.includes("end_time") &&
		(measure.canExtend || measure.canShorten) &&
		form.endTime !== "" &&
		form.endTime !==
			(measure.endTime ? toLocalInput(timestampDate(measure.endTime)) : "")
	) {
		paths.push("end_time");
	}
	return paths;
}

/** 送出前的檢查，與伺服器的規則一致（UpdateMeasure）；只檢查管理員實際改過的欄位。回傳第一個錯誤，沒有錯誤回 undefined。 */
export function validateUpdate(
	measure: Measure,
	form: UpdateForm,
	baseline: UpdateForm,
	now: Date,
): string | undefined {
	const paths = updatePaths(measure, form, baseline);
	if (paths.includes("statement")) {
		const statement = trimGoSpace(form.statement);
		if (statement.length === 0) return "請填寫給賣家的理由。";
		if (goTrimmedRuneCount(statement) > STATEMENT_MAX_LENGTH) {
			return `給賣家的理由最多 ${STATEMENT_MAX_LENGTH} 字。`;
		}
	}
	if (paths.includes("end_time")) {
		const end = fromLocalInput(form.endTime);
		if (!end) return "結束時間的格式不正確。";
		if (end.getTime() <= now.getTime()) return "結束時間必須晚於現在。";
		if (measure.endTime) {
			const current = toLocalInput(timestampDate(measure.endTime));
			const later = form.endTime > current;
			if (later && !measure.canExtend) {
				return "結束時間不能晚於目前的結束時間。";
			}
			if (!later && !measure.canShorten) {
				return "結束時間不能早於目前的結束時間。";
			}
		}
	}
	return undefined;
}

/** UpdateMeasure 的請求：update_mask 只列改過的欄位，measure 只帶那些欄位加 id 與 etag。呼叫前須先通過 validateUpdate 且至少改了一個欄位。 */
export function buildUpdateMeasure(
	measure: Measure,
	form: UpdateForm,
	baseline: UpdateForm,
): MessageInitShape<typeof UpdateMeasureRequestSchema> {
	const paths = updatePaths(measure, form, baseline);
	if (paths.length === 0) throw new Error("沒有改任何欄位");
	const next: MessageInitShape<typeof MeasureSchema> = {
		id: measure.id,
		etag: measure.etag,
	};
	if (paths.includes("statement")) next.statement = trimGoSpace(form.statement);
	if (paths.includes("end_time")) {
		const end = fromLocalInput(form.endTime);
		if (!end) throw new Error("結束時間不是有效的時間");
		next.endTime = timestampFromDate(end);
	}
	return { measure: next, updateMask: fieldMask(MeasureSchema, paths) };
}

// ---------------------------------------------------------------------------
// 裁定（DecideAppeal、DecideRemediation）

export type DecideKind = "appeal" | "remediation";
export const DECISION_REASON_MAX_LENGTH = 2000;

export function decideTitle(kind: DecideKind): string {
	return kind === "appeal" ? "裁定申覆" : "裁定解除請求";
}

export function decideAcceptLabel(kind: DecideKind): string {
	return kind === "appeal" ? "接受（撤銷這個措施）" : "接受（結束這個措施）";
}

export const DECIDE_REJECT_LABEL = "駁回";
export const DECIDE_REASON_LABEL = "裁定理由（賣家看得到）";
export const DECIDE_REVOKE_FINDINGS_LABEL =
	"一併撤銷這個措施建立的認定（引用它們的其他措施也會撤銷）";
export const REMEDIATION_REJECT_HINT =
	"駁回會計入冷卻，賣家可能要等一段時間才能再請求";

export interface DecideForm {
	/** true 接受、false 駁回、null 還沒選。 */
	accept: boolean | null;
	reason: string;
	revokeFindings: boolean;
}

/** 裁定理由的檢查（接受與駁回都必填，賣家看得到，1–2000 字）。沒有錯誤回 undefined；還沒選結果由送出鍵停用處理。 */
export function validateDecision(form: DecideForm): string | undefined {
	const reason = trimGoSpace(form.reason);
	if (reason.length === 0) return "請填寫裁定理由。";
	if (goTrimmedRuneCount(reason) > DECISION_REASON_MAX_LENGTH) {
		return `裁定理由最多 ${DECISION_REASON_MAX_LENGTH} 字。`;
	}
	return undefined;
}

export function canSubmitDecision(form: DecideForm): boolean {
	return form.accept !== null && validateDecision(form) === undefined;
}

function decisionOf(form: DecideForm): MeasureRequestState {
	if (form.accept === null) throw new Error("還沒選裁定結果");
	return form.accept
		? MeasureRequestState.ACCEPTED
		: MeasureRequestState.REJECTED;
}

/** 一併撤銷認定只在接受申覆、而且伺服器說可以（can_revoke_findings）時送；駁回時送 true 伺服器會拒絕。 */
export function buildDecideAppeal(
	measure: Measure,
	form: DecideForm,
): MessageInitShape<typeof DecideAppealRequestSchema> {
	return {
		measureId: measure.id,
		decision: decisionOf(form),
		reason: trimGoSpace(form.reason),
		revokeFindings:
			form.accept === true && form.revokeFindings && measure.canRevokeFindings,
		etag: measure.etag,
	};
}

export function buildDecideRemediation(
	measure: Measure,
	form: DecideForm,
): MessageInitShape<typeof DecideRemediationRequestSchema> {
	return {
		measureId: measure.id,
		decision: decisionOf(form),
		reason: trimGoSpace(form.reason),
		etag: measure.etag,
	};
}

// ---------------------------------------------------------------------------
// 撤銷（RevokeMeasure）

/** 撤銷可以選的原因：契約的 AdminRevokeReason 本來就只有管理員能選的值（系統寫入的原因不在裡面），這裡只去掉 UNSPECIFIED。順序同 enum。 */
export type AdminRevokeOption = Exclude<
	AdminRevokeReason,
	AdminRevokeReason.UNSPECIFIED
>;

export const REVOKE_REASON_OPTIONS: readonly AdminRevokeOption[] =
	Object.values(AdminRevokeReason).filter(
		(value): value is AdminRevokeOption =>
			typeof value === "number" && value !== AdminRevokeReason.UNSPECIFIED,
	);

/** 是不是可選的撤銷原因（UNSPECIFIED 與不認得的值都不是）。 */
export function isRevokeOption(
	reason: AdminRevokeReason,
): reason is AdminRevokeOption {
	return REVOKE_REASON_OPTIONS.some((option) => option === reason);
}

export function buildRevoke(
	measure: Measure,
	reason: AdminRevokeReason,
	revokeFindings: boolean,
): MessageInitShape<typeof RevokeMeasureRequestSchema> {
	if (!isRevokeOption(reason)) {
		throw new Error("撤銷原因不是可選的值");
	}
	return {
		id: measure.id,
		reason,
		revokeFindings: revokeFindings && measure.canRevokeFindings,
		etag: measure.etag,
	};
}

// ---------------------------------------------------------------------------
// 延後複審（ScheduleMeasureReview）

export interface ScheduleForm {
	/** 新的複審期限，`datetime-local` 的值（台北時間）。 */
	reviewBy: string;
	/** 為什麼延後（內部註記）。 */
	note: string;
}

/** 內部註記的檢查（延後複審、新增註記共用）：1–2000 字。沒有錯誤回 undefined。 */
export function noteError(value: string): string | undefined {
	const note = trimGoSpace(value);
	if (note.length === 0) return "請填寫內部註記。";
	if (goTrimmedRuneCount(note) > NOTE_MAX_LENGTH) {
		return `內部註記最多 ${NOTE_MAX_LENGTH} 字。`;
	}
	return undefined;
}

export function validateSchedule(
	form: ScheduleForm,
	now: Date,
): string | undefined {
	return reviewByError(form.reviewBy, now) ?? noteError(form.note);
}

export function buildScheduleReview(
	measure: Measure,
	form: ScheduleForm,
): MessageInitShape<typeof ScheduleMeasureReviewRequestSchema> {
	const review = fromLocalInput(form.reviewBy);
	if (!review) throw new Error("複審期限不是有效的時間");
	return {
		id: measure.id,
		reviewBy: timestampFromDate(review),
		note: trimGoSpace(form.note),
		etag: measure.etag,
	};
}

// ---------------------------------------------------------------------------
// 內部註記

export function buildCreateNote(
	measureId: string,
	body: string,
): MessageInitShape<typeof CreateMeasureNoteRequestSchema> {
	return { measureId, note: { body: trimGoSpace(body) } };
}

// ---------------------------------------------------------------------------
// 待處理請求：提出時的版本與佐證

/** 「賣家提出時是第 {r} 版，目前是第 {c} 版」。 */
export function submittedVersionText(
	submittedRevision: number,
	currentRevision: number,
): string {
	return `賣家提出時是第 ${submittedRevision} 版，目前是第 ${currentRevision} 版`;
}

/**
 * 提出時的版本與目前版本有差異時，並列兩版的理由與期間。版本相同、或找不到其中一版（不是有效資料）時回 undefined。
 * 期間是該版的結束時間（沒有固定期間時沒有）。
 */
export function revisionComparison(
	measure: Pick<Measure, "revisions" | "currentRevision">,
	submittedRevision: number,
):
	| {
			submitted: { revision: number; statement: string; endTime?: Date };
			current: { revision: number; statement: string; endTime?: Date };
	  }
	| undefined {
	if (submittedRevision === measure.currentRevision) return undefined;
	const find = (n: number) => measure.revisions.find((r) => r.revision === n);
	const submitted = find(submittedRevision);
	const current = find(measure.currentRevision);
	if (!submitted || !current) return undefined;
	const view = (r: typeof submitted) => ({
		revision: r.revision,
		statement: r.statement,
		endTime: r.endTime ? timestampDate(r.endTime) : undefined,
	});
	return { submitted: view(submitted), current: view(current) };
}

/**
 * 佐證的連結：只收伺服器給的相對路徑（`/api/files/admin/…`），經同源 `/api/bff` 讀取（沿用 KYC 的 documentSrc）；
 * 路徑不合規時沒有 href，畫面只顯示名稱、不連出去。
 */
export function evidenceLinks(
	evidence: readonly Pick<Evidence, "position" | "viewUri">[],
): { label: string; href?: string }[] {
	return evidence.map((e) => ({
		label: `佐證 ${e.position + 1}`,
		href: documentSrc(e.viewUri),
	}));
}
