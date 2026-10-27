import {
	AuthorityHoldExtensionCause,
	type AuthorityOrder,
	AuthorityOrderKind,
	AuthorityOrderState,
	type AuthorityOrderTarget,
	AuthorityStoreCoverage,
	AuthorityTargetNoticeKind,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { unknownText } from "@/lib/ui-text";

// 主管機關來函的名稱（3c-b 設計書 §5.4 (d) L3、L6、M5、M6）。這些列舉只有 admin 用，不在共用文案目錄裡，所以是 admin 的單端表
// （UNSPECIFIED 不在表裡）；不認得的值與 UNSPECIFIED 顯示「未知」。

function textOf<E extends number>(
	table: Partial<Record<E, string>>,
	value: E,
): string {
	return table[value] ?? unknownText();
}

const KIND_TEXT: Record<
	Exclude<AuthorityOrderKind, AuthorityOrderKind.UNSPECIFIED>,
	string
> = {
	[AuthorityOrderKind.RESTRICT_PRODUCT]: "限制或移除商品",
	[AuthorityOrderKind.DATA_REQUEST]: "調取資料",
	[AuthorityOrderKind.SUSPEND_ACCOUNT]: "暫停帳號",
	[AuthorityOrderKind.OTHER]: "其他",
};

/** 種類的選項，順序同契約。 */
export const AUTHORITY_KIND_OPTIONS: readonly Exclude<
	AuthorityOrderKind,
	AuthorityOrderKind.UNSPECIFIED
>[] = [
	AuthorityOrderKind.RESTRICT_PRODUCT,
	AuthorityOrderKind.DATA_REQUEST,
	AuthorityOrderKind.SUSPEND_ACCOUNT,
	AuthorityOrderKind.OTHER,
];

export function authorityKindText(kind: AuthorityOrderKind): string {
	return textOf<AuthorityOrderKind>(KIND_TEXT, kind);
}

/**
 * 撤銷登記（3c-b 修正輪）的字串：Will 2026-10-20 授權由 lead 定稿。
 * 伺服器的句子（authority_order.unavailable.revoked、revoke_reason_length）由伺服器給，不在這裡。
 */
export const AUTHORITY_REVOKE_TEXT = {
	/** 來函狀態 REVOKED。 */
	state: "已撤銷登記",
	/** 詳情頁的按鈕與對話框的標題、送出鈕。 */
	action: "撤銷登記",
	description:
		"只在登記錯誤時撤銷（例如對象或文號登記錯了）。撤銷之後這筆來函的保存解除，不再是處理中，也不能再當作措施的來源；紀錄會保留。正確的來函請另外登記。",
	reasonLabel: "原因",
	/** 重複送出（先前已由同一位管理員撤銷）的橫幅。 */
	already: "這筆來函先前已撤銷登記。",
	/** 清單的分頁籤與「撤銷時間」欄。 */
	tab: "已撤銷登記",
	timeColumn: "撤銷時間",
	/** 保存區塊：撤銷登記之後保存已解除（不是期限到了）。 */
	holdReleased: "撤銷登記時解除",
} as const;

/** 處理區塊：「已於 {時間} 撤銷登記：{原因}」。 */
export function revokedText(time: string, reason: string): string {
	return `已於 ${time} 撤銷登記：${reason}`;
}

const STATE_TEXT: Record<
	Exclude<AuthorityOrderState, AuthorityOrderState.UNSPECIFIED>,
	string
> = {
	[AuthorityOrderState.OPEN]: "處理中",
	[AuthorityOrderState.COMPLETED]: "已完成",
	[AuthorityOrderState.REVOKED]: AUTHORITY_REVOKE_TEXT.state,
};

export function authorityStateText(state: AuthorityOrderState): string {
	return textOf<AuthorityOrderState>(STATE_TEXT, state);
}

const COVERAGE_TEXT: Record<
	Exclude<AuthorityStoreCoverage, AuthorityStoreCoverage.UNSPECIFIED>,
	string
> = {
	[AuthorityStoreCoverage.HELD]: "保存中",
	[AuthorityStoreCoverage.PURGED_BEFORE_REGISTRATION]:
		"登記之前已依保留期限清除",
	[AuthorityStoreCoverage.HOLD_ENDED]: "保存期限已過",
	[AuthorityStoreCoverage.PURGED_AFTER_HOLD_ENDED]:
		"保存期限已過，個資已依保留期限清除",
	// 撤銷登記的文案：Will 2026-10-20 授權由 lead 定稿。
	[AuthorityStoreCoverage.HOLD_RELEASED_BY_REVOCATION]: "撤銷登記時解除保存",
};

/** 對象逐家的涵蓋狀態（M5）。 */
export function authorityCoverageText(
	coverage: AuthorityStoreCoverage,
): string {
	return textOf<AuthorityStoreCoverage>(COVERAGE_TEXT, coverage);
}

/** 沒有店家資料的對象（M5）。 */
export const NO_SELLER_DATA_TEXT =
	"這個帳號在平台沒有店家資料；買家與帳號資料的保存尚未支援";

const NOTICE_KIND_TEXT: Record<
	Exclude<AuthorityTargetNoticeKind, AuthorityTargetNoticeKind.UNSPECIFIED>,
	string
> = {
	[AuthorityTargetNoticeKind.NOTIFIED]: "已告知",
	[AuthorityTargetNoticeKind.WITHHELD]: "機關要求暫不告知",
	[AuthorityTargetNoticeKind.MEASURE_IS_NOTICE]: "由措施告知",
};

/** 完成來函時每個對象的告知方式（M6），順序同契約。 */
export const NOTICE_KIND_OPTIONS: readonly Exclude<
	AuthorityTargetNoticeKind,
	AuthorityTargetNoticeKind.UNSPECIFIED
>[] = [
	AuthorityTargetNoticeKind.NOTIFIED,
	AuthorityTargetNoticeKind.WITHHELD,
	AuthorityTargetNoticeKind.MEASURE_IS_NOTICE,
];

export function authorityNoticeKindText(
	kind: AuthorityTargetNoticeKind,
): string {
	return textOf<AuthorityTargetNoticeKind>(NOTICE_KIND_TEXT, kind);
}

/** M6：由措施告知的限制（已核准）。 */
export const MEASURE_NOTICE_CONDITION_TEXT =
	"只限暫停帳號或限制商品，而且已經從這份來函建立措施。";
/** M8：暫不告知的後續。 */
export const WITHHELD_FOLLOW_UP_TEXT =
	"機關要求暫不告知；到了確認日期請向機關確認能否告知。";
/** M7：調取資料完成時的保存期限說明。 */
export const DATA_PROVIDE_NOTE_TEXT = "保存期限會延到提供後 6 個月。";
/** M2 的延長歷史自動延長（M3）。 */
const AUTO_EXTENSION_TEXT = "自動延長：提供資料之後 6 個月";

/** 延長歷史的一列（M3）：「{時間}・{管理員 ID}：保存到 {前值} → {新值}（{原因}）」；自動延長時括號裡是「自動延長：提供資料之後 6 個月」。 */
export function holdExtensionText(extension: {
	createTime: string;
	adminId: string;
	fromTime: string;
	toTime: string;
	cause: AuthorityHoldExtensionCause;
	reason: string;
}): string {
	const why =
		extension.cause === AuthorityHoldExtensionCause.DATA_PROVIDED
			? AUTO_EXTENSION_TEXT
			: extension.reason;
	return `${extension.createTime}・${extension.adminId}：保存到 ${extension.fromTime} → ${extension.toTime}（${why}）`;
}

/** 保存期限的一句（L16）：「保存到 {時間}（保存中）」或「保存到 {時間}（已到期）」。 */
/** 保存區塊（L16）：保存中／已到期；撤銷登記的來函是「撤銷登記時解除」（見 AUTHORITY_REVOKE_TEXT；Will 2026-10-20 授權由 lead 定稿）。 */
export function holdUntilText(
	time: string,
	holdActive: boolean,
	revoked = false,
): string {
	const status = revoked
		? AUTHORITY_REVOKE_TEXT.holdReleased
		: holdActive
			? "保存中"
			: "已到期";
	return `保存到 ${time}（${status}）`;
}

/** 對象在清單的名稱：指定店家用店名（個資已清理時是店家 ID），只指名帳號是「帳號 {id} 的全部店家」（L7）。 */
export function targetLabel(target: AuthorityOrderTarget): string {
	if (target.storeId === "") return `帳號 ${target.accountId} 的全部店家`;
	const store = target.coveredStores.find((s) => s.storeId === target.storeId);
	return store?.storeName || target.storeName || target.storeId;
}

/** 清單的「指名的店家」欄（L7）：只有一家店就是它的名稱，多家是「{第一家} 等 {n} 家」（n 是所有對象涵蓋的不重複店家數，不是對象數）；沒有店家資料時用第一個對象的名稱。 */
export function namedStoresText(
	order: Pick<AuthorityOrder, "targets">,
): string {
	const names = new Map<string, string>();
	for (const target of order.targets) {
		for (const store of target.coveredStores) {
			if (!names.has(store.storeId)) {
				names.set(store.storeId, store.storeName || store.storeId);
			}
		}
	}
	if (names.size === 0) {
		return order.targets.length === 0 ? "-" : targetLabel(order.targets[0]);
	}
	const [first] = names.values();
	return names.size === 1 ? first : `${first} 等 ${names.size} 家`;
}

/** 從來函建立措施時可選的店家（L20）：每家一個選項「{帳號 ID}・{店名}」。 */
export interface MeasureStoreOption {
	storeId: string;
	accountId: string;
	label: string;
}

/** 建立措施的入口依哪個伺服器旗標判斷（AuthorityTargetStore）：措施（警告、停權、風險）或終止合作。 */
export type MeasureStoreFlag = "canCreateMeasure" | "canCreateTermination";

/**
 * 從來函可以對它建立措施的店家：只列伺服器旗標為真的（已關閉、已清除的店與已撤銷的來函，旗標都是假）。
 * 同一家店在多個對象出現只列一次。
 */
export function measureStoreOptions(
	order: Pick<AuthorityOrder, "targets">,
	flag: MeasureStoreFlag,
): MeasureStoreOption[] {
	const seen = new Set<string>();
	const out: MeasureStoreOption[] = [];
	for (const target of order.targets) {
		for (const store of target.coveredStores) {
			if (!store[flag] || seen.has(store.storeId)) continue;
			seen.add(store.storeId);
			out.push({
				storeId: store.storeId,
				accountId: target.accountId,
				label: `${target.accountId}・${store.storeName || store.storeId}`,
			});
		}
	}
	return out;
}
