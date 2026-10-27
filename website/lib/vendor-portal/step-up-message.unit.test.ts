import { Code, ConnectError } from "@connectrpc/connect";
import { describe, expect, it } from "vitest";
import {
	STEP_UP_DEFAULT_MESSAGE,
	STEP_UP_SESSION_GONE_MESSAGE,
	stepUpBlockedMessage,
	stepUpFailureMessage,
} from "./step-up-message";

const msg = (code: Code) => stepUpFailureMessage(new ConnectError("x", code));

describe("stepUpFailureMessage", () => {
	it("unauthenticated：回到信件連結", () => {
		expect(msg(Code.Unauthenticated)).toBe(
			"連結的使用期限已過，請重新開啟信件中的連結。",
		);
	});

	it("resource_exhausted：稍後再試", () => {
		expect(msg(Code.ResourceExhausted)).toBe(
			"短時間內驗證太多次。請稍待幾分鐘後再試一次。",
		);
	});

	it("unavailable：暫時無法驗證", () => {
		expect(msg(Code.Unavailable)).toBe("暫時無法驗證，請稍後再試。");
	});

	it("deadline_exceeded：同 unavailable，不是信箱不符的預設文字", () => {
		expect(msg(Code.DeadlineExceeded)).toBe("暫時無法驗證，請稍後再試。");
	});

	it("傳輸失敗（fetch 的 TypeError）：暫時無法驗證", () => {
		expect(stepUpFailureMessage(new TypeError("Failed to fetch"))).toBe(
			"暫時無法驗證，請稍後再試。",
		);
	});

	it("permission_denied 與其他：維持不分成因的預設文字", () => {
		expect(msg(Code.PermissionDenied)).toBe(STEP_UP_DEFAULT_MESSAGE);
		expect(msg(Code.Internal)).toBe(STEP_UP_DEFAULT_MESSAGE);
		expect(msg(Code.Unknown)).toBe(STEP_UP_DEFAULT_MESSAGE);
		expect(stepUpFailureMessage(new Error("boom"))).toBe(
			STEP_UP_DEFAULT_MESSAGE,
		);
	});
});

describe("stepUpBlockedMessage", () => {
	it("沒有 nonce（session 不存在）：不畫按鈕，回 session 已失效的文字", () => {
		expect(stepUpBlockedMessage("")).toBe(STEP_UP_SESSION_GONE_MESSAGE);
	});

	it("有 nonce：可以畫按鈕", () => {
		expect(stepUpBlockedMessage("n")).toBeNull();
	});
});
