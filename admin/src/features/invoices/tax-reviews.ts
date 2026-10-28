/**
 * A02 稅務身分審核佇列的呈現規則（端點 38）。
 *
 * 放在 feature 模組而不是頁面裡，是為了讓契約測試驗得到值域與期限分級——這兩者
 * 打錯都不會報錯：filter 值打錯只是回全部，期限分級寫反只是顏色不對。
 */

/**
 * 六個 filter 值逐字對端點 38 的值域，外加「全部」（空字串 ＝ 不篩）。
 *
 * ⚠️ **filter 值不是 reviewStatus 值**：
 *   - `NEW` ＝ `SUBMITTED`
 *   - `UNDER_REVIEW` 這個 filter 涵蓋 `UNDER_REVIEW` ＋ `SELF_ISSUE_PENDING` 兩個狀態
 *   - `NEAR_DEADLINE`（期限 ≤ 7 天）與 `EXTENDED`（延長過一次）是橫切視圖
 * ⇒ 前端當**不透明字串**傳給後端，不拿它去對 `reviewStatus`。
 *
 * ⚠️ 六者不互斥 ⇒ **不得顯示合計、不得用減法算任何一頁的筆數**。
 */
export const TAX_REVIEW_FILTERS: ReadonlyArray<{
	value: string;
	label: string;
}> = [
	{ value: "", label: "全部" },
	{ value: "NEW", label: "新申請" },
	{ value: "SUPPLEMENT_REQUIRED", label: "待補件" },
	{ value: "UNDER_REVIEW", label: "審核中" },
	{ value: "NEAR_DEADLINE", label: "即將到期" },
	{ value: "EXTENDED", label: "已延長" },
	{ value: "EXPIRED", label: "已逾期" },
];

export const TAX_REVIEW_FILTER_VALUES = TAX_REVIEW_FILTERS.map((t) => t.value);

/**
 * 「開始審核」vs「查看詳情」的判準（純文案，不是權限）。
 *
 * 集合逐字等於 A01 `pending_tax_reviews` 的口徑。授權在 Gateway 的 `requireAdmin()`，
 * 前端不做任何 gating——前端隱藏不是授權，寫了是一個假的保證。未知狀態落
 * 「查看詳情」，那是安全的那一邊。
 */
export const PENDING_REVIEW_STATUSES: readonly string[] = [
	"SUBMITTED",
	"UNDER_REVIEW",
	"SUPPLEMENT_REQUIRED",
	"SELF_ISSUE_PENDING",
];

export type DeadlineTone = "" | "warning" | "danger";

/**
 * 期限欄的文字與分級。
 *
 * 🔴 `remainingDays` **一律用後端下發的值**，前端不從 `deadlineAt` 減 now：前端只會
 * 扣週末、後端用行事曆表扣國定假日 ⇒ 同一案在兩個畫面顯示不同天數，而逾期會計
 * 違規、90 天達 3 次時建議停權（由管理員建立處分）。
 *
 * 空字串代表後端算不出來（無期限）⇒ 明說「處理期限計算中」，**不得退化成日曆天**。
 *
 * ⚠️ 分級一定要同時輸出文字：只靠底色的狀態對色覺障礙與黑白列印等於沒有標記。
 */
export function deadlineDisplay(remainingDays: string): {
	text: string;
	tone: DeadlineTone;
} {
	if (remainingDays === "") return { text: "處理期限計算中", tone: "" };
	const days = Number(remainingDays);
	if (!Number.isFinite(days)) return { text: remainingDays, tone: "" };
	if (days < 0) return { text: `已逾期 ${Math.abs(days)} 天`, tone: "danger" };
	if (days <= 7) return { text: `剩 ${days} 天`, tone: "warning" };
	return { text: `剩 ${days} 天`, tone: "" };
}
