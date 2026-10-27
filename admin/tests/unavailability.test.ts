import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	StoreActionUnavailabilitySchema,
	StoreActionUnavailableReason,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	firstIsStructural,
	firstVisibleLabel,
	GENERIC_UNAVAILABLE_LABEL,
	NO_ENTRY_REASONS,
	unavailableLabels,
} from "../src/lib/unavailability";

const U = StoreActionUnavailableReason;
const why = (reason: StoreActionUnavailableReason, message?: string) =>
	create(StoreActionUnavailabilitySchema, {
		reason,
		message: message ? { message, locale: "zh-TW" } : undefined,
	});

test("結構性集合恰好是契約註解標明「表示沒有入口」的值（讀 proto 檔，不抄一份）", async () => {
	const proto = await readFile(
		"../api/hawkr/admin/seller/v1alpha1/seller.proto",
		"utf8",
	);
	const body = proto.slice(
		proto.indexOf("enum StoreActionUnavailableReason {"),
		proto.indexOf("// StoreActionUnavailability 是此刻不能做某個動作"),
	);
	const fromContract: string[] = [];
	for (const match of body.matchAll(
		/\/\/ ([^\n]*)\n\s*STORE_ACTION_UNAVAILABLE_REASON_([A-Z_]+) = \d+;/gu,
	)) {
		if (match[1].includes("表示沒有入口")) fromContract.push(match[2]);
	}
	assert.deepEqual(
		fromContract.sort(),
		[
			"KYC_NOT_BUSINESS",
			"KYC_NOT_SUBMITTED",
			"PAYUNI_BIND_IN_PROGRESS",
			"PAYUNI_BOUND",
		],
		"契約的註解變了：先決定新值是否有入口",
	);
	assert.deepEqual(
		NO_ENTRY_REASONS.map((r) => StoreActionUnavailableReason[r]).sort(),
		fromContract,
		"client 的結構性集合與契約註解不一致",
	);
});

test("第一個原因是結構性的才是沒有入口：依先後判斷，後面的結構性原因不算", () => {
	assert.equal(firstIsStructural([]), false);
	assert.equal(firstIsStructural([why(U.PAYUNI_BOUND)]), true);
	assert.equal(
		firstIsStructural([
			why(U.STORE_CLOSED, "店家已關閉。"),
			why(U.PAYUNI_BOUND),
		]),
		false,
	);
	assert.equal(firstIsStructural([why(U.UNSPECIFIED)]), false);
});

test("結構性原因不顯示任何句子，也不顯示通用文字（PAYUNI_BIND_IN_PROGRESS 沒有 message）", () => {
	assert.equal(firstVisibleLabel([why(U.PAYUNI_BIND_IN_PROGRESS)]), undefined);
	assert.equal(
		firstVisibleLabel([
			why(U.KYC_NOT_SUBMITTED, "只有審核中的申請能核准或退回。"),
			why(U.STORE_CLOSED, "店家已關閉。"),
		]),
		undefined,
		"第一個是結構性的：後面的原因也不顯示",
	);
	assert.equal(firstVisibleLabel([]), undefined);
});

test("伺服器列出全部原因，畫面只顯示第一個的句子；沒有 message 才顯示通用文字", () => {
	assert.equal(
		firstVisibleLabel([
			why(U.STORE_CLOSED, "店家已關閉。"),
			why(U.ACCOUNT_SUSPENDED, "帳號停權中。"),
		]),
		"店家已關閉。",
		"終止合作關店時伺服器給 [已關閉, 停權]，只顯示第一句",
	);
	assert.equal(
		firstVisibleLabel([why(U.STORE_CLOSED)]),
		GENERIC_UNAVAILABLE_LABEL,
	);
	assert.deepEqual(unavailableLabels([]), [GENERIC_UNAVAILABLE_LABEL]);
});
