import { describe, expect, it } from "vitest";
import { PostPublishStatus } from "@/lib/codemeta/enums";
import {
	isExpandingPublishTarget,
	POST_PUBLISH_CONFIRM_TEXT,
	POST_PUBLISH_SCHEDULED_CONFIRM_TEXT,
	postPublishConfirmText,
	postPublishToggleTarget,
} from "../postPublishActions";

/**
 * 貼文發布狀態機的方向表。與商品那側的 `shelfToggleTarget` 同構，但值域不對稱這件事
 * 只有在這裡被機械守住：`publish`／`unpublish` 是兩支各自寫死目標的命令端點，
 * **沒有任何一支能把貼文送回 4401**。
 */
describe("postPublishToggleTarget", () => {
	it("未發布 → 發布", () => {
		expect(postPublishToggleTarget(PostPublishStatus.DRAFT)).toBe(
			PostPublishStatus.PUBLISHED,
		);
	});

	it("已下架 → 再次發布（回得到 4402）", () => {
		expect(postPublishToggleTarget(PostPublishStatus.UNPUBLISHED)).toBe(
			PostPublishStatus.PUBLISHED,
		);
	});

	it("已發布 → 下架", () => {
		expect(postPublishToggleTarget(PostPublishStatus.PUBLISHED)).toBe(
			PostPublishStatus.UNPUBLISHED,
		);
	});

	it("缺值／未知碼一律 null（上游沒給 ≠ 已發布）", () => {
		expect(postPublishToggleTarget(undefined)).toBeNull();
		expect(postPublishToggleTarget("")).toBeNull();
		expect(postPublishToggleTarget("4004")).toBeNull();
	});

	it("沒有任何輸入吐得出 4401（賣家回不到未發布）", () => {
		for (const input of [
			PostPublishStatus.DRAFT,
			PostPublishStatus.PUBLISHED,
			PostPublishStatus.UNPUBLISHED,
			undefined,
			"",
			"4401",
			"9999",
		]) {
			expect(postPublishToggleTarget(input)).not.toBe(PostPublishStatus.DRAFT);
		}
	});
});

/**
 * 擴張／收縮的分類是 capability 掛法的依據（46 §3.3）：發布掛 CAP_MARKETING、
 * 下架掛 CAP_CONTENT_CONTRACT。分錯邊就會讓停權賣家失去自清的手，或反過來讓
 * 停權賣家把內容推回買家面前。
 */
describe("isExpandingPublishTarget", () => {
	it("發布是擴張型", () => {
		expect(isExpandingPublishTarget(PostPublishStatus.PUBLISHED)).toBe(true);
	});
	it("下架是收縮型", () => {
		expect(isExpandingPublishTarget(PostPublishStatus.UNPUBLISHED)).toBe(false);
	});
});

/**
 * 發布一則有排程的貼文會取消排程（`SetPublishStatus` 把 pending 的排程一併結束），
 * 確認文字要讓店主知道；下架與沒有排程的發布不加這句。
 */
describe("postPublishConfirmText", () => {
	it("發布且有排程：確認文案後多一句取消排程", () => {
		expect(postPublishConfirmText(PostPublishStatus.PUBLISHED, true)).toBe(
			`${POST_PUBLISH_CONFIRM_TEXT[PostPublishStatus.PUBLISHED]}\n這則動態已排程，現在發布會取消排程。`,
		);
		expect(POST_PUBLISH_SCHEDULED_CONFIRM_TEXT).toBe(
			"這則動態已排程，現在發布會取消排程。",
		);
	});

	it("發布但沒有排程：只有原本的確認文案", () => {
		expect(postPublishConfirmText(PostPublishStatus.PUBLISHED, false)).toBe(
			POST_PUBLISH_CONFIRM_TEXT[PostPublishStatus.PUBLISHED],
		);
	});

	it("下架不加排程那句", () => {
		expect(postPublishConfirmText(PostPublishStatus.UNPUBLISHED, true)).toBe(
			POST_PUBLISH_CONFIRM_TEXT[PostPublishStatus.UNPUBLISHED],
		);
	});
});
