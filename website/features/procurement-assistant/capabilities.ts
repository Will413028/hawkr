/**
 * Hawkr Agent 的 Web capability flags（設計書 v6 Part V §V.13）。
 *
 * 四支旗標各自對應一個 delivery slice，**預設全部 OFF**。規則出自 V.13：
 *
 * - 規則 2「Backend capability 先開、Web flag 後開」：這裡只描述 Web 端要不要
 *   渲染新表面，不代表後端契約已存在。旗標開了但後端沒給 typed event，畫面
 *   必須落回既有 renderer，不得自行探測 contract。
 * - 規則 5「Security 與 Quote invariant 沒有『回到舊不安全行為』的 rollback」：
 *   關掉旗標只會隱藏新入口，不會把 raw reasoning、free-form publish 這類已被
 *   移除的行為放回來。
 * - 規則 6「NEXT_PUBLIC_* 是 build-time 值，不是即時 kill switch」：所以這個
 *   模組**不讀 process.env**。值由 Server Component 解析後往下傳（見
 *   `capabilities.server.ts`），client 只消費一個純資料物件。
 *
 * 之所以不做成 React context：entry / thread / rail 都在同一棵子樹裡，prop
 * 傳遞就夠了，而 context 會讓「這個畫面到底看哪一支旗標」變得不可追蹤。
 */

/** V.13 表格中 scope 為 Web 的四支旗標。順序＝設計書表格順序。 */
export const WEB_AGENT_FLAGS = [
	/** snapshot／reducer／replay transport（回退：legacy durable procurement UI，不回 seller-match 入口）。 */
	"WEB_PROCUREMENT_AGENT_RUNTIME_V1",
	/** assistant-ui／Task／Tool／artifact renderer（回退：保留 canonical runtime，退回既有 renderer）。 */
	"WEB_PROCUREMENT_AGENT_UI_V1",
	/** 市場參考、詢價上下限與 update-trial flow。 */
	"WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1",
	/** Web #489／P0-13C Quote companion。 */
	"WEB_PROCUREMENT_RFQ_APPROVAL_V1",
] as const;

export type WebAgentFlag = (typeof WEB_AGENT_FLAGS)[number];

export type AgentCapabilities = Readonly<Record<WebAgentFlag, boolean>>;

/**
 * 預設值＝全 OFF。
 *
 * 這個常數同時是 fail-safe：解析失敗、Server Component 沒傳、或 client 元件在
 * 測試裡單獨掛載時，一律得到「什麼新表面都不開」，而不是繼承上一次的值。
 */
export const AGENT_CAPABILITIES_OFF: AgentCapabilities = Object.freeze(
	Object.fromEntries(WEB_AGENT_FLAGS.map((flag) => [flag, false])),
) as AgentCapabilities;

/**
 * 只有字串 `"true"` 算開。
 *
 * 刻意不接受 `"1"`／`"on"`／`"yes"`：旗標的開關是 runbook 動作，寬鬆解析只會
 * 讓「我明明設了」與「它其實沒開」變成一場考古。`"false"`、空字串、未設定與
 * 任何其他值都是 OFF，與 `seoIndexable` 的判定方式一致。
 */
export function isFlagEnabled(raw: string | undefined | null): boolean {
	return raw?.trim() === "true";
}

/**
 * 把任意來源（env、runtime config endpoint、測試 fixture）收斂成完整的
 * capability 物件。未列出的旗標一律補 false——新增旗標時舊的 config 來源不會
 * 讓它意外變成 `undefined` 而在 `if (caps.X)` 之外的地方（例如 `!==false`）
 * 誤判為開。
 */
export function parseAgentCapabilities(
	source: Partial<Record<string, string | boolean | undefined>> = {},
): AgentCapabilities {
	return Object.freeze(
		Object.fromEntries(
			WEB_AGENT_FLAGS.map((flag) => {
				const raw = source[flag];
				return [
					flag,
					typeof raw === "boolean" ? raw : isFlagEnabled(raw ?? undefined),
				];
			}),
		),
	) as AgentCapabilities;
}
