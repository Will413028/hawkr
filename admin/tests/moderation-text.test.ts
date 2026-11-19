import assert from "node:assert/strict";
import test from "node:test";
import { ReportTargetKind } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import {
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
