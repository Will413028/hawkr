import assert from "node:assert/strict";
import test from "node:test";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	ALL_CATEGORIES,
	ASSESSABLE_CATEGORIES,
	policyCategoryText,
	REPORTER_CATEGORIES,
} from "../src/lib/policy-category";

/**
 * 政策分類（common.PolicyCategory）在 admin 的子集與顯示文字。子集由伺服器驗證（backend/internal/platform/policy），
 * 這裡的常數只是下拉的選項：少一個值或多一個值，都要讓這個測試失敗，不是等到伺服器回 invalid_argument。
 */

const ALL_VALUES = Object.values(PolicyCategory).filter(
	(v): v is PolicyCategory =>
		typeof v === "number" && v !== PolicyCategory.UNSPECIFIED,
);

/** 社群內容的類別：不在店家治理的兩個子集（backend 的 policy.Community()）。 */
const COMMUNITY = [
	PolicyCategory.SPAM,
	PolicyCategory.HARASSMENT,
	PolicyCategory.HATE,
];

test("檢舉人可選的類別：全部除了 PLATFORM_ABUSE 與社群類別（12 個，含 OTHER）", () => {
	assert.equal(ALL_VALUES.length, 16);
	assert.deepEqual(
		[...REPORTER_CATEGORIES].sort((a, b) => a - b),
		ALL_VALUES.filter(
			(v) => v !== PolicyCategory.PLATFORM_ABUSE && !COMMUNITY.includes(v),
		),
	);
	assert.ok(REPORTER_CATEGORIES.includes(PolicyCategory.OTHER));
});

test("平台判定與認定可用的類別：全部除了 OTHER 與社群類別（12 個，含 PLATFORM_ABUSE）", () => {
	assert.deepEqual(
		[...ASSESSABLE_CATEGORIES].sort((a, b) => a - b),
		ALL_VALUES.filter(
			(v) => v !== PolicyCategory.OTHER && !COMMUNITY.includes(v),
		),
	);
	assert.ok(ASSESSABLE_CATEGORIES.includes(PolicyCategory.PLATFORM_ABUSE));
});

test("全部的類別：店家治理的 13 個值，一個不多一個不少（所有通報清單的類別篩選；不含社群類別）", () => {
	assert.deepEqual(
		[...ALL_CATEGORIES].sort((a, b) => a - b),
		ALL_VALUES.filter((v) => !COMMUNITY.includes(v)),
	);
});

test("三個清單都沒有重複，也不含 UNSPECIFIED；顯示順序同契約的數字", () => {
	for (const subset of [
		REPORTER_CATEGORIES,
		ASSESSABLE_CATEGORIES,
		ALL_CATEGORIES,
	]) {
		assert.equal(new Set(subset).size, subset.length);
		assert.ok(!subset.includes(PolicyCategory.UNSPECIFIED));
		assert.deepEqual(
			[...subset],
			[...subset].sort((a, b) => a - b),
		);
	}
});

test("每個值都有目錄的名稱（術語表的詞），UNSPECIFIED 與不認得的值是「未知」", () => {
	const names = ALL_VALUES.map(policyCategoryText);
	assert.deepEqual(names, [
		"食品安全",
		"違法或禁售商品",
		"未經許可販售",
		"不實或誤導資訊",
		"假認證",
		"仿冒商品",
		"侵害智慧財產權",
		"詐欺或操縱交易",
		"繞過平台交易",
		"不當使用個人資料",
		"冒用或誤導交易主體",
		"干擾或濫用平台",
		"其他",
		"垃圾訊息",
		"騷擾或霸凌",
		"仇恨或歧視言論",
	]);
	assert.equal(policyCategoryText(PolicyCategory.UNSPECIFIED), "未知");
	assert.equal(policyCategoryText(99 as PolicyCategory), "未知");
});
