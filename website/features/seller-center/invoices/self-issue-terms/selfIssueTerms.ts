/**
 * S04 的五條要點。**SSOT 是 `docs/legal/self-issue-attestation-terms.md`**，
 * 本檔是它的 TypeScript 投影，由 `selfIssueTerms.unit.test.ts` 逐條對拍。
 *
 * 🔴 不要在畫面元件裡直接寫這些字串：那會讓條款只存在於 bundle 裡——沒有可引用的
 * 文本、沒有版本歷程、改了也沒有任何東西會叫。
 *
 * 🔴 **版本號不在這裡**：條款版本的權威是後端的 `SelfIssueTermsVersion`，
 * 由端點 21 的 `currentTermsVersion` 送來。前端硬編碼一份必然漂移，而漂移的症狀是
 * 賣家按「同意」被 400 拒——一個看不出原因的失敗。
 */

export interface SelfIssueTermPoint {
	title: string;
	description: string;
}

export const SELF_ISSUE_TERM_POINTS: readonly SelfIssueTermPoint[] = [
	{
		title: "本條款只用來展示具結流程",
		description: "這一頁示範賣家在稅務身分審核期間自行開票的同意畫面。",
	},
	{
		title: "作品集不開立真實發票",
		description: "示範專案沒有串接電子發票服務，不會產生任何發票。",
	},
	{
		title: "買家申訴只是示範資料",
		description: "申訴畫面使用虛構資料，不會通知任何人。",
	},
	{
		title: "處理期限只是畫面設定",
		description: "畫面上的處理天數只用來展示流程，不會累積任何記錄。",
	},
	{
		title: "審核天數與延長規則只是示範",
		description: "這些天數只為展示畫面，不代表任何真實的審核期限。",
	},
] as const;

/**
 * 底部常駐 banner。**不得改寫、不得刪除、不得摺疊**（58A §S04 逐字）。
 *
 * 它要防的誤解很具體：賣家把 60 天讀成「這段期間可以不開發票」。
 */
export const SELF_ISSUE_DEADLINE_NOTICE =
	"畫面上的天數只用來展示流程，不是任何法定期限。";

/** 勾選框文字。勾選本身就是明示同意，所以沒有二次確認 dialog。 */
export const SELF_ISSUE_CONSENT_LABEL =
	"我已閱讀並同意於稅務身分確認前，自行負責開立及交付發票";
