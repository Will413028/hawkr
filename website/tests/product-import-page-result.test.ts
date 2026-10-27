import assert from "node:assert/strict";
import { test } from "node:test";
import { normalizeImportPageResult } from "../features/seller-center/products-import/repo";

test("import pagination keeps valid records and supplies harmless numeric defaults", () => {
	const result = normalizeImportPageResult<{ id: string }>(
		{ records: [{ id: "1" }] },
		"匯入任務",
	);

	assert.deepEqual(result, {
		records: [{ id: "1" }],
		total: 1,
		page: 1,
		pageSize: 1,
	});
});

test("malformed import pagination becomes a recoverable request error", () => {
	assert.throws(
		() => normalizeImportPageResult({}, "匯入任務"),
		/匯入任務回應格式不正確/,
	);
});
