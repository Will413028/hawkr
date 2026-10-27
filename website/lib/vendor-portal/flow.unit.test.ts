import { Code, ConnectError } from "@connectrpc/connect";
import { describe, expect, it } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import {
	resolveEntryStep,
	resolveInviteFailureStep,
	resolveSessionFailureStep,
} from "./flow";

const SECRET = "c".repeat(64);
const RECIPIENT_ID = "843856384063471755";

describe("resolveEntryStep", () => {
	it("fragment 有 secret 就走預覽——即使網址上也有收件人 ID", () => {
		// 第二封信指向的是另一筆 RFQ，不沿用網址上舊的收件人。
		expect(resolveEntryStep(`#invite=${SECRET}`, `?r=${RECIPIENT_ID}`)).toEqual(
			{ kind: "preview", secret: SECRET },
		);
	});

	it("沒有 secret 但網址帶收件人 ID（兌換之後的網址），以那張邀請的 session 讀", () => {
		expect(resolveEntryStep("", `?r=${RECIPIENT_ID}`)).toEqual({
			kind: "session",
			rfqRecipientId: RECIPIENT_ID,
		});
	});

	it("?r 不是正的標準十進位 int64＝連結失效，不問伺服器", () => {
		for (const bad of ["?r=abc", "?r=-1", "?r=007", "?r=9223372036854775808"]) {
			expect(resolveEntryStep("", bad), bad).toEqual({
				kind: "failed",
				reason: "link_invalid",
			});
		}
	});

	it("兩者都沒有＝有人直接開 /vendor/rfq，不打任何端點", () => {
		expect(resolveEntryStep("", "")).toEqual({
			kind: "failed",
			reason: "link_invalid",
		});
		expect(resolveEntryStep("", "?r=")).toEqual({
			kind: "failed",
			reason: "link_invalid",
		});
	});
});

describe("resolveInviteFailureStep", () => {
	it("Connect 的 not_found（邀請不可用）＝連結失效，沒有退路", () => {
		expect(
			resolveInviteFailureStep(new ConnectError("not found", Code.NotFound)),
		).toEqual({ kind: "failed", reason: "link_invalid" });
	});

	it("Connect 的 resource_exhausted 是節流，不說連結失效", () => {
		expect(
			resolveInviteFailureStep(new ConnectError("x", Code.ResourceExhausted)),
		).toEqual({ kind: "failed", reason: "rate_limited" });
	});

	it("其餘（unavailable、斷線）走一般錯誤，讓業者可以重試", () => {
		expect(
			resolveInviteFailureStep(new ConnectError("x", Code.Unavailable)),
		).toEqual({ kind: "failed", reason: "error" });
		expect(resolveInviteFailureStep(new Error("network"))).toEqual({
			kind: "failed",
			reason: "error",
		});
	});
});

describe("resolveSessionFailureStep", () => {
	// session 這條是最後一條路——它失敗時已經沒有退路了，但「沒有退路」不等於
	// 「連結無效」：被節流或後端暫時掛掉時叫業者去檢查信件連結是錯的指示。
	it("GetSession 的 unauthenticated（沒有這張邀請的有效 session）＝連結失效", () => {
		expect(
			resolveSessionFailureStep(new ConnectError("x", Code.Unauthenticated)),
		).toEqual({ kind: "failed", reason: "link_invalid" });
	});

	it("GetSession 的 invalid_argument（收件人 ID 不合法）＝連結失效", () => {
		expect(
			resolveSessionFailureStep(new ConnectError("x", Code.InvalidArgument)),
		).toEqual({ kind: "failed", reason: "link_invalid" });
	});

	it("Connect 的 resource_exhausted 是節流、unavailable 是一般錯誤", () => {
		expect(
			resolveSessionFailureStep(new ConnectError("x", Code.ResourceExhausted)),
		).toEqual({ kind: "failed", reason: "rate_limited" });
		expect(
			resolveSessionFailureStep(new ConnectError("x", Code.Unavailable)),
		).toEqual({ kind: "failed", reason: "error" });
	});

	it("REST 的 401（gateway：沒有有效的 vendor session）＝連結失效，同 Connect 的 unauthenticated", () => {
		expect(
			resolveSessionFailureStep(new ApiClientError("unauthorized", 401)),
		).toEqual({ kind: "failed", reason: "link_invalid" });
	});

	it("404 才是連結失效", () => {
		expect(
			resolveSessionFailureStep(new ApiClientError("not found", 404)),
		).toEqual({ kind: "failed", reason: "link_invalid" });
	});

	it("429 是節流", () => {
		expect(
			resolveSessionFailureStep(new ApiClientError("too many", 429)),
		).toEqual({ kind: "failed", reason: "rate_limited" });
	});

	it("5xx 與斷線是一般錯誤，要讓業者知道可以重試", () => {
		expect(resolveSessionFailureStep(new ApiClientError("boom", 503))).toEqual({
			kind: "failed",
			reason: "error",
		});
		expect(resolveSessionFailureStep(new Error("network"))).toEqual({
			kind: "failed",
			reason: "error",
		});
	});
});
