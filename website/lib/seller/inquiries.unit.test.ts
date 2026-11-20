import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { BadRequestSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	InquirySchema,
	InquiryState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";
import { defaultErrorTextFor } from "@/lib/i18n/ui-text";
import {
	fetchMyInquiries,
	fetchMyInquiry,
	fetchPendingInquiryCount,
	replyFieldErrors,
	settleReplyFailure,
	submitInquiryReply,
} from "./inquiries";

const defaults = defaultErrorTextFor("zh-TW");
const inquiry = (id: string, canReply = false) =>
	create(InquirySchema, { id, canReply });

describe("fetchMyInquiries", () => {
	it("依 next_page_token 讀完所有分頁，待回覆數取最後一頁的值", async () => {
		const listMyInquiries = vi
			.fn()
			.mockResolvedValueOnce({
				inquiries: [inquiry("1"), inquiry("2")],
				nextPageToken: "t2",
				pendingReplyCount: 3,
			})
			.mockResolvedValueOnce({
				inquiries: [inquiry("3")],
				nextPageToken: "",
				pendingReplyCount: 3,
			});
		const result = await fetchMyInquiries(
			InquiryState.OPEN,
			() => ({ listMyInquiries }) as never,
		);
		expect(result.inquiries.map((i) => i.id)).toEqual(["1", "2", "3"]);
		expect(result.pendingReplyCount).toBe(3);
		expect(listMyInquiries).toHaveBeenNthCalledWith(1, {
			pageSize: 100,
			pageToken: "",
			state: InquiryState.OPEN,
		});
		expect(listMyInquiries).toHaveBeenNthCalledWith(2, {
			pageSize: 100,
			pageToken: "t2",
			state: InquiryState.OPEN,
		});
	});

	it("next_page_token 沒有前進就丟錯，不無限迴圈", async () => {
		const listMyInquiries = vi.fn().mockResolvedValue({
			inquiries: [],
			nextPageToken: "same",
			pendingReplyCount: 0,
		});
		await expect(
			fetchMyInquiries(
				InquiryState.UNSPECIFIED,
				() => ({ listMyInquiries }) as never,
			),
		).rejects.toThrow("next_page_token");
	});
});

describe("fetchPendingInquiryCount", () => {
	it("只讀一筆並回伺服器算的待回覆數", async () => {
		const listMyInquiries = vi
			.fn()
			.mockResolvedValue({ inquiries: [], pendingReplyCount: 4 });
		expect(
			await fetchPendingInquiryCount(() => ({ listMyInquiries }) as never),
		).toBe(4);
		expect(listMyInquiries).toHaveBeenCalledWith({
			pageSize: 1,
			state: InquiryState.OPEN,
		});
	});
});

describe("fetchMyInquiry 與 submitInquiryReply", () => {
	it("回應沒有詢問時丟錯", async () => {
		const getMyInquiry = vi.fn().mockResolvedValue({});
		await expect(
			fetchMyInquiry("i1", () => ({ getMyInquiry }) as never),
		).rejects.toThrow("GetMyInquiry");
	});

	it("回覆帶詢問 id、內文與附件 token", async () => {
		const replyToInquiry = vi
			.fn()
			.mockResolvedValue({ inquiry: inquiry("i1") });
		const result = await submitInquiryReply(
			"i1",
			{ body: "已提供", evidenceStagedTokens: ["t1", "t2"] },
			() => ({ replyToInquiry }) as never,
		);
		expect(result.id).toBe("i1");
		expect(replyToInquiry).toHaveBeenCalledWith({
			inquiryId: "i1",
			body: "已提供",
			evidenceStagedTokens: ["t1", "t2"],
		});
	});
});

describe("replyFieldErrors", () => {
	it("以精確的 key 對到內文、整個附件欄位與單張", () => {
		expect(
			replyFieldErrors({
				body: "內容有問題",
				evidence_staged_tokens: "最多 5 張",
				"evidence_staged_tokens[2]": "已過期",
			}),
		).toEqual({
			body: "內容有問題",
			evidence: "最多 5 張",
			evidenceByIndex: { 2: "已過期" },
		});
	});

	it("對不上任何欄位時退回整體說明，沒有就用第一個欄位的文字", () => {
		expect(replyFieldErrors({ x: "格式錯誤" }, "資料有誤").other).toBe(
			"資料有誤",
		);
		expect(replyFieldErrors({ x: "格式錯誤" }).other).toBe("格式錯誤");
	});
});

describe("settleReplyFailure", () => {
	const invalid = () =>
		new ConnectError("invalid", Code.InvalidArgument, undefined, [
			{
				desc: BadRequestSchema,
				value: {
					fieldViolations: [
						{
							field: "body",
							description: "invalid",
							localizedMessage: { locale: "zh-TW", message: "內容不可空白" },
						},
					],
				},
			},
		]);

	it("failed_precondition 重讀，伺服器說不能回覆就關閉表單", async () => {
		const reload = vi.fn().mockResolvedValue(inquiry("i1", false));
		const out = await settleReplyFailure(
			new ConnectError("done", Code.FailedPrecondition),
			defaults,
			"i1",
			reload,
		);
		expect(reload).toHaveBeenCalledWith("i1");
		expect(out.kind).toBe("closed");
	});

	it("failed_precondition 但重讀後仍可回覆：顯示伺服器的文字", async () => {
		const reload = vi.fn().mockResolvedValue(inquiry("i1", true));
		const out = await settleReplyFailure(
			new ConnectError("busy", Code.FailedPrecondition),
			defaults,
			"i1",
			reload,
		);
		expect(out.kind).toBe("message");
	});

	it("重讀失敗時退回訊息，不丟出", async () => {
		const reload = vi.fn().mockRejectedValue(new Error("offline"));
		const out = await settleReplyFailure(
			new ConnectError("x", Code.FailedPrecondition),
			defaults,
			"i1",
			reload,
		);
		expect(out.kind).toBe("message");
	});

	it("invalid_argument 回欄位錯誤、不重讀", async () => {
		const reload = vi.fn();
		const out = await settleReplyFailure(invalid(), defaults, "i1", reload);
		expect(out).toEqual({
			kind: "fields",
			fields: { body: "內容不可空白", evidenceByIndex: {} },
		});
		expect(reload).not.toHaveBeenCalled();
	});

	it("其他失敗不重讀，文字用目錄的預設文字", async () => {
		const reload = vi.fn();
		const out = await settleReplyFailure(
			new ConnectError("x", Code.Unavailable),
			defaults,
			"i1",
			reload,
		);
		expect(out.kind).toBe("message");
		expect(reload).not.toHaveBeenCalled();
	});
});
