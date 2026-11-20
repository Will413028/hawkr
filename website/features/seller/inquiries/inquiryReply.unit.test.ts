import { describe, expect, it } from "vitest";
import { INQUIRY_REPLY_MAX_LENGTH } from "@/lib/seller/inquiries";
import { itemErrorsByToken, replyDraftState } from "./inquiryReply";

const base = { hasFailedPhoto: false, uploading: false, busy: false };

describe("replyDraftState", () => {
	it("空白與只有空白不能送出", () => {
		expect(replyDraftState({ ...base, body: "" }).canSubmit).toBe(false);
		expect(replyDraftState({ ...base, body: " \n\t " }).canSubmit).toBe(false);
	});

	it("有內容可以送出，字數以去掉頭尾空白後的 rune 計", () => {
		const state = replyDraftState({ ...base, body: "  你好\u{20BB7}  " });
		expect(state).toEqual({ length: 3, tooLong: false, canSubmit: true });
	});

	it("剛好上限可以送出，多一個字不行", () => {
		const ok = replyDraftState({
			...base,
			body: "字".repeat(INQUIRY_REPLY_MAX_LENGTH),
		});
		expect(ok.canSubmit).toBe(true);
		const over = replyDraftState({
			...base,
			body: "字".repeat(INQUIRY_REPLY_MAX_LENGTH + 1),
		});
		expect(over.tooLong).toBe(true);
		expect(over.canSubmit).toBe(false);
	});

	it("有上傳失敗的附件或送出中都不能送出", () => {
		expect(
			replyDraftState({ ...base, body: "好", hasFailedPhoto: true }).canSubmit,
		).toBe(false);
		expect(replyDraftState({ ...base, body: "好", busy: true }).canSubmit).toBe(
			false,
		);
	});
});

describe("replyDraftState 附件上傳中", () => {
	it("附件還在上傳時不能送出，上傳完才可以", () => {
		expect(
			replyDraftState({ ...base, body: "好", uploading: true }).canSubmit,
		).toBe(false);
		expect(
			replyDraftState({ ...base, body: "好", uploading: false }).canSubmit,
		).toBe(true);
	});
});

describe("itemErrorsByToken", () => {
	it("以送出時的位置對到 token，超出範圍的略過", () => {
		expect(itemErrorsByToken({ 0: "已過期", 5: "不存在" }, ["a", "b"])).toEqual(
			{ a: "已過期" },
		);
	});
});
