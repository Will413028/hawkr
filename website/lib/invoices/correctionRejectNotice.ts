import { INVOICE_CORRECTION_REJECT_REASON_LABEL } from "@/lib/codemeta/generated";

/**
 * B06 送出更正之後，Dialog 要顯示的那一句。
 *
 * 🔴 **被婉拒時必須把理由接上去。**
 * 後端對 `REJECTED` 只給一句中性引導——`invoice-service` 的
 * `query_views.go:CorrectionStatusNotices` 逐字寫著「REJECTED 只給一句中性的引導：
 * **真正的原因由 `rejectReasonCode` 決定文案**」。而那句話的內容是
 * 「這次修改無法進行，請見下方原因」，於是畫面上出現一句指向不存在之物的指示。
 *
 * dev 實測（2026-09-05，訂單 `O153756363`）：後端正確回
 * `rejectReasonCode = "ALLOWANCE_EXISTS"`（該 document 確有一筆 PENDING 折讓），
 * 而買家在 Dialog、重新載入後的整頁、以及通知裡**三處都找不到理由**；
 * 生成好的 `INVOICE_CORRECTION_REJECT_REASON_LABEL` 當時是零消費者。
 * 後果是他反覆按同一顆按鈕，而且不知道該改走折讓還是申訴。
 *
 * ⚠️ 文案表是 **CodeMeta 生成物**，不在這裡拼字串——值域改動時生成器會同步，
 * 而手寫的 switch 不會（`lib/codemeta/generated.ts` 是唯一來源）。
 * ⚠️ 未知碼**不落回代碼本身**：那等於把 `PROVIDER_STATE_CONFLICT` 這種字丟到買家面前。
 * 未知時只顯示後端那句中性引導，寧可少說也不要說一句他看不懂的。
 */
export function correctionResultNotice(result: {
	statusNotice: string;
	rejectReasonCode: string;
}): string {
	const base = result.statusNotice || "已收到你的修改申請。";
	const label = INVOICE_CORRECTION_REJECT_REASON_LABEL[result.rejectReasonCode];
	if (!label) return base;
	// 後端那句以「，請見下方原因」收尾（指向這裡要接上的東西）。直接串會變成
	// 「請見下方原因原因：已存在折讓。」——2026-09-05 dev 實拍到的就是這個。
	// 剝掉那個引導句再接，讀起來才是一句話。
	//
	// ⚠️ 這條正則綁的是後端當下的措辭。後端改字時它會安靜地不匹配，退化成
	// 「…請見下方原因原因：X。」——**難看但不是錯的**（理由仍然看得到），
	// 刻意選這個退化方向，而不是用「後端沒說對就不顯示理由」那種會把資訊藏起來的寫法。
	const lead = base.replace(/[，,]?\s*請見下方原因[。.]?\s*$/u, "");
	const joiner = lead === base ? "" : "。";
	return `${lead}${joiner}原因：${label}。`;
}
