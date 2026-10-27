import { describe, expect, it } from "vitest";
import { correctionResultNotice } from "./correctionRejectNotice";

/**
 * B06 婉拒時「請見下方原因」而下方沒有原因（R5 lane A5 dev 實測，訂單 O153756363）。
 *
 * 期望值一律硬寫字面值——不從 INVOICE_CORRECTION_REJECT_REASON_LABEL 取，
 * 否則生成物一改斷言跟著變，這支就驗不到任何東西。
 */
describe("correctionResultNotice", () => {
	const REJECT_NOTICE = "這次修改無法進行，請見下方原因";

	it("被婉拒時把理由接上，且不得出現「原因原因」", () => {
		// 期望值整句硬寫——不從 REJECT_NOTICE 拼，否則後端措辭一改斷言跟著變。
		// dev 實拍（2026-09-05）修復前的錯誤形態是「…請見下方原因原因：已存在折讓。」
		expect(
			correctionResultNotice({
				statusNotice: REJECT_NOTICE,
				rejectReasonCode: "ALLOWANCE_EXISTS",
			}),
		).toBe("這次修改無法進行。原因：已存在折讓。");

		expect(
			correctionResultNotice({
				statusNotice: REJECT_NOTICE,
				rejectReasonCode: "PAST_VOID_DEADLINE",
			}),
		).toBe("這次修改無法進行。原因：已逾可作廢期限。");
	});

	it("後端改掉引導句時退化成「直接接上」，理由仍然看得到", () => {
		expect(
			correctionResultNotice({
				statusNotice: "系統無法處理這次修改",
				rejectReasonCode: "ALLOWANCE_EXISTS",
			}),
		).toBe("系統無法處理這次修改原因：已存在折讓。");
	});

	it("沒有理由碼時原樣顯示後端那句（成功路徑不得被改寫）", () => {
		const ok = "原發票作廢與重新開立處理中，完成後會寄送 Email";
		expect(
			correctionResultNotice({ statusNotice: ok, rejectReasonCode: "" }),
		).toBe(ok);
	});

	it("未知理由碼不得把代碼本身丟到買家面前", () => {
		const got = correctionResultNotice({
			statusNotice: REJECT_NOTICE,
			rejectReasonCode: "SOMETHING_THE_FRONTEND_HAS_NOT_SEEN",
		});
		expect(got).toBe(REJECT_NOTICE);
		expect(got).not.toContain("SOMETHING_THE_FRONTEND_HAS_NOT_SEEN");
	});

	it("後端連中性引導都沒給時有落底文案", () => {
		expect(
			correctionResultNotice({ statusNotice: "", rejectReasonCode: "" }),
		).toBe("已收到你的修改申請。");
	});
});
