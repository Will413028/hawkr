import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ListMeasuresRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type {
	Measure,
	MeasureRequest,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	MeasureRequestKind,
	MeasureRequestState,
	MeasureState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerMeasureRequestKind } from "@hawkr/ui-text/keys";
import {
	formatStaff,
	sellerMeasureRequestStateWithKindStaff,
} from "@hawkr/ui-text/zh-TW";
import { staffTextOrUnknown, unknownText } from "@/lib/ui-text";

// 措施收件匣（/admin/measures）的純邏輯：分頁對應的伺服器篩選、請求種類與狀態的文字、已等候天數。時間一律由呼叫端傳入（now）。
// 排序由伺服器決定（ListMeasures：複審到期在前，其次依請求提出時間舊到新），這裡與畫面都不重排。

export type MeasureTabKey = "pending" | "review-due" | "active" | "all";

export interface MeasureListTab {
	key: MeasureTabKey;
	label: string;
	emptyText: string;
	/** 這個分頁送給 ListMeasures 的篩選；其餘條件不送。 */
	filter: Pick<
		MessageInitShape<typeof ListMeasuresRequestSchema>,
		"requestState" | "reviewDue" | "state"
	>;
}

/** 清單的分頁：待裁定｜複審到期｜生效中｜全部，全部是 ListMeasures 既有的伺服器篩選。 */
export const MEASURE_LIST_TABS: readonly MeasureListTab[] = [
	{
		key: "pending",
		label: "待裁定",
		emptyText: "沒有待裁定的申覆或解除請求",
		filter: { requestState: MeasureRequestState.PENDING },
	},
	{
		key: "review-due",
		label: "複審到期",
		emptyText: "沒有到期的複審",
		filter: { reviewDue: true },
	},
	{
		key: "active",
		label: "生效中",
		emptyText: "尚無措施紀錄",
		filter: { state: MeasureState.ACTIVE },
	},
	{
		key: "all",
		label: "全部",
		emptyText: "尚無措施紀錄",
		filter: {},
	},
];

export function measureListRequest(
	tab: MeasureListTab,
	pageToken: string,
	pageSize: number,
): MessageInitShape<typeof ListMeasuresRequestSchema> {
	return { ...tab.filter, pageToken, pageSize };
}

// ---------------------------------------------------------------------------
// 請求（申覆、解除請求）

/** 請求種類的顯示文字，取自共用文案目錄；UNSPECIFIED 與不認得的值是「未知」。 */
export function requestKindLabel(kind: MeasureRequestKind): string {
	return staffTextOrUnknown(sellerMeasureRequestKind(kind));
}

/**
 * 請求的種類加狀態（「申覆待裁定」「解除請求成立」…），後台員工受眾的文字取自共用文案目錄；
 * 種類或狀態是 UNSPECIFIED、狀態不認得時是「未知」（不認得的種類用目錄 `other` 的文字）。
 */
export function requestStateText(
	request: Pick<MeasureRequest, "kind" | "state">,
): string {
	if (
		request.kind === MeasureRequestKind.UNSPECIFIED ||
		request.state === MeasureRequestState.UNSPECIFIED
	) {
		return unknownText();
	}
	const message = sellerMeasureRequestStateWithKindStaff(request.state, {
		kind: request.kind,
	});
	return message ? formatStaff(message) : unknownText();
}

/** 措施上待處理的請求（伺服器推導的 state 是 PENDING；同一措施同時最多一筆）；沒有回 undefined。 */
export function pendingRequestOf(
	measure: Pick<Measure, "requests">,
): MeasureRequest | undefined {
	return measure.requests.find((r) => r.state === MeasureRequestState.PENDING);
}

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * 請求已等候的完整天數：現在減提出時間，向下取整。不用台北日期相減：23:59 提出的請求隔天早上就會算 1 天，
 * 和 7×24 小時的時限對不起來。時鐘差造成的負值算 0。
 */
export function waitingDays(submitTime: Date, now: Date): number {
	return Math.max(
		0,
		Math.floor((now.getTime() - submitTime.getTime()) / DAY_MS),
	);
}

/** 「已等候 {N} 天」；0 天是「今天提出」。 */
export function waitingText(days: number): string {
	return days === 0 ? "今天提出" : `已等候 ${days} 天`;
}

/** 清單列的「已等候」：只有待處理的請求有；沒有提出時間（不是有效資料）回 undefined。 */
export function rowWaitingText(
	measure: Pick<Measure, "requests">,
	now: Date,
): string | undefined {
	const submit = pendingRequestOf(measure)?.submitTime;
	return submit
		? waitingText(waitingDays(timestampDate(submit), now))
		: undefined;
}

/** 店家欄：店名；沒有店名時「店家 {store_id}」（不退回措施 ID，措施 ID 不是店家的識別）。 */
export function storeColumnText(
	measure: Pick<Measure, "storeName" | "storeId">,
): string {
	return measure.storeName || `店家 ${measure.storeId}`;
}
