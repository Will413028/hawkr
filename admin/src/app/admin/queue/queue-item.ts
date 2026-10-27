import {
	type ReviewQueueItem,
	ReviewQueueItemKind,
} from "@hawkr/api/gen/hawkr/admin/reviewqueue/v1alpha1/review_queue_pb";

/**
 * 待辦種類的文字（設計 step10e-3d §5 #104a–k）。只有 admin 一端用的 enum，所以是單端文字表；
 * UNSPECIFIED 不是有效值，不在表裡，也不顯示。
 */
export const QUEUE_KIND_TEXT: Record<
	Exclude<ReviewQueueItemKind, ReviewQueueItemKind.UNSPECIFIED>,
	string
> = {
	[ReviewQueueItemKind.REPORT_TRIAGE]: "分案通報",
	[ReviewQueueItemKind.CASE_DECISION]: "決定案件",
	[ReviewQueueItemKind.CASE_INQUIRY_OVERDUE]: "跟進逾期的詢問",
	[ReviewQueueItemKind.AUTHORITY_ORDER]: "處理主管機關來函",
	[ReviewQueueItemKind.AUTHORITY_TARGET_NOTICE]: "確認是否告知被指名者",
	[ReviewQueueItemKind.MEASURE_APPEAL]: "裁定申覆",
	[ReviewQueueItemKind.MEASURE_LIFT_REQUEST]: "裁定解除請求",
	[ReviewQueueItemKind.MEASURE_REVIEW]: "複審停權",
	[ReviewQueueItemKind.KYC_APPLICATION]: "審核商家認證",
	[ReviewQueueItemKind.PAYUNI_BIND_RETRY]: "重新綁定 PayUni",
};

/** 篩選下拉的選項順序：enum 的順序，不含 UNSPECIFIED。 */
export const QUEUE_KIND_OPTIONS: readonly ReviewQueueItemKind[] = (
	Object.keys(QUEUE_KIND_TEXT) as unknown as string[]
).map((key) => Number(key) as ReviewQueueItemKind);

/** 種類的文字；沒有有效種類（UNSPECIFIED 或不認得的值）是 undefined，畫面不顯示。 */
export function queueKindText(kind: ReviewQueueItemKind): string | undefined {
	return (QUEUE_KIND_TEXT as Record<number, string | undefined>)[kind];
}

/** 店家欄（#113）：店名；沒有店名是「店家 {id}」；沒有店家是「-」，但告知被指名者的對象有帳號時是「帳號 {id}」。 */
export function queueStoreText(item: ReviewQueueItem): string {
	if (item.storeName) return item.storeName;
	if (item.storeId) return `店家 ${item.storeId}`;
	const subject = item.subject;
	if (subject.case === "authorityTarget" && subject.value.accountId) {
		return `帳號 ${subject.value.accountId}`;
	}
	return "-";
}

/** 列的連結（設計 §4.1）：進到處理這件待辦的詳情頁；需要時帶錨點到列上的那一項。沒有對象時是 undefined。 */
export function itemHref(item: ReviewQueueItem): string | undefined {
	const subject = item.subject;
	switch (subject.case) {
		case "reportId":
			return `/admin/violation-reports/${subject.value}`;
		case "caseId":
			return `/admin/violation-reports/cases/${subject.value}`;
		case "caseInquiry":
			return `/admin/violation-reports/cases/${subject.value.caseId}#inquiry-${subject.value.inquiryId}`;
		case "authorityOrderId":
			return `/admin/authority-orders/${subject.value}`;
		case "authorityTarget":
			return `/admin/authority-orders/${subject.value.orderId}#target-${subject.value.position}`;
		case "measureRequest":
			return `/admin/measures/${subject.value.measureId}`;
		case "measureId":
			return `/admin/measures/${subject.value}`;
		case "kycApplication":
			return `/admin/merchants/${subject.value.storeId}#kyc`;
		default:
			return undefined;
	}
}
