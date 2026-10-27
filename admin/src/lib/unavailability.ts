import type { LocalizedMessage } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { StoreActionUnavailableReason } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";

// 伺服器給的「此刻不能做」原因的呈現規則（前提 2026-10-15：能不能做與為什麼不能都來自伺服器，client 只呈現）。
// 措施的原因（MeasureActionUnavailability）與店家動作的原因（StoreActionUnavailability）共用：兩者都是 { reason, message }。

/** 伺服器說不能操作、但沒有給文字（原因清單空，或該原因沒有 message）時的文字：仍然相信伺服器的 false。 */
export const GENERIC_UNAVAILABLE_LABEL = "目前不能操作";

/** 一個不能做的原因：只用到伺服器的句子，原因的值只用來分支（見 firstIsStructural）。 */
interface Unavailability {
	message?: LocalizedMessage;
}

/**
 * 伺服器給的原因（依優先序，可以有多個）的文字：直接顯示每個原因的 `message.message`（錯誤模型 ADR 的 client 規則：有文字就顯示，沒有才顯示通用文字），
 * 不另建原因對照表；保持順序、相同的文字只列一次。原因是空的而動作不能做（伺服器的 can_* 是 false）時也回通用文字。呼叫端只在 can_* 是 false 時使用。
 * 一次可以傳入多個動作的清單：依序合併。
 */
export function unavailableLabels(
	...lists: ReadonlyArray<readonly Unavailability[]>
): string[] {
	const labels: string[] = [];
	for (const list of lists) {
		for (const item of list) {
			const text = item.message?.message;
			const label = text ? text : GENERIC_UNAVAILABLE_LABEL;
			if (!labels.includes(label)) labels.push(label);
		}
	}
	return labels.length > 0 ? labels : [GENERIC_UNAVAILABLE_LABEL];
}

/**
 * 哪些店家動作的原因「表示沒有入口」：這個分類是伺服器的語意，SSOT 是契約（`StoreActionUnavailableReason` 每個值的註解），這裡照契約實作。
 * 寫成窮舉的 Record：契約新增值時 `tsc` 會要求在這裡決定。UNSPECIFIED 不是伺服器會給的值，當成有入口（顯示通用文字）。
 */
const NO_ENTRY: Record<StoreActionUnavailableReason, boolean> = {
	[StoreActionUnavailableReason.UNSPECIFIED]: false,
	[StoreActionUnavailableReason.KYC_NOT_SUBMITTED]: true,
	[StoreActionUnavailableReason.KYC_NOT_BUSINESS]: true,
	[StoreActionUnavailableReason.PAYUNI_MERCHANT_TERMINAL]: false,
	[StoreActionUnavailableReason.PAYUNI_BIND_IN_PROGRESS]: true,
	[StoreActionUnavailableReason.PAYUNI_BOUND]: true,
	[StoreActionUnavailableReason.FOOD_REGISTRATION_NOT_PENDING]: true,
	[StoreActionUnavailableReason.KYC_NOT_APPROVED]: false,
	[StoreActionUnavailableReason.STORE_CLOSED]: false,
	[StoreActionUnavailableReason.STORE_PENDING_DELETION]: false,
	[StoreActionUnavailableReason.ACCOUNT_SUSPENDED]: false,
};

/** 伺服器契約標明「表示沒有入口」的原因值（給測試對照契約註解）。 */
export const NO_ENTRY_REASONS: readonly StoreActionUnavailableReason[] = (
	Object.keys(NO_ENTRY) as unknown as StoreActionUnavailableReason[]
)
	.map(Number)
	.filter((reason) => NO_ENTRY[reason as StoreActionUnavailableReason]);

/** 依先後，第一個原因是結構性的（沒有入口）。清單是空的回 false。 */
export function firstIsStructural(
	list: ReadonlyArray<{ reason: StoreActionUnavailableReason }>,
): boolean {
	const first = list[0];
	return first !== undefined && NO_ENTRY[first.reason] === true;
}

/**
 * 店家動作不能做時要顯示的那一句：伺服器列出全部原因（依先後），畫面只顯示第一個，與 PayUni 卡、代行清單一致。清單空（可以做）或第一個原因是結構性的
 * （沒有入口）時沒有句子（undefined）；其餘是第一個原因的伺服器句子，沒有 message 才是通用文字。
 */
export function firstVisibleLabel(
	list: ReadonlyArray<{
		reason: StoreActionUnavailableReason;
		message?: LocalizedMessage;
	}>,
): string | undefined {
	if (list.length === 0 || firstIsStructural(list)) return undefined;
	return unavailableLabels(list)[0];
}
