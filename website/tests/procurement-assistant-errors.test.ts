import assert from "node:assert/strict";
import test from "node:test";
import { parseProcurementError } from "../features/procurement-assistant/errors";

test("讀取獨立的 reason 欄位，不再切字串", () => {
	assert.deepEqual(
		parseProcurementError(
			"REQUIREMENT_NOT_READY",
			"需求尚未就緒，仍有必填欄位缺漏",
		),
		{ code: "REQUIREMENT_NOT_READY", detail: "需求尚未就緒，仍有必填欄位缺漏" },
	);
	assert.deepEqual(
		parseProcurementError(
			"CANDIDATE_FACTS_STALE",
			"候選資料已更新，請重新抓取候選並確認",
		),
		{
			code: "CANDIDATE_FACTS_STALE",
			detail: "候選資料已更新，請重新抓取候選並確認",
		},
	);
	assert.deepEqual(
		parseProcurementError("LANE_MISMATCH", "lane 與候選實際所屬 lane 不符"),
		{ code: "LANE_MISMATCH", detail: "lane 與候選實際所屬 lane 不符" },
	);
	assert.deepEqual(
		parseProcurementError("INPUT_TOO_LONG", "單則訊息上限 2000 字"),
		{ code: "INPUT_TOO_LONG", detail: "單則訊息上限 2000 字" },
	);
});

test("reason 空字串時落到 UNKNOWN（Gateway 對無 BusinessError detail 的錯誤送出空 reason）", () => {
	assert.deepEqual(
		parseProcurementError(
			"",
			"clientRequestId 已用於其他需求，請改用新的 clientRequestId",
		),
		{
			code: "UNKNOWN",
			detail: "clientRequestId 已用於其他需求，請改用新的 clientRequestId",
		},
	);
	assert.deepEqual(parseProcurementError("", "internal error"), {
		code: "UNKNOWN",
		detail: "internal error",
	});
});
