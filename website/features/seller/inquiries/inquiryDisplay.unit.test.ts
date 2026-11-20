import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	InquiryKind,
	InquiryReplyChannel,
	InquiryReplySchema,
	InquirySchema,
	InquiryState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	sellerInquiryKind,
	sellerInquiryReplyChannel,
	sellerInquiryState,
} from "@hawkr/ui-text/keys";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import en from "@/messages/en.json";
import zhTW from "@/messages/zh-TW.json";
import {
	INQUIRY_FILTERS,
	inquiryBody,
	inquiryDeadline,
	inquiryFilterState,
	inquiryHref,
	replyIsOffSystem,
} from "./inquiryDisplay";

const due = new Date("2026-12-01T00:00:00Z");
const open = (canReply = true) =>
	create(InquirySchema, {
		id: "i1",
		state: InquiryState.OPEN,
		canReply,
		replyDeadline: {
			startTime: timestampFromDate(new Date("2026-11-20T00:00:00Z")),
			dueTime: timestampFromDate(due),
		},
	});

describe("名稱取自共用文案目錄", () => {
	it("UNSPECIFIED 與不認得的值沒有文字", () => {
		expect(sellerInquiryKind(InquiryKind.UNSPECIFIED)).toBeUndefined();
		expect(sellerInquiryState(InquiryState.UNSPECIFIED)).toBeUndefined();
		expect(
			sellerInquiryReplyChannel(InquiryReplyChannel.UNSPECIFIED),
		).toBeUndefined();
		expect(sellerInquiryKind(99 as InquiryKind)).toBeUndefined();
	});

	it("每個有效成員在 zh-TW 與 en 都有文字，篩選的 all 在 messages", () => {
		const members = [
			sellerInquiryKind(InquiryKind.RECORDS),
			sellerInquiryKind(InquiryKind.EXPLANATION),
			sellerInquiryState(InquiryState.OPEN),
			sellerInquiryState(InquiryState.REPLIED),
			sellerInquiryState(InquiryState.WITHDRAWN),
			sellerInquiryReplyChannel(InquiryReplyChannel.PHONE),
			sellerInquiryReplyChannel(InquiryReplyChannel.EMAIL),
			sellerInquiryReplyChannel(InquiryReplyChannel.MESSAGING),
			sellerInquiryReplyChannel(InquiryReplyChannel.OTHER),
			sellerInquiryReplyChannel(InquiryReplyChannel.SELLER_CENTER),
		];
		for (const locale of ["zh-TW", "en"] as const) {
			const ut = uiTextFor(locale);
			for (const m of members) expect(m && ut(m)).toBeTruthy();
		}
		for (const messages of [zhTW, en]) {
			expect(messages.seller.inquiries.filter.all).toBeTruthy();
		}
		expect(INQUIRY_FILTERS).toContain("all");
	});

	it("zh-TW 與 en 的 seller.inquiries 有相同的 key 結構", () => {
		const shape = (v: unknown): unknown =>
			v && typeof v === "object"
				? Object.fromEntries(Object.entries(v).map(([k, x]) => [k, shape(x)]))
				: "text";
		expect(shape(en.seller.inquiries)).toEqual(shape(zhTW.seller.inquiries));
	});
});

describe("inquiryDeadline", () => {
	it("期限前是 due，剛好到期不算逾期，超過才是 overdue", () => {
		expect(inquiryDeadline(open(), new Date("2026-11-30T00:00:00Z")).kind).toBe(
			"due",
		);
		expect(inquiryDeadline(open(), due).kind).toBe("due");
		const late = inquiryDeadline(open(), new Date(due.getTime() + 1));
		expect(late.kind).toBe("overdue");
		expect(late.kind === "overdue" && late.due.getTime()).toBe(due.getTime());
	});

	it("非開放中的詢問沒有期限標記", () => {
		const replied = create(InquirySchema, {
			state: InquiryState.REPLIED,
			replyDeadline: { dueTime: timestampFromDate(due) },
		});
		expect(inquiryDeadline(replied, new Date("2027-01-01T00:00:00Z"))).toEqual({
			kind: "none",
		});
	});

	it("沒有 due_time 時沒有期限", () => {
		expect(
			inquiryDeadline(
				create(InquirySchema, { state: InquiryState.OPEN }),
				new Date(),
			),
		).toEqual({ kind: "none" });
	});
});

describe("inquiryBody", () => {
	it("逾期的開放詢問讀 canReply：仍顯示表單", () => {
		expect(inquiryBody(open(true))).toBe("form");
	});

	it("canReply 為 false 時不顯示表單，不從狀態推導能不能回覆", () => {
		expect(inquiryBody(open(false))).toBe("readonly");
	});

	it("已回覆顯示回覆內容，已撤回唯讀", () => {
		const replied = create(InquirySchema, {
			state: InquiryState.REPLIED,
			reply: create(InquiryReplySchema, {
				body: "好",
				channel: InquiryReplyChannel.SELLER_CENTER,
			}),
		});
		expect(inquiryBody(replied)).toBe("replied");
		expect(
			inquiryBody(create(InquirySchema, { state: InquiryState.WITHDRAWN })),
		).toBe("withdrawn");
	});
});

describe("replyIsOffSystem", () => {
	const replied = (channel: InquiryReplyChannel) =>
		create(InquirySchema, {
			state: InquiryState.REPLIED,
			reply: create(InquiryReplySchema, { body: "好", channel }),
		});

	it("賣家中心以外的管道是管理員代記的系統外回覆", () => {
		expect(replyIsOffSystem(replied(InquiryReplyChannel.PHONE))).toBe(true);
		expect(replyIsOffSystem(replied(InquiryReplyChannel.SELLER_CENTER))).toBe(
			false,
		);
		expect(replyIsOffSystem(create(InquirySchema))).toBe(false);
	});
});

describe("篩選與連結", () => {
	it("篩選對到契約的狀態，全部不篩", () => {
		expect(inquiryFilterState("all")).toBe(InquiryState.UNSPECIFIED);
		expect(inquiryFilterState("open")).toBe(InquiryState.OPEN);
		expect(inquiryFilterState("replied")).toBe(InquiryState.REPLIED);
		expect(inquiryFilterState("withdrawn")).toBe(InquiryState.WITHDRAWN);
	});

	it("詳情連結編碼 id", () => {
		expect(inquiryHref({ id: "a/b" })).toBe("/seller/inquiries/a%2Fb");
	});
});
