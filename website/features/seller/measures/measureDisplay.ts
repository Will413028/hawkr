import { type Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import {
	MeasureBasis,
	MeasureKind,
	type MeasureRequest,
	MeasureRequestKind,
	MeasureRequestState,
	MeasureState,
	RemediationUnavailableReason,
	RevokeReason,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	commonPolicyCategory,
	sellerMeasureAppealDeadline,
	sellerMeasureBasis,
	sellerMeasureMetaKind,
	sellerMeasureMetaSeverity,
	sellerMeasureMetaTerminationGround,
	sellerMeasureRejectionCooldownWarning,
	sellerMeasureRequestKindSubmittedNote,
	sellerMeasureRequestReplyNote,
	sellerMeasureRequestStateWithKind,
	sellerMeasureState,
	sellerMeasureTerminationEffective,
	sellerMeasureTerminationEffectiveEnded,
	sellerMeasureTerminationNotice,
	sellerMeasureTitleFallback,
	sellerMeasureTitleRisk,
	sellerMeasureTitleRiskFallback,
	sellerRevokeReason,
} from "@hawkr/ui-text/keys";
import type { ChipColor } from "@/components/ui/Chip";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";
import {
	appealBlockedReason,
	remediationBlockedReason,
	type SellerMeasure,
} from "@/lib/seller/measures";

/**
 * `/seller/measures` 的顯示文字。契約只給列舉，名詞與固定句型取自共用文案目錄（`@hawkr/ui-text`），純函式的第一個參數
 * 是 `UiText`；UNSPECIFIED 與沒有文字的值不是有效值，回 undefined，由呼叫端決定不顯示或用通用文字。
 * 生效與否、能不能申覆都讀伺服器給的 `state`、`canAppeal`，這裡不從時間或申覆推導。
 */

export const MEASURES_PATH = "/seller/measures";

/** 措施詳情頁的連結；停權公告的每張卡與列表都連到這裡。 */
export function measureHref(measure: Pick<SellerMeasure, "id">): string {
	return `${MEASURES_PATH}/${encodeURIComponent(measure.id)}`;
}

/** 申覆的獨立頁。 */
export function appealHref(measure: Pick<SellerMeasure, "id">): string {
	return `${measureHref(measure)}/appeal`;
}

/** 請求解除的獨立頁。 */
export function remediationHref(measure: Pick<SellerMeasure, "id">): string {
	return `${measureHref(measure)}/remediation`;
}

/**
 * 標題：違規依據顯示主要依據（第一筆認定）的類別；風險依據是「風險：{風險類型}」。沒有對照的用通用文字，與 seller-app 相同。
 */
export function measureTitle(
	ut: UiText,
	m: Pick<SellerMeasure, "basis" | "findings" | "riskGround">,
): string {
	switch (m.basis) {
		case MeasureBasis.VIOLATION: {
			const lead = m.findings[0];
			return (
				(lead && optionalText(ut, commonPolicyCategory(lead.category))) ??
				ut(sellerMeasureTitleFallback())
			);
		}
		case MeasureBasis.RISK:
			return ut(
				sellerMeasureTitleRisk({ ground: m.riskGround }) ??
					sellerMeasureTitleRiskFallback(),
			);
		case MeasureBasis.CONTRACT:
			return (
				optionalText(ut, sellerMeasureBasis(MeasureBasis.CONTRACT)) ??
				ut(sellerMeasureTitleFallback())
			);
		default:
			return ut(sellerMeasureTitleFallback());
	}
}

/**
 * 「嚴重程度：高 · 措施：停權」；嚴重程度只有認定才有，終止合作多一段「事由：{事由}」。沒有任何有效值時回 undefined。
 */
export function measureMeta(
	ut: UiText,
	m: Pick<SellerMeasure, "findings" | "kind"> &
		Partial<Pick<SellerMeasure, "terminationGround">>,
): string | undefined {
	const lead = m.findings[0];
	const severity = lead
		? optionalText(ut, sellerMeasureMetaSeverity({ severity: lead.severity }))
		: undefined;
	const kind = optionalText(ut, sellerMeasureMetaKind({ kind: m.kind }));
	const ground =
		m.kind === MeasureKind.TERMINATION && m.terminationGround !== undefined
			? optionalText(
					ut,
					sellerMeasureMetaTerminationGround({ ground: m.terminationGround }),
				)
			: undefined;
	const parts = [severity, kind, ground].filter((part): part is string =>
		Boolean(part),
	);
	return parts.length > 0 ? parts.join(" · ") : undefined;
}

function formatTimestamp(ts: Timestamp): string {
	return formatTaipeiDateTime(timestampDate(ts));
}

/**
 * 措施的期間。警告沒有期間（伺服器不設 `period`）。停權沒有結束時間表示由平台定期複審，不是永久。
 * 終止合作是「終止生效：{開始}」（預告終止在預告期間還沒到開始時間）；只有預告終止在店家關閉時才有結束時間，
 * 此時加「合作已於 {結束} 結束」。停權的期間句兩端（網頁與 App）的資訊設計不同，留在 client。
 */
export function measurePeriodText(
	ut: UiText,
	m: Pick<SellerMeasure, "period" | "kind">,
): string | undefined {
	const { period } = m;
	if (!period?.startTime) return undefined;
	const start = formatTimestamp(period.startTime);
	if (m.kind === MeasureKind.TERMINATION) {
		return ut(
			period.endTime
				? sellerMeasureTerminationEffectiveEnded({
						start,
						end: formatTimestamp(period.endTime),
					})
				: sellerMeasureTerminationEffective({ start }),
		);
	}
	if (period.endTime) {
		return `停權期間：${start} 至 ${formatTimestamp(period.endTime)}`;
	}
	return `停權期間：${start} 起，未定結束時間，平台定期複審`;
}

export interface StatusDisplay {
	label: string;
	color: ChipColor;
}

/** 措施狀態標籤的顏色；UNSPECIFIED 與沒有值的狀態沒有顏色，不顯示。終止合作生效後也是 ACTIVE，同樣用警示色。 */
function measureStateColor(state: MeasureState): ChipColor | undefined {
	switch (state) {
		case MeasureState.ISSUED:
		case MeasureState.SCHEDULED:
			return "warning";
		case MeasureState.ACTIVE:
			return "danger";
		case MeasureState.ENDED:
		case MeasureState.REVOKED:
			return "default";
		default:
			return undefined;
	}
}

/** 待處理的請求：`requests` 裡第一筆 PENDING（狀態是伺服器算好的生效狀態，client 不比對措施的狀態）。 */
function pendingRequest(
	m: Pick<SellerMeasure, "requests">,
): MeasureRequest | undefined {
	return m.requests.find((r) => r.state === MeasureRequestState.PENDING);
}

/**
 * 最新一筆已裁定的請求（申覆與解除請求合計）：`requests` 裡第一筆 ACCEPTED 或 REJECTED。待處理、CLOSED 與沒有值的
 * 不算：CLOSED 的原因已由措施的狀態表達。
 */
function latestDecidedRequest(
	m: Pick<SellerMeasure, "requests">,
): MeasureRequest | undefined {
	return m.requests.find(
		(r) =>
			r.state === MeasureRequestState.ACCEPTED ||
			r.state === MeasureRequestState.REJECTED,
	);
}

/**
 * 解除請求入口：能不能按讀伺服器的 `canRequestRemediation`；不能按時讀 `remediationUnavailabilities` 的第一筆：
 * 待處理或冷卻是反灰的按鈕並附伺服器的句子，NOT_RISK、NOT_ACTIVE（與沒有值）不顯示入口。client 不看措施的依據或狀態。
 */
export function remediationEntry(
	m: Pick<
		SellerMeasure,
		"canRequestRemediation" | "remediationUnavailabilities"
	>,
): { enabled: boolean; reason?: string } | undefined {
	if (m.canRequestRemediation) return { enabled: true };
	switch (m.remediationUnavailabilities[0]?.reason) {
		case RemediationUnavailableReason.REMEDIATION_PENDING:
		case RemediationUnavailableReason.APPEAL_PENDING:
		case RemediationUnavailableReason.COOLDOWN:
			return { enabled: false, reason: remediationBlockedReason(m) };
		default:
			return undefined;
	}
}

/** 請求狀態標籤：{種類}審核中、已通過、已駁回（目錄的 `with_kind` 變體）；CLOSED 與沒有值的沒有標籤。 */
function requestStatusDisplay(
	ut: UiText,
	request: MeasureRequest,
): StatusDisplay | undefined {
	const label = optionalText(
		ut,
		sellerMeasureRequestStateWithKind(request.state, { kind: request.kind }),
	);
	if (label === undefined) return undefined;
	switch (request.state) {
		case MeasureRequestState.PENDING:
			return { label, color: "warning" };
		case MeasureRequestState.ACCEPTED:
			return { label, color: "success" };
		case MeasureRequestState.REJECTED:
			return { label, color: "danger" };
		default:
			return undefined;
	}
}

/**
 * 狀態標籤：待處理的請求優先（申覆審核中、解除請求審核中），其次是最新一筆已裁定的結果（通過、駁回），再來才是措施本身的狀態。
 * 以 CLOSED 結案的請求不顯示標籤，回到措施狀態；兩邊都沒有有效值時回 undefined。
 */
export function measureStatusDisplay(
	ut: UiText,
	m: Pick<SellerMeasure, "requests" | "state" | "kind">,
): StatusDisplay | undefined {
	const pending = pendingRequest(m);
	if (pending) return requestStatusDisplay(ut, pending);
	const decided = latestDecidedRequest(m);
	if (decided) {
		const display = requestStatusDisplay(ut, decided);
		if (display) return display;
	}
	return measureStateDisplay(ut, m);
}

/** 措施本身的狀態標籤（終止合作生效後用「合作已終止」）；不看請求。 */
function measureStateDisplay(
	ut: UiText,
	m: Pick<SellerMeasure, "state" | "kind">,
): StatusDisplay | undefined {
	const color = measureStateColor(m.state);
	const label = optionalText(ut, sellerMeasureState(m.state, { kind: m.kind }));
	return color && label ? { label, color } : undefined;
}

/**
 * 卡片上並列的狀態標籤：有待處理的請求時，措施狀態標籤在前、請求標籤（申覆審核中、解除請求審核中）在後，
 * 兩個都顯示，不讓請求蓋掉措施狀態（與 seller-app 一致）；其他情況只有 `measureStatusDisplay` 的一個標籤。
 */
export function measureStatusChips(
	ut: UiText,
	m: Pick<SellerMeasure, "requests" | "state" | "kind">,
): StatusDisplay[] {
	const pending = pendingRequest(m);
	if (!pending) {
		const single = measureStatusDisplay(ut, m);
		return single ? [single] : [];
	}
	const state = measureStateDisplay(ut, m);
	const request = measureStatusDisplay(ut, m);
	return [state, request].filter((c): c is StatusDisplay => c !== undefined);
}

/**
 * 請求進度的說明行（一行）：待處理的請求顯示等候複核，否則顯示最新一筆已裁定請求的「平台回覆：{裁定理由}」
 * （申覆與解除請求共用同一種寫法，沒有裁定理由就沒有說明行）。CLOSED 的請求沒有說明；冷卻時間只出現在解除請求按鈕的反灰原因。
 */
export function requestNote(
	ut: UiText,
	m: Pick<SellerMeasure, "requests">,
): string | undefined {
	const pending = pendingRequest(m);
	if (pending) {
		return optionalText(
			ut,
			sellerMeasureRequestKindSubmittedNote(pending.kind),
		);
	}
	const decided = latestDecidedRequest(m);
	return decided?.decisionReason
		? ut(sellerMeasureRequestReplyNote({ reason: decided.decisionReason }))
		: undefined;
}

/** 賣家端需要說明為什麼被撤銷的原因：認定被撤銷、關聯的終止被撤銷；接受申覆已由申覆狀態說明，管理員更正、外部決定與其他不另說明。 */
function isRevokeNoteReason(reason: RevokeReason): boolean {
	return (
		reason === RevokeReason.FINDING_REVOKED ||
		reason === RevokeReason.SOURCE_REVOKED
	);
}

/** 撤銷原因的說明行；只有認定與關聯終止被撤銷的原因需要告訴賣家為什麼。 */
export function revokeNote(
	ut: UiText,
	m: Pick<SellerMeasure, "revokeReason" | "state">,
): string | undefined {
	return m.state === MeasureState.REVOKED && isRevokeNoteReason(m.revokeReason)
		? optionalText(ut, sellerRevokeReason(m.revokeReason))
		: undefined;
}

/**
 * 預告終止的橫幅文字（預定生效的終止合作，`ListMyMeasures(state=SCHEDULED)`）：生效時間讀伺服器的 `period.startTime`；
 * 不是終止合作或沒有開始時間回 undefined，不顯示。
 */
export function terminationNoticeText(
	ut: UiText,
	m: Pick<SellerMeasure, "kind" | "state" | "period">,
): string | undefined {
	if (m.kind !== MeasureKind.TERMINATION) return undefined;
	if (m.state !== MeasureState.SCHEDULED) return undefined;
	const start = m.period?.startTime;
	if (!start) return undefined;
	return ut(sellerMeasureTerminationNotice({ start: formatTimestamp(start) }));
}

/**
 * 申覆入口：能不能用讀伺服器的 `canAppeal`；一律顯示，不能用時反灰並附伺服器原因的第一筆句子（`appealUnavailabilities`，
 * client 不從 state 或 requests 推導）。終止合作另附「申覆期限：{時間}」（伺服器的 `appealDeadlineTime`，期限內與期限後都顯示）。
 */
export function appealEntry(
	ut: UiText,
	m: Pick<
		SellerMeasure,
		"canAppeal" | "appealUnavailabilities" | "kind" | "appealDeadlineTime"
	>,
): { enabled: boolean; reason?: string; deadline?: string } {
	const deadline =
		m.kind === MeasureKind.TERMINATION && m.appealDeadlineTime
			? ut(
					sellerMeasureAppealDeadline({
						deadline: formatTimestamp(m.appealDeadlineTime),
					}),
				)
			: undefined;
	return m.canAppeal
		? { enabled: true, deadline }
		: { enabled: false, reason: appealBlockedReason(m), deadline };
}

const SECONDS_PER_DAY = 86400;

/**
 * 送出前的冷卻警告：`rejectionCooldown` 是這筆措施的下一個請求若被駁回會開始的冷卻（伺服器決定，申覆與解除請求的駁回
 * 都計入）；不設就沒有警告。天數取整天（不足一天進位為一天，不會少報）。
 */
export function rejectionCooldownWarning(
	ut: UiText,
	measure: Pick<SellerMeasure, "rejectionCooldown">,
): string | undefined {
	const cooldown = measure.rejectionCooldown;
	if (!cooldown) return undefined;
	const seconds = Number(cooldown.seconds) + cooldown.nanos / 1e9;
	const days = Math.max(1, Math.ceil(seconds / SECONDS_PER_DAY));
	return ut(sellerMeasureRejectionCooldownWarning({ days }));
}

export const MEASURE_LOAD_FAILED_TITLE = "無法載入措施紀錄";
export const MEASURE_LOAD_FAILED_DESCRIPTION = "請檢查網路後再試一次。";
export const MEASURE_NOT_FOUND_TITLE = "找不到這項措施";
export const MEASURE_NOT_FOUND_DESCRIPTION =
	"這項措施不存在，或不屬於目前的帳號。";
export const MEASURE_NOT_FOUND_ACTION = "回到措施與申覆";
