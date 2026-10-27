/**
 * 後端 envelope 的 typed error，帶 code + traceId。獨立放在 dep-free 模組（不放
 * api.ts，因為 api.ts 是 "use client"）— 讓 Sentry server/edge config 能透過
 * trace-tag import 它，而不把 client 程式碼拖進 server bundle。
 */
export class ApiClientError extends Error {
	constructor(
		message: string,
		readonly code?: number,
		readonly traceId?: string,
		/**
		 * envelope 的 `data`。**非零業務碼也可能帶 data**：58 §6.1.0 (13) 有一批標
		 * `preserveData` 的碼，HTTP status 恆為 200 而 `code !== 0`，價值全在 data 裡
		 * （`documentId` / `unlockId` / `providerRequestId` / `intakeLimit`）。
		 *
		 * 這些碼在 `api.ts` 一律走 throw（見該檔 `code !== 0` 分支）⇒ 呼叫端只能從
		 * 例外物件上拿到它們。少了這個欄位，症狀不是報錯而是**按鈕點了沒反應**：
		 * 前端拿不到要導去的 documentId，於是什麼都不做。
		 *
		 * 型別是 `unknown` 而非具體形狀：每個碼的 data 形狀都不同，收斂成一個
		 * union 會逼所有呼叫端認識全部的碼。呼叫端自己 narrow。
		 */
		readonly data?: unknown,
	) {
		super(message);
		this.name = "ApiClientError";
	}
}
