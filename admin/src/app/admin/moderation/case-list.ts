import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	type Case,
	CaseState,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { ReportTargetKind } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import { durationText } from "@/lib/deadline";
import { formatDateTimeShort } from "@/lib/format";
import {
	caseStateText,
	caseStateVariant,
	targetKindText,
} from "./moderation-text";

export const CASES_PAGE_SIZE = 20;

/** 狀態篩選：預設進行中；UNSPECIFIED 是全部。 */
export const CASE_STATE_FILTERS: readonly {
	state: CaseState;
	label: string;
}[] = [
	{ state: CaseState.OPEN, label: "進行中" },
	{ state: CaseState.CLOSED, label: "已結案" },
	{ state: CaseState.UNSPECIFIED, label: "全部" },
];

export interface CaseFilters {
	state: CaseState;
	targetKind: ReportTargetKind;
}

export const DEFAULT_CASE_FILTERS: CaseFilters = {
	state: CaseState.OPEN,
	targetKind: ReportTargetKind.UNSPECIFIED,
};

/** ListCases 的請求：page token 綁定篩選條件，換條件時呼叫端回到第一頁。 */
export function listCasesRequest(filters: CaseFilters, pageToken: string) {
	return {
		pageSize: CASES_PAGE_SIZE,
		pageToken,
		state: filters.state,
		targetKind: filters.targetKind,
	};
}

export interface CaseRow {
	id: string;
	targetKind: string;
	targetId: string;
	state: string;
	stateVariant: "warning" | "gray";
	createTime: string;
	dwell: string;
}

/** 一個案件一列；停留時間是開案到結案（進行中到現在）。時間由呼叫端傳入（now）。 */
export function caseRow(kase: Case, now: Date): CaseRow {
	const created = kase.createTime ? timestampDate(kase.createTime) : undefined;
	const end = kase.resolveTime ? timestampDate(kase.resolveTime) : now;
	return {
		id: kase.id,
		targetKind: targetKindText(
			kase.target?.kind ?? ReportTargetKind.UNSPECIFIED,
		),
		targetId: kase.target?.id ?? "",
		state: caseStateText(kase.state),
		stateVariant: caseStateVariant(kase.state),
		createTime: created ? formatDateTimeShort(created) : "-",
		dwell: created ? durationText(end.getTime() - created.getTime()) : "-",
	};
}
