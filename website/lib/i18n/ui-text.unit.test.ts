import { Code } from "@connectrpc/connect";
import { commonUnknown } from "@hawkr/ui-text/keys";
import { describe, expect, it } from "vitest";
import { messagesFor } from "@/lib/intl/config";
import { defaultErrorTextFor, uiTextFor } from "./ui-text";

const CODES = [
	Code.Unknown,
	Code.InvalidArgument,
	Code.DeadlineExceeded,
	Code.NotFound,
	Code.AlreadyExists,
	Code.PermissionDenied,
	Code.ResourceExhausted,
	Code.FailedPrecondition,
	Code.Aborted,
	Code.OutOfRange,
	Code.Unimplemented,
	Code.Internal,
	Code.Unavailable,
	Code.DataLoss,
	Code.Unauthenticated,
] as const;

describe("defaultErrorTextFor", () => {
	it("Connect 的 15 個 code 都有文字，且不是 key 本身", () => {
		const text = defaultErrorTextFor("zh-TW");
		for (const code of CODES) {
			const got = text(code);
			expect(got, Code[code]).not.toBe("");
			expect(got, Code[code]).not.toContain("common.error");
		}
	});

	it("文字取自目錄", () => {
		const text = defaultErrorTextFor("zh-TW");
		expect(text(Code.InvalidArgument)).toBe("參數錯誤");
		expect(text(Code.Internal)).toBe("系統錯誤");
		expect(text(Code.ResourceExhausted)).toBe("操作過於頻繁，請稍後再試");
	});
});

describe("uiTextFor", () => {
	it("不認得的值顯示未知", () => {
		expect(uiTextFor("zh-TW")(commonUnknown())).toBe("未知");
	});
});

describe("every supported locale resolves from its own catalog", () => {
	it("en has the same keys as zh-TW: no catalog key leaks into the text, and the Connect codes all resolve", () => {
		for (const locale of ["zh-TW", "en"] as const) {
			const text = defaultErrorTextFor(locale);
			for (const code of CODES) {
				const got = text(code);
				expect(got, `${locale} ${Code[code]}`).not.toBe("");
				expect(got, `${locale} ${Code[code]}`).not.toContain("common.error");
			}
			expect(uiTextFor(locale)(commonUnknown()), locale).not.toContain(
				"uitext",
			);
		}
	});

	it("each locale gets the messages of its own locale (the provider's messages follow the URL)", () => {
		expect(messagesFor("zh-TW").uitext).toBeDefined();
		expect(messagesFor("en").uitext).toBeDefined();
		expect(messagesFor("zh-TW").common.error).not.toBe(
			messagesFor("en").common.error,
		);
	});
});
