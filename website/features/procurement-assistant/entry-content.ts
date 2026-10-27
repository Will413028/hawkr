import type { AgentCapabilities } from "./capabilities";
import type { HawkrAgentCapabilityIconName } from "./HawkrAgentCapabilityIcon";

/**
 * Hawkr Agent 新對話中央入口的文案與起手內容（設計書 v6 §V.11.0 Screen
 * Contract 00）。
 *
 * 抽成資料表而不是寫死在 JSX 裡，是為了讓 §V.11.0 的兩條硬規則可以被測：
 * 「Slice A 只使用『找供應商、整理採購需求、比較候選』」與「未啟用的能力
 * 完全不出現」。清單是資料，contract test 才能斷言它剛好三筆、而且沒有任何
 * 一筆會自動送出。
 */

/** §V.11.0：唯一 h1。 */
export const ENTRY_HEADING = "今天想請 Hawkr 幫你完成什麼？";

/** §V.11.0：h1 底下的一行說明。 */
export const ENTRY_SUBHEADING =
	"用一句話描述你要的品項、數量與到貨時間，Hawkr 會幫你把條件整理好，再去找可能的供應來源。";

/** §V.11.0 建議 placeholder，逐字採用。 */
export const ENTRY_COMPOSER_PLACEHOLDER =
	"例如：幫我找能在三週後配送到台北、供應 50 公斤台灣豬五花的廠商";

export interface AgentCapabilityChip {
	id: string;
	label: string;
	/** 可選：只有已完成視覺與動作驗證的能力才配置。 */
	icon?: HawkrAgentCapabilityIconName;
	/** 按鈕內的短說明。 */
	description?: string;
	/**
	 * 點擊後帶入 composer 的可編輯範本。不會自動送出。
	 *
	 * 這裡放的是完整例句而不是半截開頭：§V.11.0 原本把「chip 帶 prompt
	 * starter」與「起手卡帶可編輯範本」拆成兩排，但兩排的三個項目其實是同一組
	 * （找供應商／整理採購需求／比較候選），下面那排是重複的，已依產品端決定
	 * 移除。範本因此併到 chip 上——留半截開頭反而讓使用者接不下去。
	 */
	template: string;
}

/**
 * §V.11.0：Slice A 只顯示這三個 chip。
 *
 * 市場價格、報價比較、追蹤與對外聯絡都還沒有真實能力，設計書要求「未啟用時
 * 完全不出現」——所以這張表就只有三筆，而不是有一張完整清單再靠旗標過濾。
 * 需要靠旗標顯示的能力請放進 `flaggedCapabilityChips()`，那裡才有 gate。
 */
export const SLICE_A_CAPABILITY_CHIPS: readonly AgentCapabilityChip[] =
	Object.freeze([
		{
			id: "find-suppliers",
			label: "找供應商",
			icon: "find-suppliers",
			template:
				"我要找能在三週後配送到台北、供應 50 公斤台灣豬五花的廠商，預算每公斤 220 元以內。",
		},
		{
			id: "organise-requirements",
			label: "整理採購需求",
			icon: "organise-requirements",
			template:
				"我要採購水果禮盒 50 箱，希望在三週後送到桃園市，每箱預算 800 元，幫我把條件整理成完整的採購需求。",
		},
		{
			id: "compare-candidates",
			label: "比較候選",
			icon: "compare-candidates",
			template:
				"幫我比較幾家能供應有機蔬菜的商家，我最在意產銷履歷、到貨穩定度和單價。",
		},
	]);

/**
 * 需要 capability flag 才顯示的 chip。
 *
 * 目前只有「看市場行情」掛在 `WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1` 之後；
 * 預設 OFF 時 Entry 仍嚴格只有使用者定稿的三個 illustrated buttons。這個未配
 * illustration 的 future chip 不可在旗標前露出；後端前置（#2403／#2442／
 * Pricebook visibility ADR）未完成時也不得靠 client 探測自行開啟。
 */
export function flaggedCapabilityChips(
	capabilities: AgentCapabilities,
): readonly AgentCapabilityChip[] {
	const chips: AgentCapabilityChip[] = [];
	if (capabilities.WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1) {
		chips.push({
			id: "market-price",
			label: "看市場行情",
			template:
				"我想先看看目前的行情價：台灣豬五花 50 公斤，三週後送到台北，大概行情多少？",
		});
	}
	return Object.freeze(chips);
}
