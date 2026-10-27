import { describe, expect, it } from "vitest";
import { InvoiceComplaintStatus } from "@/lib/codemeta/generated";
import {
	complaintActionErrorKind,
	complaintActionFieldRule,
	complaintActions,
	foldInvoiceComplaintStatus,
} from "./complaintStatus";

/**
 * 這張折疊表守三件事，每一件失敗都**不會讓任何東西轉紅**：
 *
 *   1. 後端先上線一個前端還不知道的狀態 → 白畫面（§0.6 C11）
 *   2. `OVERDUE` 與 `OPEN` 被合併 → 買家最需要知道的那句（賣家逾期了、平台會
 *      接手）消失，而畫面看起來正常
 *   3. 買家面出現「違規」二字 → 向買家揭露一家店的治理結論
 */
describe("申訴案件狀態的折疊表", () => {
	it("七個定版值都有自己的分支（不落 default）", () => {
		for (const status of Object.values(InvoiceComplaintStatus)) {
			expect(foldInvoiceComplaintStatus(status).unknown).toBe(false);
		}
	});

	it("後端加了第八個狀態時落 default 顯示處理中，不是白畫面", () => {
		const unknown = foldInvoiceComplaintStatus("AWAITING_SELLER");
		expect(unknown.unknown).toBe(true);
		expect(unknown.title).toBeTruthy();
		expect(unknown.body).toBeTruthy();
		// 未知狀態不得宣稱已解決，也不得宣稱逾期。
		expect(unknown.tone).toBe("info");
		expect(unknown.showCountdown).toBe(false);
	});

	it("OVERDUE 與 OPEN 是兩句不同的話", () => {
		const open = foldInvoiceComplaintStatus(InvoiceComplaintStatus.OPEN);
		const overdue = foldInvoiceComplaintStatus(InvoiceComplaintStatus.OVERDUE);
		expect(overdue.title).not.toBe(open.title);
		expect(overdue.body).not.toBe(open.body);
		// 逾期是否構成違規由平台裁定，買家面不預告治理結果。
		expect(overdue.title + overdue.body).not.toContain("違規");
	});

	it("已解決與判定違規對買家是同一件事，且不出現「違規」二字", () => {
		const resolved = foldInvoiceComplaintStatus(
			InvoiceComplaintStatus.RESOLVED,
		);
		const violation = foldInvoiceComplaintStatus(
			InvoiceComplaintStatus.VIOLATION_CONFIRMED,
		);
		expect(violation).toEqual(resolved);
		expect(violation.title + violation.body).not.toContain("違規");
	});

	it("只有等待中的三態顯示倒數，已結案不顯示", () => {
		const withCountdown = Object.values(InvoiceComplaintStatus).filter(
			(status) => foldInvoiceComplaintStatus(status).showCountdown,
		);
		expect(new Set(withCountdown)).toEqual(
			new Set([
				InvoiceComplaintStatus.OPEN,
				InvoiceComplaintStatus.OVERDUE,
				InvoiceComplaintStatus.SELLER_RESPONDED,
			]),
		);
	});
});

/**
 * 三顆動作鍵的可見與可按**一律由 DTO 欄位決定，不由錯誤碼決定**（§B07 第 4 條）。
 * 特別是重開：後端不下發 `canReopen` 布林，前端要自己讀 `reopenCount` ＋
 * `reopenAllowedAfter` 兩欄——判錯的症狀是按鈕看起來可以按，按下去拿 9336。
 */
describe("申訴的三顆動作鍵", () => {
	const past = "2026-08-01T00:00:00Z";
	const future = "2099-01-01T00:00:00Z";
	const now = Date.parse("2026-08-30T00:00:00Z");

	it("撤回只在三個等待中的狀態出現，重開只在已解決出現", () => {
		const withdrawable = Object.values(InvoiceComplaintStatus).filter(
			(s) => complaintActions(s, "0", past, now).canWithdraw,
		);
		expect(new Set(withdrawable)).toEqual(
			new Set([
				InvoiceComplaintStatus.OPEN,
				InvoiceComplaintStatus.OVERDUE,
				InvoiceComplaintStatus.SELLER_RESPONDED,
			]),
		);
		const reopenable = Object.values(InvoiceComplaintStatus).filter(
			(s) => complaintActions(s, "0", past, now).showReopen,
		);
		expect(reopenable).toEqual([InvoiceComplaintStatus.RESOLVED]);
	});

	it("「這樣沒有解決」只在賣家已回覆時出現", () => {
		const rejectable = Object.values(InvoiceComplaintStatus).filter(
			(s) => complaintActions(s, "0", past, now).canReject,
		);
		expect(rejectable).toEqual([InvoiceComplaintStatus.SELLER_RESPONDED]);
	});

	it("冷卻未到：按鈕仍然出現但不可按", () => {
		const a = complaintActions(
			InvoiceComplaintStatus.RESOLVED,
			"1",
			future,
			now,
		);
		expect(a.showReopen).toBe(true);
		expect(a.canReopen).toBe(false);
		expect(a.reopenExhausted).toBe(false);
	});

	it("已達 2 次上限：不可按，且與冷卻是兩種不同的說明", () => {
		const a = complaintActions(InvoiceComplaintStatus.RESOLVED, "2", past, now);
		expect(a.canReopen).toBe(false);
		expect(a.reopenExhausted).toBe(true);
	});

	it("reopenAllowedAfter 為空字串時不可重開（不是「隨時可以」）", () => {
		expect(
			complaintActions(InvoiceComplaintStatus.RESOLVED, "0", "", now).canReopen,
		).toBe(false);
	});

	it("reopenCount 是數字字串：空字串當 0，不當成 NaN 也不當成已用盡", () => {
		const a = complaintActions(InvoiceComplaintStatus.RESOLVED, "", past, now);
		expect(a.reopenExhausted).toBe(false);
		expect(a.canReopen).toBe(true);
	});

	/**
	 * 🔴 #2911：後端把 `VIOLATION_CONFIRMED`（終態）在**買家視角**投影成
	 * `RESOLVED`，好讓狀態 Chip 不印出「判定賣家違規」。投影之後前端唯一能分辨
	 * 「這個案子還能不能重開」的依據就是 `reopenAllowedAfter` ——
	 * invoice-service K13 的述詞含 `reopen_allowed_after IS NOT NULL`，
	 * K06／K08／K12 一律寫 `now + 24h`，K09（判定違規）寫 NULL。
	 *
	 * 少了這一條，終態案件會渲染出一顆永遠按不下去、而且沒有任何說明句的重開鍵。
	 */
	it("沒有重開邊（reopenAllowedAfter 空）時整顆重開鍵不渲染，不是渲染成 disabled", () => {
		const terminal = complaintActions(
			InvoiceComplaintStatus.RESOLVED,
			"0",
			"",
			now,
		);
		expect(terminal.showReopen).toBe(false);
		expect(terminal.canReopen).toBe(false);
	});

	it("真正的 RESOLVED 一律帶得到 reopenAllowedAfter ⇒ 加上這條判斷不改變既有行為", () => {
		// 冷卻中（未到期）與冷卻已過，兩種都必須照舊渲染按鈕。
		expect(
			complaintActions(InvoiceComplaintStatus.RESOLVED, "0", future, now)
				.showReopen,
		).toBe(true);
		expect(
			complaintActions(InvoiceComplaintStatus.RESOLVED, "0", past, now)
				.showReopen,
		).toBe(true);
	});
});

/**
 * 這一條守的是 §B-1.0.9 B18t：**建案的說明是選填、63 的理由是必填**，兩者相反。
 * 照抄既有售後的 `AppealDialog`（必填）到建案上，症狀是買家送不出最常見的那個
 * 情況而按鈕只是灰的、沒有任何錯誤訊息——沒有東西會轉紅。
 */
describe("三支動作端點的輸入欄規則", () => {
	it("63 必填、65 選填、64 根本沒有輸入欄", () => {
		expect(complaintActionFieldRule("reject")).toEqual({
			hasField: true,
			required: true,
		});
		expect(complaintActionFieldRule("reopen")).toEqual({
			hasField: true,
			required: false,
		});
		expect(complaintActionFieldRule("withdraw")).toEqual({
			hasField: false,
			required: false,
		});
	});

	it("只有 63 是必填——三者不可共用同一條規則", () => {
		const required = (["reject", "withdraw", "reopen"] as const).filter(
			(kind) => complaintActionFieldRule(kind).required,
		);
		expect(required).toEqual(["reject"]);
	});
});

/**
 * 🔴 這一條守的是本波最容易做錯、而且**沒有任何東西會轉紅**的一件事：
 * 9322 與 9336 都是 HTTP 409，處置卻相反。9322 借的是稅務側的碼與 message
 * （「稅務資格資料已被更新」），照著印出來，買家會讀到一句與申訴完全無關的話。
 */
describe("三支動作端點的錯誤分流", () => {
	it("9322 用申訴語境的自有文案，9336 才直接用後端 message", () => {
		expect(complaintActionErrorKind(9322)).toBe("versionConflict");
		expect(complaintActionErrorKind(9336)).toBe("backendMessage");
		// 兩者都是 409：分流只能靠業務碼，不能靠 HTTP status。
		expect(complaintActionErrorKind(9322)).not.toBe(
			complaintActionErrorKind(9336),
		);
	});

	it("9309（Tier A）有自己的分支，不會混進通用錯誤", () => {
		expect(complaintActionErrorKind(9309)).toBe("notFound");
	});

	it("只有 9350 走換鍵的那一支", () => {
		expect(complaintActionErrorKind(9350)).toBe("idempotencyMismatch");
		for (const code of [0, 500, 9301, 9310, 9311, 9322, 9336]) {
			expect(complaintActionErrorKind(code)).not.toBe("idempotencyMismatch");
		}
	});

	it("未知碼落 generic，不落 backendMessage（那會把內部訊息漏給買家）", () => {
		expect(complaintActionErrorKind(9999)).toBe("generic");
	});
});
