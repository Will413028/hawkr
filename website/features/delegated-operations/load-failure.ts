/**
 * 代行「讀取失敗」的分類（§6.5.1 封閉碼表在 www 這一側的落點）。
 *
 * **為什麼需要它**：代行 BFF 一律不透傳 message，`delegatedFetch` 因此把每一次失敗
 * 都包成同一句字串 `代行請求失敗（HTTP {status}／代碼 {code}）`。把那句話直接畫進
 * 畫面，會同時做錯三件事：
 * 1. **9275（找不到／不屬於本店）與「上游暫時掛掉」被畫成同一件事**。前者重試一萬次
 *    都不會有結果，後者只要再按一次就好 —— 共用一個畫面等於兩邊都拿不到正確動作。
 * 2. **把 HTTP 狀態碼與內部技術碼印給使用者**。它們既不是他能處理的資訊，也不穩定：
 *    9280 正在從 502 改成 424，任何以狀態碼為錨的判斷都會在那天靜默改變意義。
 * 3. 沒有出口。畫面說「失敗」，卻沒有任何一顆按鈕能讓人離開那個狀態。
 *
 * ⚠️ **判別一律以 envelope 的業務碼為準，不看 HTTP status**（上面第 2 點）。
 *
 * ⚠️ 終態碼（9270／9271／9272）刻意不列在這裡：工作階段終止是**整個工作區**的事，由
 * `DelegatedWorkspaceFrame` 切阻擋畫面處理，不是某一頁的讀取失敗。它們落到 `temporary`
 * 這條分支上是安全的 —— 30 秒內的下一次 `GET current` 會把整個工作區換掉。
 */

/** 資源不屬於本店，**或根本不存在**。Gateway 刻意不區分兩者（不給枚舉他店資源的訊號）。 */
export const DELEGATION_RESOURCE_NOT_OWNED = 9275;
/** 本次工作階段沒有這條端點需要的 capability。 */
export const DELEGATION_CAPABILITY_DENIED = 9262;

export type DelegatedLoadFailureKind = "not-owned" | "denied" | "temporary";

/**
 * 從任何拋出物取出業務碼。
 *
 * 刻意 duck-type 而不是 `instanceof ApiClientError`：BFF 自己的失敗碼是**字串**
 * （`UPSTREAM_FAILURE`），型別上宣告成 `number` 但執行期不是，而 `instanceof` 在
 * 跨模組複本／重新拋出的情境下也不可靠。這裡只要那個值本身。
 */
export function delegatedFailureCode(error: unknown): number | string | null {
	if (typeof error !== "object" || error === null) return null;
	const code = (error as { code?: unknown }).code;
	return typeof code === "number" || typeof code === "string" ? code : null;
}

export function classifyDelegatedLoadFailure(
	error: unknown,
): DelegatedLoadFailureKind {
	switch (delegatedFailureCode(error)) {
		case DELEGATION_RESOURCE_NOT_OWNED:
			return "not-owned";
		case DELEGATION_CAPABILITY_DENIED:
			return "denied";
		default:
			// 逾時、斷線、UPSTREAM_FAILURE、以及任何還沒被這張表認得的碼。
			// 預設倒向「可重試」是刻意的：把暫時性故障誤講成「找不到」，會讓操作者
			// 相信目標店家真的沒有這筆資料 —— 那是本次六個 P1 的共同病灶。
			return "temporary";
	}
}

export interface DelegatedLoadFailureCopy {
	title: string;
	hint: string;
	/** 畫不畫「重新載入」。`false` 時重試必然再失敗，畫了就是騙人再按一次。 */
	retryable: boolean;
}

/**
 * 批次逐項結果的業務碼 → 操作者看得懂的一句話。
 *
 * **為什麼不能直接印碼**：批次列曾寫「2 筆中 0 成功、2 失敗（876659813789762327：
 * 代碼 9260、872854281232616611：代碼 9260）」。`9260` 對操作者不是資訊，他既無從
 * 判斷該重試、該重新載入、還是該回 ADM；而 `9276`／`9266`／`9262` 三個碼要求的下一步
 * 完全不同。碼表本身是 `docs/代碼管理（CodeMeta）/錯誤碼.md` 的代行區段（9260~9289）。
 *
 * ⚠️ 未登錄的碼**照原樣帶出碼值**（`代碼 {code}`），不編一句像模像樣的話：批次是唯一
 * 拿得到逐項碼的路徑，把陌生碼吞成「未知錯誤」會讓新碼在畫面上完全無跡可循。
 */
const DELEGATED_BATCH_FAILURE_TEXT: Record<number, string> = {
	9260: "缺少必填欄位或版本，這一筆沒有送出",
	9262: "本次工作階段沒有這項操作權限",
	9266: "賣家或另一位管理員已改過它，版本不符",
	9273: "操作過於頻繁，稍後再試這一筆",
	9275: "這筆資料不屬於本次代行的店家",
	9276: "整批未執行",
	9279: "稽核紀錄寫入失敗，這一筆沒有執行",
	9280: "代行服務暫時無法使用",
};

export function delegatedBatchFailureText(code: number): string {
	return DELEGATED_BATCH_FAILURE_TEXT[code] ?? `代碼 ${code}`;
}

/** 商品讀取失敗的三種畫面。`denied` 不在這裡 —— 它走共用的授權空狀態。 */
export function delegatedProductLoadFailureCopy(
	kind: Exclude<DelegatedLoadFailureKind, "denied">,
): DelegatedLoadFailureCopy {
	if (kind === "not-owned") {
		return {
			title: "找不到這個商品",
			hint: "這個商品不存在，或不屬於本次代行的目標店家。代行只能操作工作階段綁定的那一家店；若是從別的分頁或書籤進來的，請回到商品管理重新選一次。",
			retryable: false,
		};
	}
	return {
		title: "暫時無法載入這個商品",
		hint: "代行服務目前沒有回應，商品資料沒有任何變動。請稍後重新載入；若持續失敗，回到商品管理或返回 ADM 重新建立工作階段。",
		retryable: true,
	};
}
