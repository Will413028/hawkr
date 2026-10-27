import assert from "node:assert/strict";
import test from "node:test";
import { extractLiveEntityId } from "./seo/_helpers";

test("live seller discovery reads the canonical storeId field", () => {
	assert.equal(
		extractLiveEntityId(
			{
				data: {
					records: [{ storeId: "7487768729113814280" }],
				},
			},
			"seller",
		),
		"7487768729113814280",
	);
});

test("live entity discovery keeps product ids and empty results distinct", () => {
	assert.equal(
		extractLiveEntityId({ data: { records: [{ id: 42 }] } }, "product"),
		"42",
	);
	assert.equal(extractLiveEntityId({ data: { records: [] } }, "seller"), null);
});
