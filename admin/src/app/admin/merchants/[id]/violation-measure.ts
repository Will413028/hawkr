import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import type { CreateMeasureRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	MeasureKind,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { ASSESSABLE_CATEGORIES } from "@/lib/policy-category";
import {
	DAY_MS,
	fromLocalInput,
	NOTE_MAX_LENGTH,
	STATEMENT_MAX_LENGTH,
	sortedIds,
	sourceAuthorityOrderField,
	sourceCaseField,
	toLocalInput,
} from "./measure";

// 建立措施（違規依據的警告與停權，CreateMeasure 的 warning／suspension.violation）的輸入、驗證與請求組裝
// （決策簡報 step10e-3-brief-v1 K7、step10e-3-brief D10）。畫面只接結果。時間一律由呼叫端傳入（now），這裡不讀時鐘。

/** 一個新認定最多的來源通報數（NewFinding.source_report_ids 的上限）。 */
export const SOURCE_REPORTS_MAX = 20;

/**
 * 種類下拉的選項順序：違規依據只有警告與停權（終止合作有自己的對話框）。選項的值是請求 oneof 的 case
 * （`CreateMeasureRequest.measure`），標籤用目錄的種類名詞（`kind`）；沒有選是 `case: undefined`。
 */
export const VIOLATION_MEASURE_OPTIONS = [
	{ case: "warning", kind: MeasureKind.WARNING },
	{ case: "suspension", kind: MeasureKind.SUSPENSION },
] as const;

/** 選到的種類，形狀同 `CreateMeasureRequest.measure` 的 oneof：結束時間只存在於停權（`datetime-local` 的字串，空字串＝沒填）。 */
export type ViolationMeasureChoice =
	| { case: undefined }
	| { case: "warning" }
	| { case: "suspension"; endTime: string };

/** 嚴重度下拉的選項順序（同 enum；不含 UNSPECIFIED）。認定的類別選項是 `ASSESSABLE_CATEGORIES`（政策分類中平台判定與認定可用的值）。 */
export const VIOLATION_SEVERITY_OPTIONS: readonly ViolationSeverity[] = [
	ViolationSeverity.LOW,
	ViolationSeverity.MEDIUM,
	ViolationSeverity.HIGH,
	ViolationSeverity.CRITICAL,
];

/** 對話框的輸入。 */
export interface ViolationMeasureForm {
	measure: ViolationMeasureChoice;
	category: PolicyCategory;
	severity: ViolationSeverity;
	/** 成立新認定的通報 id（同一家店、沒有判定不成立）；可以不選。集合：送出時排序（sortedIds）。 */
	sourceReportIds: readonly string[];
	/** 一併引用的既有認定 id；可以不選。集合：送出時排序（sortedIds）。 */
	citedFindingIds: readonly string[];
	/** 給賣家的理由。 */
	statement: string;
	/** 第一則內部註記（賣家看不到）。 */
	note: string;
	/** 從案件建立時的案件 id；沒有或空字串是不連案件。 */
	sourceCaseId?: string;
	/** 從主管機關來函建立時的來函 id；沒有或空字串是不連來函。 */
	sourceAuthorityOrderId?: string;
	/** 成立新認定的主管機關來函（集合，送出時排序，見 sortedIds）；從來函建立時是那一份來函。 */
	sourceAuthorityOrderIds?: readonly string[];
}

/**
 * 對話框開啟時的初始值：只是 `useState` 的初始值，之後每一欄都可以改，必填的檢查照常；沒給的欄位維持空白。
 * 從發票申訴的停權建議開啟時帶種類、結束時間與理由（measureInitialFromSuggestion），類別不預填，由管理員選；
 * 從通報詳情開啟時帶預先勾選的來源通報。
 */
export interface CreateMeasureInitial {
	/** 預選的種類；停權的結束時間是 `datetime-local` 的字串（台北時間）。 */
	measure?: { case: "warning" } | { case: "suspension"; endTime?: string };
	/** 給賣家的理由。 */
	statement?: string;
	category?: PolicyCategory;
	/** 預先勾選的來源通報（id 的集合）。 */
	sourceReportIds?: readonly string[];
	/** 成立新認定的來源來函（id 的集合）；從主管機關來函開啟時是那一份來函。 */
	sourceAuthorityOrderIds?: readonly string[];
}

/**
 * 發票申訴的停權建議帶進建立措施對話框的初始值（3b 修訂 6-1）：種類是停權；結束時間是 now 加建議的天數，天數是空字串、0 或
 * 不是正整數時不預填，送出時由「請填寫結束時間。」擋下；理由是建議的原文，空字串時不預填；類別不預填，由管理員選。
 * 發票模組只提建議、沒有任何人因此被停權，所以這些值都只是起點，每一欄都可以改。時間一律由呼叫端傳入（now），這裡不讀時鐘。
 */
export function measureInitialFromSuggestion(
	suggestion: { suggestedSuspendDays: string; suggestedReason: string },
	now: Date,
): CreateMeasureInitial {
	const initial: CreateMeasureInitial = {};
	let endTime: string | undefined;
	const days = /^\d+$/u.test(suggestion.suggestedSuspendDays)
		? Number(suggestion.suggestedSuspendDays)
		: 0;
	const end = new Date(now.getTime() + days * DAY_MS);
	if (days > 0 && !Number.isNaN(end.getTime())) {
		// 對話框接不了的時間（例如年份超過四位數）不預填，免得一開啟就是一個送不出去的值。
		const text = toLocalInput(end);
		if (fromLocalInput(text)) endTime = text;
	}
	initial.measure = endTime
		? { case: "suspension", endTime }
		: { case: "suspension" };
	if (suggestion.suggestedReason !== "") {
		initial.statement = suggestion.suggestedReason;
	}
	return initial;
}

/**
 * 送出前的檢查，與伺服器的規則一致（CreateMeasureRequest）：種類（oneof 的 case）是警告或停權；類別與嚴重度必須是選項裡的值；
 * 來源通報不超過上限、不重複；理由與註記去掉前後空白後 1–2000 字；停權的結束時間必填、晚於現在，警告不帶結束時間。
 * 回傳第一個錯誤，沒有錯誤回 undefined。
 * `resend` 是同一份內容的重送（沿用上一次的 request_id）：上一次可能已經建立成功，伺服器依 request_id 回傳原本的措施，
 * 與現在的時間無關，所以不再用現在的時間檢查結束時間；格式與必填照常檢查。
 */
export function validateViolationMeasure(
	form: ViolationMeasureForm,
	now: Date,
	resend = false,
): string | undefined {
	if (form.measure.case === undefined) return "請選擇種類。";
	if (!ASSESSABLE_CATEGORIES.includes(form.category)) {
		return "請選擇類別。";
	}
	if (!VIOLATION_SEVERITY_OPTIONS.includes(form.severity)) {
		return "請選擇嚴重度。";
	}
	if (form.sourceReportIds.length > SOURCE_REPORTS_MAX) {
		return `來源通報最多 ${SOURCE_REPORTS_MAX} 筆。`;
	}
	const statement = trimGoSpace(form.statement);
	if (statement.length === 0) return "請填寫給賣家的理由。";
	if (goTrimmedRuneCount(statement) > STATEMENT_MAX_LENGTH) {
		return `給賣家的理由最多 ${STATEMENT_MAX_LENGTH} 字。`;
	}
	if (form.measure.case === "suspension") {
		if (form.measure.endTime === "") return "請填寫結束時間。";
		const end = fromLocalInput(form.measure.endTime);
		if (!end) return "結束時間的格式不正確。";
		if (!resend && end.getTime() <= now.getTime()) {
			return "結束時間必須晚於現在。";
		}
	}
	const note = trimGoSpace(form.note);
	if (note.length === 0) return "請填寫內部註記。";
	if (goTrimmedRuneCount(note) > NOTE_MAX_LENGTH) {
		return `內部註記最多 ${NOTE_MAX_LENGTH} 字。`;
	}
	return undefined;
}

/**
 * CreateMeasure 的請求：違規依據的警告或停權。新認定一定有（類別、嚴重度、來源通報），既有認定選填；理由與註記送去掉前後空白的內容；
 * 來源通報與引用的認定是集合，排序後送出（同一組選擇不論勾選順序都是同一個請求）。
 * 呼叫前須先通過 validateViolationMeasure（種類、類別、嚴重度或結束時間不是有效值時丟例外，不送出缺值的請求）。
 */
export function buildCreateViolationMeasure(
	requestId: string,
	storeId: string,
	form: ViolationMeasureForm,
): MessageInitShape<typeof CreateMeasureRequestSchema> {
	if (!ASSESSABLE_CATEGORIES.includes(form.category)) {
		throw new Error("類別不是有效的值");
	}
	if (!VIOLATION_SEVERITY_OPTIONS.includes(form.severity)) {
		throw new Error("嚴重度不是有效的值");
	}
	const violation = {
		finding: {
			category: form.category,
			severity: form.severity,
			sourceReportIds: sortedIds(form.sourceReportIds),
			sourceAuthorityOrderIds: sortedIds(form.sourceAuthorityOrderIds ?? []),
		},
		citedFindingIds: sortedIds(form.citedFindingIds),
	};
	const base = {
		requestId,
		storeId,
		statement: trimGoSpace(form.statement),
		note: trimGoSpace(form.note),
		...sourceCaseField(form.sourceCaseId),
		...sourceAuthorityOrderField(form.sourceAuthorityOrderId),
	};
	switch (form.measure.case) {
		case "warning":
			return { ...base, measure: { case: "warning", value: { violation } } };
		case "suspension": {
			const end = fromLocalInput(form.measure.endTime);
			if (!end) throw new Error("結束時間不是有效的時間");
			return {
				...base,
				measure: {
					case: "suspension",
					value: {
						basis: { case: "violation", value: violation },
						endTime: timestampFromDate(end),
					},
				},
			};
		}
		case undefined:
			throw new Error("種類不是警告或停權");
	}
}

/**
 * 輸入內容的鍵：去掉前後空白後的內容，與送出的請求一一對應（警告沒有結束時間，不納入；來源通報與引用的認定用排序後的清單）。
 * 與 request_id 一起用（measure.ts 的 requestIdFor）。
 */
export function violationMeasureRequestKey(
	storeId: string,
	form: ViolationMeasureForm,
): string {
	return JSON.stringify([
		storeId,
		form.measure.case,
		form.category,
		form.severity,
		sortedIds(form.sourceReportIds),
		sortedIds(form.citedFindingIds),
		trimGoSpace(form.statement),
		form.measure.case === "suspension" ? form.measure.endTime : "",
		trimGoSpace(form.note),
		form.sourceCaseId ?? "",
		form.sourceAuthorityOrderId ?? "",
		sortedIds(form.sourceAuthorityOrderIds ?? []),
	]);
}
