import { INVOICE_MANUAL_REASON_LABEL } from "@/lib/codemeta/generated";

/**
 * 賣家看到的「為什麼卡住」那一句。
 *
 * 🔴 **2026-09-05 第 6 輪稽核抓到的缺口**：後端四個入口都填了 `manualReasonCode`、
 * 前端型別接了、CodeMeta 的 label 表也生成了 —— 而**畫面層零消費者**。
 * 同時 `invoice.issue_failed.seller` 的通知逐字寫著「請至賣家中心的『發票異常作業』
 * **查看原因**並處理」⇒ 賣家照著去，看到的只有狀態 badge，**沒有原因**。
 *
 * 最嚴重的一格是 `SELLER_TRACK_INSUFFICIENT`（AMEGO 3040111 字軌不足）：
 * 它的處置與其他失敗**完全相反** —— 重試多久都不會好，賣家必須去財政部平台配字軌
 * （CodeMeta 的 description 逐字：「要賣家去財政部配字軌，不是我方改資料、也不是等
 * provider 恢復」）。看不到原因的賣家只會反覆按重試。
 *
 * ⚠️ label 走 CodeMeta 生成物、**不在這裡拼字串**：值域改動時生成器會同步，
 * 手寫的 switch 不會。這裡只負責「哪幾個碼要另外給一句行動指引」。
 */
const ACTION_HINT: Record<string, string> = {
	// 唯一一個「平台做不了、只有賣家自己能解」的碼。行動指引寫在這裡而不是後端，
	// 是因為它是純文案；判定（哪個碼）仍然只由後端決定。
	SELLER_TRACK_INSUFFICIENT:
		"請至財政部電子發票整合服務平台申請新的字軌號碼；配號完成後系統會自動繼續開立，不需要你按重試。",
};

export type ManualReasonNotice = {
	/** 例：「賣家字軌不足」。空字串代表沒有原因可顯示（呼叫端整段不渲染）。 */
	label: string;
	/** 行動指引；沒有對應碼時為空字串。 */
	hint: string;
};

export function manualReasonNotice(code: string): ManualReasonNotice {
	if (!code) return { label: "", hint: "" };
	const label = INVOICE_MANUAL_REASON_LABEL[code];
	// 🔴 未知碼**不落回代碼本身**：那等於把 SELLER_TRACK_INSUFFICIENT 這種字丟到
	// 賣家面前。認不得就不顯示，寧可少說也不要說一句他看不懂的
	// （與 correctionRejectNotice 同一條原則）。
	if (!label) return { label: "", hint: "" };
	return { label, hint: ACTION_HINT[code] ?? "" };
}
