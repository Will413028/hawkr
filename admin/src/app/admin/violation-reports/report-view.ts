import { type Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import type {
	Report,
	ReportMove,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { formatDate as formatCalendarDate } from "@hawkr/api/proto/date";
import { documentSrc } from "@/app/admin/merchants/[id]/kyc-application";
import {
	measureKindLabel,
	measureStateBadge,
} from "@/app/admin/merchants/[id]/measure";
import { formatDateTimeShort } from "@/lib/format";
import { policyCategoryText } from "@/lib/policy-category";

// 通報與案件詳情頁的顯示邏輯（純函式）：檢舉人、佐證的名稱、補充與移動紀錄的句子。資料與可做的動作都來自伺服器。

/** 時間的顯示（台北時間，到分鐘）；沒有時間是「-」。 */
export function timeText(value: Timestamp | undefined): string {
	return value ? formatDateTimeShort(timestampDate(value)) : "-";
}

export type ReporterView =
	| { kind: "buyer"; buyerId: string }
	| {
			kind: "anonymous";
			/** 沒有提供的欄位不列。 */
			contact: { label: string; value: string }[];
			/** 聯絡方式已依保留期限清除時的說明。 */
			erasedText: string | undefined;
	  };

/** 檢舉人：登入的檢舉人是買家 ID；匿名的列出電話、聯絡信箱與姓名，聯絡方式已清除時說明清除的時間。 */
export function reporterView(
	report: Pick<Report, "reporterBuyerId" | "contact" | "contactEraseTime">,
): ReporterView {
	if (report.reporterBuyerId) {
		return { kind: "buyer", buyerId: report.reporterBuyerId };
	}
	const contact: { label: string; value: string }[] = [];
	if (report.contact?.phone) {
		contact.push({ label: "電話", value: report.contact.phone });
	}
	if (report.contact?.email) {
		contact.push({ label: "聯絡信箱", value: report.contact.email });
	}
	if (report.contact?.name) {
		contact.push({ label: "姓名", value: report.contact.name });
	}
	return {
		kind: "anonymous",
		contact,
		erasedText: report.contactEraseTime
			? `聯絡方式已於 ${timeText(report.contactEraseTime)} 依保留期限清除`
			: undefined,
	};
}

/** 補充的標題：「補充 {n}・{時間}」（n 從 1 起，舊到新）。 */
export function supplementTitle(
	index: number,
	createTime: Timestamp | undefined,
): string {
	return `補充 ${index + 1}・${timeText(createTime)}`;
}

export interface EvidenceItem {
	label: string;
	/** 伺服器給的相對路徑不合規時沒有連結，畫面只顯示名稱。 */
	href: string | undefined;
}

/**
 * 佐證的名稱與連結（機敏檔，每次讀取都記錄，所以在新分頁開啟、不預先載入）。原通報的佐證是「佐證 {n}」，
 * 補充帶來的佐證是「補充 {m} 的佐證 {n}」（m 是第幾則補充，n 是那則補充的第幾份）；佐證在 Report.evidence 裡依位置排列，
 * 補充指出它的佐證在哪些位置（evidence_positions）。
 */
export function evidenceItems(
	report: Pick<Report, "evidence" | "supplements">,
): EvidenceItem[] {
	const owner = new Map<number, { supplement: number; ordinal: number }>();
	report.supplements.forEach((supplement, i) => {
		supplement.evidencePositions.forEach((position, j) => {
			owner.set(position, { supplement: i + 1, ordinal: j + 1 });
		});
	});
	let original = 0;
	return report.evidence.map((evidence) => {
		const from = owner.get(evidence.position);
		const label = from
			? `補充 ${from.supplement} 的佐證 ${from.ordinal}`
			: `佐證 ${++original}`;
		return { label, href: documentSrc(evidence.viewUri) };
	});
}

/** 案件裡一筆通報的摘要：「通報 {id}・{類別}・建立 {時間}」（類別是檢舉人選的）。 */
export function reportSummary(
	report: Pick<Report, "id" | "category" | "createTime">,
): string {
	return `通報 ${report.id}・${policyCategoryText(report.category)}・建立 ${timeText(report.createTime)}`;
}

/** 補充的摘要：「補充 {n} 則・最新 {時間}」；沒有補充是 undefined。 */
export function supplementSummary(
	report: Pick<Report, "supplementCount" | "latestSupplementTime">,
): string | undefined {
	if (report.supplementCount <= 0) return undefined;
	return `補充 ${report.supplementCount} 則・最新 ${timeText(report.latestSupplementTime)}`;
}

/** 移動紀錄的一行，站在 caseId 這個案件看：移進來的，或移出去的。 */
export function moveRecordText(move: ReportMove, caseId: string): string {
	const when = timeText(move.moveTime);
	return move.toCaseId === caseId
		? `通報 ${move.reportId} 於 ${when} 從案件 ${move.fromCaseId} 移入（${move.movedByAdminId}）`
		: `通報 ${move.reportId} 於 ${when} 移到案件 ${move.toCaseId}（${move.movedByAdminId}）`;
}

/** 日期（google.type.Date）的顯示：「2026/10/19」；沒有日期是「-」。 */
export function calendarDateText(
	date: { year: number; month: number; day: number } | undefined,
): string {
	const iso = formatCalendarDate(date);
	return iso === "" ? "-" : iso.replaceAll("-", "/");
}

/** 導出的措施的一行：「{種類}・{狀態}・措施 {id}」。 */
export function derivedMeasureText(
	measure: Pick<Measure, "id" | "kind" | "state">,
): string {
	return `${measureKindLabel(measure.kind)}・${measureStateBadge(measure.state, measure.kind).label}・措施 ${measure.id}`;
}

/** 「{admin ID}・{時間}」：誰在什麼時候做的（分案、結案）；都沒有是「-」。 */
export function actorTimeText(
	adminId: string,
	time: Timestamp | undefined,
): string {
	if (!adminId && !time) return "-";
	return `${adminId || "-"}・${timeText(time)}`;
}
