import assert from "node:assert/strict";
import { test } from "node:test";
import {
	isImportItemBatchEditable,
	isImportItemCommitEligible,
	isImportItemRestorable,
	isImportItemSelectable,
} from "../features/seller-center/products-import/itemState";
import {
	IMPORT_ITEM_STATUS,
	type ImportItemBrief,
} from "../features/seller-center/products-import/types";

function item(
	status: ImportItemBrief["status"],
	blockingErrorCount = "0",
): ImportItemBrief {
	return {
		importItemId: "1",
		productKey: "p1",
		status,
		name: "商品",
		categoryId: "10",
		catL3Name: "葉菜",
		skuCount: "1",
		priceMin: "100",
		priceMax: "100",
		firstImageUrl: "",
		blockingErrorCount,
		warningCount: "0",
		needsFix: blockingErrorCount !== "0",
		hasDuplicateExisting: false,
		commitRevision: "1",
		version: "1",
	};
}

test("committed import item is not selectable or batch editable", () => {
	const committed = item(IMPORT_ITEM_STATUS.COMMITTED);

	assert.equal(isImportItemSelectable(committed), false);
	assert.equal(isImportItemBatchEditable(committed), false);
});

test("ready import item is commit eligible only without blocking errors", () => {
	assert.equal(
		isImportItemCommitEligible(item(IMPORT_ITEM_STATUS.READY)),
		true,
	);
	assert.equal(
		isImportItemCommitEligible(item(IMPORT_ITEM_STATUS.READY, "2")),
		false,
	);
});

test("removed import item remains selectable only for restore", () => {
	const removed = item(IMPORT_ITEM_STATUS.REMOVED);

	assert.equal(isImportItemSelectable(removed), true);
	assert.equal(isImportItemBatchEditable(removed), false);
	assert.equal(isImportItemCommitEligible(removed), false);
	assert.equal(isImportItemRestorable(removed), true);
});
