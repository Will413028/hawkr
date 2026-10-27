import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	CaseSchema,
	CaseState,
	OutcomeOptionSchema,
	ReportSchema,
	ReportState,
	TriageTargetSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import type { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

/** 測試用的通報與案件：伺服器回應的資料，欄位預設是一筆待分案的食安通報／進行中的食安案件，用 patch 改。 */

export const STORE_ID = "100";
export const REPORT_ID = "0190b000-0000-7000-8000-000000000001";
export const CASE_ID = "0190b000-0000-7000-8000-0000000000c1";

export const T0 = new Date("2026-10-18T02:00:00Z");

export function ts(date: Date) {
	return timestampFromDate(date);
}

export function report(patch: MessageInitShape<typeof ReportSchema> = {}) {
	return create(ReportSchema, {
		id: REPORT_ID,
		storeId: STORE_ID,
		storeName: "阿里山農場",
		category: PolicyCategory.FOOD_SAFETY,
		description: "吃了便當之後腹瀉",
		state: ReportState.AWAITING_TRIAGE,
		createTime: ts(T0),
		etag: "r-1",
		...patch,
	});
}

/** 伺服器給的一個可設定的結果（OutcomeOption）；negatesSource 是伺服器說設定它會否定這筆通報。 */
export function outcome(value: ReportOutcome, negatesSource = false) {
	return create(OutcomeOptionSchema, { outcome: value, negatesSource });
}

/**
 * 伺服器給的分案與移動的去處（TriageTarget）：沒給案件就是這個類別會開新案件，給了就是加入那個進行中的案件
 * （id、嚴重度、etag、現有通報數）。
 */
export function target(
	category: PolicyCategory,
	joins?: {
		caseId: string;
		severity: ViolationSeverity;
		etag?: string;
		reportCount?: number;
	},
) {
	return create(TriageTargetSchema, {
		category,
		caseId: joins?.caseId ?? "",
		severity: joins?.severity ?? ViolationSeverity.UNSPECIFIED,
		caseEtag: joins ? (joins.etag ?? "c-1") : "",
		caseReportCount: joins?.reportCount ?? 0,
	});
}

export function kase(patch: MessageInitShape<typeof CaseSchema> = {}) {
	return create(CaseSchema, {
		id: CASE_ID,
		storeId: STORE_ID,
		storeName: "阿里山農場",
		category: PolicyCategory.FOOD_SAFETY,
		severity: ViolationSeverity.HIGH,
		state: CaseState.OPEN,
		openedByAdminId: "9001",
		openTime: ts(T0),
		reportCount: 1,
		etag: "c-1",
		...patch,
	});
}

/**
 * 版本衝突：etag 不符（aborted＋ErrorInfo VERSION_CONFLICT）。conflictTarget 是伺服器在 ErrorInfo.metadata 說的哪個物件的 etag 過時
 * （`report`、`case`、`target_case` 或 `category_case_opened`）；不給就沒有 metadata。
 */
export function versionConflict(
	conflictTarget?: "report" | "case" | "target_case" | "category_case_opened",
): ConnectError {
	return new ConnectError("etag mismatch", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: {
				reason: "VERSION_CONFLICT",
				domain: "hawkr",
				metadata: conflictTarget ? { conflictTarget } : {},
			},
		},
	]);
}

/** 伺服器隨錯誤給的一句話（LocalizedMessage）。 */
export function withMessage(code: Code, message: string): ConnectError {
	return new ConnectError("raw", code, undefined, [
		{ desc: LocalizedMessageSchema, value: { locale: "zh-TW", message } },
	]);
}
