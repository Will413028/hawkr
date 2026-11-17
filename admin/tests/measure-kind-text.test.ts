import assert from "node:assert/strict";
import test from "node:test";
import { ReviewQueueItemKind } from "@hawkr/api/gen/hawkr/admin/reviewqueue/v1alpha1/review_queue_pb";
import {
	MeasureKind,
	MeasureState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerMeasureKind } from "@hawkr/ui-text/keys";
import { formatStaff } from "@hawkr/ui-text/zh-TW";
import {
	measureKindLabel,
	measureStateBadge,
} from "../src/app/admin/merchants/[id]/measure";
import { QUEUE_KIND_OPTIONS } from "../src/app/admin/queue/queue-item";

/**
 * 措施種類的顯示文字取自共用文案目錄：內容下架（社群檢舉案件與貼文管理建立的措施）與其他種類一樣，
 * 措施頁與詳情不另寫一份對照表。
 */

test("內容下架的種類文字就是目錄的文字", () => {
	const message = sellerMeasureKind(MeasureKind.CONTENT_TAKEDOWN);
	assert.ok(message);
	assert.equal(
		measureKindLabel(MeasureKind.CONTENT_TAKEDOWN),
		formatStaff(message),
	);
	assert.equal(measureKindLabel(MeasureKind.CONTENT_TAKEDOWN), "內容下架");
});

test("每個有效的措施種類都取自目錄，UNSPECIFIED 是「未知」", () => {
	for (const kind of [
		MeasureKind.WARNING,
		MeasureKind.SUSPENSION,
		MeasureKind.TERMINATION,
		MeasureKind.CONTENT_TAKEDOWN,
	]) {
		const message = sellerMeasureKind(kind);
		assert.ok(message, `目錄沒有種類 ${kind}`);
		assert.equal(measureKindLabel(kind), formatStaff(message));
	}
	assert.equal(measureKindLabel(MeasureKind.UNSPECIFIED), "未知");
});

test("內容下架的措施狀態文字取自目錄", () => {
	assert.notEqual(
		measureStateBadge(MeasureState.ACTIVE, MeasureKind.CONTENT_TAKEDOWN).label,
		"未知",
	);
});

test("待辦的篩選有社群檢舉案件的種類", () => {
	assert.ok(QUEUE_KIND_OPTIONS.includes(ReviewQueueItemKind.MODERATION_CASE));
});
