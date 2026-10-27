/**
 * 代行執行期旗標（56 號 §4.5 L1051／定案第 1 條 ③）。
 *
 * 代行工作區沿用賣家中心的元件，而那些元件的資料層原本一律打 `/app/*`。切換
 * 漏掉任何一支，最壞的後果**不是壞掉而是成功**：管理員本人恰好也是賣家、且在
 * 同一個瀏覽器登入過 www 時，`/app/*` 會用他自己的身分成功執行 —— 操作到他自己
 * 的店，零稽核零告警。
 *
 * 所以旗標為真時 `dispatch()` 的 `mode === "authed"` 分支**立刻 throw、不發請求**：
 * 漏切的元件會在工作區當場炸出明確錯誤，而不是靜默打到別人的店。
 *
 * ⚠️ 掛在 `dispatch()` 不是 `authedFetch`：`apiClient.authed.*` 走同一個 dispatch
 * 卻不經過 `authedFetch`，實查有 15 個生產檔案直接用它。只守 `authedFetch` 等於
 * 留一扇開著的門。
 *
 * ⚠️ **只在瀏覽器生效**。這是模組級變數，在伺服器上它是跨請求共用的：由 SSR 設
 * 起來會讓同一個 isolate 內其他人的賣家請求一起 throw。代行工作區的資料存取全
 * 部發生在瀏覽器，伺服器側由「www host 上沒有 ADM 憑證」這個結構性事實兜底。
 */

let delegatedRuntime = false;

/** 進入 `/adm` 工作區時開啟；回傳的函式在離開時關閉（一定要呼叫）。 */
export function enterDelegatedRuntime(): () => void {
	if (typeof window === "undefined") return () => undefined;
	delegatedRuntime = true;
	return () => {
		delegatedRuntime = false;
	};
}

export function isDelegatedRuntime(): boolean {
	return delegatedRuntime;
}

export class DelegatedRuntimeTransportError extends Error {
	constructor(path: string) {
		super(
			`代行工作區禁止呼叫賣家端點（${path}）：這個資料層還沒切到代行 BFF（/adm/api/*）。`,
		);
		this.name = "DelegatedRuntimeTransportError";
	}
}
