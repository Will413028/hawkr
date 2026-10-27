import { describe, expect, it } from "vitest";
import {
	type DelegatedSessionSummary,
	resolveDelegatedSessionState,
} from "./session";

/**
 * 工作階段狀態的合成（P1-5 的第二半）。
 *
 * 病灶：cookie 一旦不在，BFF 回的是 `401 CREDENTIAL_MISSING`（**字串**碼，對不上終態
 * 數字表），舊寫法「有錯就 invalid」於是把一個已經到期的工作階段顯示成「目前無法確認
 * 代行工作階段」。到期是這個系統最尋常的結束方式，而 `labels.ts` 為它寫的那段文案
 * 因此是死碼。
 *
 * ⚠️ 這一條與 cookie 的 grace 是**同一個修復的兩半**，缺一都會退回原狀：
 * - 只加 grace：grace 用完之後 cookie 仍會死，畫面會從「已到期」變回「無法確認」
 * - 只改優先序：grace 之前那 3.3 秒的窗口仍然只有 invalid
 */

const STORE_ID = "2685914682";
const ISSUED_AT = "2026-08-16T04:00:00.000Z";
const EXPIRES_AT = "2026-08-16T04:30:00.000Z";

function summaryOf(
	patch: Partial<DelegatedSessionSummary> = {},
): DelegatedSessionSummary {
	return {
		sessionId: "7494590778405027840",
		displayCode: "OPS-ABCDEFGHIJKLM",
		target: {
			storeId: STORE_ID,
			sellerAccountId: "2685914682",
			storeName: "梨山盛花園",
		},
		actor: { adminId: "1", displayName: "alexis" },
		reason: { code: "SUPPORT", label: "協助賣家" },
		caseReference: null,
		capabilities: ["products.read"],
		issuedAt: ISSUED_AT,
		expiresAt: EXPIRES_AT,
		status: "active",
		revision: 0,
		...patch,
	};
}

const during = Date.parse("2026-08-16T04:10:00.000Z");
const after = Date.parse("2026-08-16T04:30:01.000Z");

describe("resolveDelegatedSessionState", () => {
	it("沒有失敗時照 summary 推導", () => {
		expect(
			resolveDelegatedSessionState({
				summary: summaryOf(),
				failed: false,
				failureCode: null,
				expectedStoreId: STORE_ID,
				now: during,
			}),
		).toBe("active");
	});

	it("上游明講的終態碼優先於一切", () => {
		expect(
			resolveDelegatedSessionState({
				summary: summaryOf(),
				failed: true,
				failureCode: 9271,
				expectedStoreId: STORE_ID,
				now: during,
			}),
		).toBe("revoked");
	});

	/** 這一條就是 P1-5：cookie 先死 ⇒ 字串碼 ⇒ 不得把「已到期」降級成「無法確認」。 */
	it("cookie 消失（CREDENTIAL_MISSING 字串碼）時，已到期的工作階段仍然是 expired", () => {
		expect(
			resolveDelegatedSessionState({
				summary: summaryOf(),
				failed: true,
				failureCode: "CREDENTIAL_MISSING",
				expectedStoreId: STORE_ID,
				now: after,
			}),
		).toBe("expired");
	});

	it("已撤銷／目標不符同樣不因傳輸失敗而退回 invalid", () => {
		expect(
			resolveDelegatedSessionState({
				summary: summaryOf({ status: "revoked" }),
				failed: true,
				failureCode: "UPSTREAM_FAILURE",
				expectedStoreId: STORE_ID,
				now: during,
			}),
		).toBe("revoked");
		expect(
			resolveDelegatedSessionState({
				summary: summaryOf(),
				failed: true,
				failureCode: "UPSTREAM_FAILURE",
				expectedStoreId: "9999999999",
				now: during,
			}),
		).toBe("target_mismatch");
	});

	/**
	 * 反向鑑別：工作階段**還沒到期**時失敗，就是真的不知道 —— 不得樂觀地宣稱 active。
	 * 少了這一條，上面那條可以用「永遠回 derived」來作弊通過。
	 */
	it("尚未到期時的傳輸失敗仍是 invalid（不得把不知道講成有效）", () => {
		expect(
			resolveDelegatedSessionState({
				summary: summaryOf(),
				failed: true,
				failureCode: "UPSTREAM_FAILURE",
				expectedStoreId: STORE_ID,
				now: during,
			}),
		).toBe("invalid");
	});

	it("連 summary 都沒有就是 invalid", () => {
		expect(
			resolveDelegatedSessionState({
				summary: null,
				failed: false,
				failureCode: null,
				expectedStoreId: STORE_ID,
				now: during,
			}),
		).toBe("invalid");
	});
});
