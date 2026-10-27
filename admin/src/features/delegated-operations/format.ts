import { formatDateTime } from "@/lib/format";
import type { DelegatedSessionStatus } from "./governance";
import { DELEGATED_SESSION_STATUS_LABELS } from "./labels";

/**
 * 代行治理台的時間呈現。
 *
 * 後端一律回 RFC 3339 UTC（`…Z`），治理台一律以瀏覽器本地時區顯示。兩邊各寫各的，
 * 同一筆紀錄就會在表格上是 `2026/8/15 08:03:00`、在右側稽核內容面板上是
 * `2026-08-14T23:03:00Z` —— 讀的人會以為那是兩件事，而稽核頁的讀者正是在比對時序。
 */
export function formatClockTime(iso: string | null): string {
	if (!iso) return "—";
	// 台北釘死（backend#3053 的稽查）：代行營運的時間軸是稽核用的，
	// 讀的人在哪個時區都必須看到同一個值。
	return formatDateTime(iso);
}

/**
 * `<input type="datetime-local">` 給的是本地時間、無時區；後端只收 RFC 3339 UTC，而且
 * 前端 parser 最多接受三位小數（`parseTimestamp`），所以毫秒要切掉。
 *
 * 兩個治理畫面（單一工作階段的稽核紀錄、跨工作階段的工作階段列表）共用同一支：各寫
 * 一份的話，漂移的症狀是「同一個時間在兩個畫面上篩出不同的區間」，而兩邊各自單看
 * 都正確。空字串代表沒有選，原樣回空字串（不篩選）。
 */
export function localInputToUtcInstant(local: string): string {
	if (!local) return "";
	const parsed = new Date(local);
	return Number.isNaN(parsed.getTime())
		? ""
		: parsed.toISOString().replace(/\.\d{3}Z$/u, "Z");
}

/**
 * 「稽核狀態」那一列的文案。**三態，不是兩態。**
 *
 * ① 有未確認 → 說幾筆。寫死成全綠，操作者就永遠不會在他最該對帳的那一刻知道
 *    有結果未確認。
 * ② 一筆紀錄都沒有 → 不能說「均已確認」。那是對空集合的全稱宣稱：邏輯為真，
 *    讀起來卻像「做過的事都對過帳了」，而實際上什麼都沒做。撤銷一個未動手的
 *    工作階段時（`共 0 筆紀錄` 的畫面）正是此態。
 * ③ 其餘 → 確實有紀錄且都已確認。
 *
 * `failedCount` 來自第二支查詢，可能還沒回來。成功或未確認任一非零就已經證明
 * 「有紀錄」、不必等它；兩者皆 0 時才真的要等，而**那時的未知不可以當成零**
 * —— 把載入中誤讀成結論，正是這一列最初就犯的錯。
 *
 * ⚠️ 措辭刻意避開「寫入」：同一頁表格的「變更摘要」欄用「未寫入」表示**領域寫入
 * 沒有發生**，主語不同。兩個「寫入」並陳時讀者無從分辨哪個是哪個，而這一列問的是
 * 「稽核列的結果確認了沒」，與本函式 ① 分支的「未確認」是同一組詞彙。
 */
export function auditConfirmationText(
	counts: {
		operationCount: number;
		pendingAuditCount: number;
	},
	failedCount: number | null,
): string {
	if (counts.pendingAuditCount > 0) {
		return `${counts.pendingAuditCount} 筆結果未確認`;
	}
	if (counts.operationCount > 0) return "所有結果均已確認";
	if (failedCount === null) return "正在確認稽核紀錄";
	return failedCount > 0 ? "所有結果均已確認" : "本次沒有任何操作紀錄";
}

/**
 * 「結束時間」那一列的文案。
 *
 * 「尚未結束」只有在工作階段真的還在進行時才成立。修前的 `RevokeSession` 只寫
 * `revoked_at`、不寫 `ended_at`，所以已撤銷的工作階段回來的 `endedAt` 是 `null`
 * —— 照字面印就會在右上角掛著「已撤銷」的同一個畫面上說它「尚未結束」，兩句話直接
 * 對打。後端那句 CAS 現在會一併寫 `ended_at`，但這道判斷仍然必要：修復前留下的列
 * 照樣是 `null`，而到期還沒被 sweep 的工作階段本來就沒有人替它記過結束時刻。
 *
 * 因此以 `status` 為準：終態不得宣稱未結束；時刻缺席就說缺席，**不**從
 * `issuedAt + durationSeconds` 反推一個沒有人記錄過的時間（那個 duration 在
 * `endedAt` 為空時是以「現在」算的，每次重新整理都會變）。
 */
export function sessionEndText(
	endedAt: string | null,
	status: DelegatedSessionStatus,
): string {
	if (endedAt) return formatClockTime(endedAt);
	if (status === "active") return "尚未結束";
	return `${DELEGATED_SESSION_STATUS_LABELS[status]}（未記錄時刻）`;
}

/**
 * 「撤銷紀錄」那一塊的第一行：什麼時候、誰撤的。
 *
 * 三段拼接的規則寫在這裡而不是散在 JSX：JSX 的 `{expr}{" "}` 換行會自己補空白，而
 * **全形括號本身已經帶了半形寬的視覺留白** —— 兩者疊起來就是「（2661027599） 撤銷」
 * 那個看起來像少了一個字的空隙。純函式讓這條排版規則有唯一判定處，也測得到。
 *
 * 兩個 fallback 分開處理，因為缺的東西不同：
 *   - 顯示名缺席（後端與 `admin_accounts` 的 join 落空）→ 仍知道是誰，印出 admin id
 *   - 兩者都缺 → 只能說沒記錄；**不得**省略整句，那會讓「誰撤的」看起來從未被問過
 *
 * ⚠️ `displayName` 填的是 email（`admin_accounts` 沒有顯示名欄位），呼叫端的標籤
 * 不得寫「姓名」。
 */
export function sessionRevokedByText(
	revokedAt: string,
	revokedByAdminDisplayName: string | null,
	revokedByAdminId: string | null,
): string {
	const name = revokedByAdminDisplayName;
	const id = revokedByAdminId;
	// 全形括號結尾後不再補半形空格；純西文/中文結尾才補。
	const actor =
		name && id
			? `${name}（${id}）撤銷`
			: name
				? `${name} 撤銷`
				: id
					? `管理員（${id}）撤銷`
					: "未記錄操作者撤銷";
	return `${formatClockTime(revokedAt)} 由 ${actor}`;
}

function formatDuration(seconds: number): string {
	const minutes = Math.floor(seconds / 60);
	const rest = seconds % 60;
	return minutes > 0 ? `${minutes} 分 ${rest} 秒` : `${rest} 秒`;
}

/**
 * 「工作時長」那一列的文案。與 `sessionEndText` 同一個前提，因此必須同一時間修。
 *
 * 後端的 `durationSeconds` 是 `LEAST(ended_at, 閒置逾時點, expires_at, now) − issued_at`
 * （account-service `usecase/summary.go`），只有右端落到 `now` 時它才是會長大的值。哪
 * 一種狀態會落到 `now`，從 `status` 就推得出來：
 *   active  —— 本來就該取 now，這一列問的正是「目前已經進行多久」。
 *   expired —— 代表兩支到期述詞至少一支已成立，右端必在過去，值固定。
 *   revoked —— 撤銷不讓 `expires_at` 提前，右端只能由 `ended_at` 提供；沒有它，三個
 *              位於過去的候選一個都接不到，只剩 now。
 *
 * 所以唯一不可信的組合是「已撤銷且沒有結束時刻」：那個秒數每重新整理一次就長一點，
 * 而同一張卡的「結束時間」已經因為同一個前提改口說「已撤銷（未記錄時刻）」。一列拒絕
 * 反推、另一列照印，畫面就自己打自己 —— 而讀的人正拿這張卡當舉證。
 *
 * 撤銷路徑現在會一併寫 `ended_at`（`RevokeSession` 的單句 CAS），因此這是 fail-closed
 * 的守門而不是常態分支：它接住的是修復前留下的舊列，以及後端哪天又把那一半拿掉。
 */
export function sessionDurationText(
	durationSeconds: number,
	endedAt: string | null,
	status: DelegatedSessionStatus,
): string {
	if (status === "revoked" && endedAt === null) {
		return "未知（撤銷未記錄結束時刻）";
	}
	return formatDuration(durationSeconds);
}

/**
 * 授權變更歷程的「這一次到底動了什麼」。
 *
 * 修前那一欄把 before 與 after 兩份**完整**清單並排印出來。能力共九項、中文名 4–7 字，
 * 一次全開的列光 after 就約 50 字，連同 before 與八個頓號超過 110 字 —— 折成四五行，
 * 而同列其餘欄位只有一兩行（`.adm-audit-table td` 是 `vertical-align: top`，列高取最高
 * 者），整張表高度參差；更要命的是**集合減法丟給讀的人自己做**，而這張表存在的理由
 * 正是「誰在什麼時候把範圍改成什麼樣」。
 *
 * 只回原始碼值，翻譯留給呼叫端：`delegationCapabilityLabel` 對後端新增而 ADM 還沒跟上
 * 的能力會退回機器名，那是顯示層的取捨，不該讓集合運算跟著承擔。
 *
 * 順序沿用來源陣列（後端下發的順序），不另行排序 —— 排序會讓同一批能力在「新增」與
 * 「完整清單」兩處以不同順序出現。
 */
export function delegationScopeDelta(
	before: readonly string[],
	after: readonly string[],
): { added: string[]; removed: string[] } {
	const beforeSet = new Set(before);
	const afterSet = new Set(after);
	return {
		added: after.filter((code) => !beforeSet.has(code)),
		removed: before.filter((code) => !afterSet.has(code)),
	};
}
