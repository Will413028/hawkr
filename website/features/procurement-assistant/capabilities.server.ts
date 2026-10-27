import "server-only";

import {
	type AgentCapabilities,
	parseAgentCapabilities,
	WEB_AGENT_FLAGS,
} from "./capabilities";

/**
 * Hawkr Agent capability flags 的 server 端解析（設計書 v6 §V.13 規則 6）。
 *
 * 規則 6 明說 `NEXT_PUBLIC_*` 是 build-time 值、不是即時 kill switch，Web
 * capability「應由 Server Component／bootstrap 或受控 runtime config 傳入，
 * 並有短 cache 與失效策略」。所以這裡讀的是**沒有 NEXT_PUBLIC_ 前綴**的 env：
 * 那些值不會被 Next 內聯進 client bundle，而是每次 server render 現讀。
 *
 * ⚠️ 誠實的界線：env 仍需要重啟 process 才會變，所以這一版**還不是**真正的
 * 即時 kill switch，只是把值移出了 client bundle、讓 rollback 不必重新 build。
 * 真正的受控 runtime config（帶短 cache 與失效）是後續工作；接上時只要換掉這
 * 支函式的內容，呼叫端與 `AgentCapabilities` 型別都不用動。
 */
export function readAgentCapabilities(): AgentCapabilities {
	return parseAgentCapabilities(
		Object.fromEntries(
			WEB_AGENT_FLAGS.map((flag) => [flag, process.env[flag]]),
		),
	);
}
