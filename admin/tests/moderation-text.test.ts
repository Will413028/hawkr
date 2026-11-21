import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	CaseAction,
	CaseReportSchema,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { ReportTargetKind } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import {
	actionsFor,
	reporterText,
	TARGET_KIND_OPTIONS,
	targetKindText,
} from "../src/app/admin/moderation/moderation-text";

test("對象種類的篩選選項涵蓋契約的每個種類（不含 UNSPECIFIED），各有文字", () => {
	const all = Object.values(ReportTargetKind)
		.filter((v): v is ReportTargetKind => typeof v === "number")
		.filter((v) => v !== ReportTargetKind.UNSPECIFIED);
	assert.deepEqual(
		[...TARGET_KIND_OPTIONS].sort((a, b) => a - b),
		all.sort((a, b) => a - b),
	);
	for (const kind of TARGET_KIND_OPTIONS) {
		assert.notEqual(targetKindText(kind), "");
	}
});

// 與伺服器 `Action.AllowedOn`（moderation/internal/model/model.go）逐種類一致。
test("每種對象可用的處置與伺服器的 AllowedOn 一致", () => {
	const hwb = [CaseAction.HIDE, CaseAction.WARN, CaseAction.BAN];
	const want = new Map<ReportTargetKind, readonly CaseAction[]>([
		[ReportTargetKind.POST, [CaseAction.TAKEDOWN]],
		[ReportTargetKind.COMMENT, [...hwb, CaseAction.TAKEDOWN]],
		[ReportTargetKind.FORUM_ENTRY, [...hwb, CaseAction.TAKEDOWN]],
		[ReportTargetKind.BUYER, [CaseAction.WARN, CaseAction.BAN]],
		[ReportTargetKind.REVIEW, hwb],
		[ReportTargetKind.CHAT_MESSAGE, [...hwb, CaseAction.TAKEDOWN]],
		[ReportTargetKind.REVIEW_REPLY, [CaseAction.TAKEDOWN]],
	]);
	for (const kind of TARGET_KIND_OPTIONS) {
		assert.deepEqual(actionsFor(kind), want.get(kind), String(kind));
	}
	assert.deepEqual(actionsFor(ReportTargetKind.UNSPECIFIED), []);
});

test("檢舉人文字區分買家與店家", () => {
	const buyer = create(CaseReportSchema, {
		reporter: { case: "reporterBuyerId", value: "5" },
	});
	const store = create(CaseReportSchema, {
		reporter: { case: "reporterStoreId", value: "7" },
	});
	assert.equal(reporterText(buyer), "檢舉人（買家）5");
	assert.equal(reporterText(store), "檢舉人（店家）7");
});
