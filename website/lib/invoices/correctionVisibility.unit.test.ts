import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * B04 送出修改申請之後，**重新載入頁面仍然看得到進度與婉拒理由**。
 *
 * 🔴 2026-09-05 第 6 輪稽核抓到的缺口：`fetchInvoiceCorrection`（端點 12 的
 * client）在網頁**零呼叫端**，因為端點 8 的回應沒有任何管道拿得到 correction_id。
 * 症狀是買家送出申請、關掉頁面再回來，畫面上一個字都沒有——而 `rejectReasonCode`
 * 決定他該改走折讓還是申訴。
 *
 * 本檔用源碼字串守，理由與 `correctionEntry` 那一組相反：這裡要守的是「**有沒有
 * 接上**」而不是「判準對不對」，而接沒接上唯一的痕跡就是呼叫點存在。
 * ⚠️ 因此它擋不住「呼叫了但渲染錯」——那一格由 tsc（型別）與 dev 實測覆蓋。
 */
const section = readFileSync(
	join(process.cwd(), "features/invoices/order/OrderInvoiceSection.tsx"),
	"utf8",
);
const repo = readFileSync(
	join(process.cwd(), "lib/invoices/buyer-repo.ts"),
	"utf8",
);

describe("B04 修改申請的持久可見性", () => {
	it("端點 12 有呼叫端（零呼叫端＝未交付）", () => {
		expect(section).toMatch(/fetchInvoiceCorrection\(/);
	});

	it("correction id 取自端點 8 的 latestCorrectionId，不是前端自己記的 state", () => {
		expect(section).toMatch(/invoice\?\.latestCorrectionId/);
		// 後端真的有回這一欄，前端才拿得到。少了 normalizer 那一行，
		// 值恆為 undefined ⇒ 整段靜默不渲染（TS 不會叫，因為型別是 string）。
		expect(repo).toMatch(/latestCorrectionId: str\(data\.latestCorrectionId\)/);
	});

	it("婉拒理由走與 Dialog 相同的文案函式（兩份拼字必然漂移）", () => {
		expect(section).toMatch(/correctionResultNotice\(latestCorrection\)/);
	});
});
