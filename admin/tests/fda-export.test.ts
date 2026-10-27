import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	ReporterContactSchema,
	ReportSupplementSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { EvidenceSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	FoodRegistrationSchema,
	StoreSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	buildFdaExport,
	fdaExportFileName,
} from "../src/app/admin/violation-reports/fda-export";
import { CASE_ID, kase, report, T0, ts } from "./fixtures/report-fixtures";

/**
 * 食安案件的轉報資料（.md，client 組）：交給主管機關的文件。通報照伺服器給的原樣列出（伺服器只回結果不是未違規、內容不足或濫報的），
 * 不放檢舉人的聯絡方式，佐證只寫份數。台北時間：2026-10-18T02:00Z 是 2026/10/18 10:00。
 */

const NOW = new Date("2026-10-18T07:30:00Z"); // 15:30

const store = create(StoreSchema, {
	id: "100",
	storeName: "阿里山農場",
	storeCode: "alishan",
	contactName: "陳老闆",
	contactPhone: "05-2561234",
	approvedFoodRegistration: create(FoodRegistrationSchema, {
		registrationNumber: "A-123456789-00000-0",
	}),
});

const evidence = (position: number) =>
	create(EvidenceSchema, { position, viewUri: "/api/files/admin/x" });

test("完整的轉報資料：時間、被通報的店家、每筆通報（商品、訂單、內容、補充、佐證份數）與結尾", () => {
	const later = new Date(T0.getTime() + 60 * 60 * 1000); // 11:00
	const md = buildFdaExport(
		{
			case: kase({ openTime: ts(T0), closeTime: ts(later) }),
			reports: [
				report({
					id: "r1",
					productId: "p-9",
					orderNo: "O-2026-1",
					description: "吃了便當之後腹瀉",
					createTime: ts(T0),
					evidence: [evidence(0), evidence(1)],
					supplements: [
						create(ReportSupplementSchema, {
							id: "s1",
							description: "隔天仍不舒服\n已就醫",
							createTime: ts(later),
						}),
					],
				}),
			],
			store,
		},
		NOW,
	);
	assert.equal(
		md,
		[
			`# 食安通報轉報資料（案件 ${CASE_ID}）`,
			"",
			"本轉報由 Hawkr 平台以平台名義提出；通報人的聯絡方式不在本檔。",
			"",
			"## 1. 時間",
			"",
			"- 開案時間：2026/10/18 10:00",
			"- 結案時間：2026/10/18 11:00",
			"",
			"## 2. 被通報的店家",
			"",
			"- 店名：阿里山農場",
			"- 店號：alishan",
			"- 聯絡人：陳老闆",
			"- 聯絡電話：05-2561234",
			"- 地址：無",
			"- 食品業者登錄字號：A-123456789-00000-0",
			"",
			"## 3. 通報（1 筆）",
			"",
			"### 通報 1（2026/10/18 10:00）",
			"",
			"- 商品 ID：p-9",
			"- 訂單編號：O-2026-1",
			"",
			"內容：",
			"",
			"吃了便當之後腹瀉",
			"",
			"補充：",
			"",
			"- 2026/10/18 11:00：隔天仍不舒服",
			"  已就醫",
			"",
			"佐證：2 份，另附",
			"",
			"---",
			"",
			"本檔由後台於 2026/10/18 15:30 產生，供轉報主管機關使用。",
			"",
		].join("\n"),
	);
});

test("通報原樣列出（GetFDAExport 只回結果不是未違規、內容不足或濫報的通報，client 不再過濾）；標題的筆數就是收到的筆數", () => {
	// 就算收到的通報帶著否定的結果，也不在 client 悄悄丟掉：過濾是伺服器的規則，只有它能改。
	const md = buildFdaExport(
		{
			case: kase({}),
			reports: [
				ReportOutcome.UNSPECIFIED,
				ReportOutcome.ACTION_TAKEN,
				ReportOutcome.DUPLICATE,
				ReportOutcome.NO_VIOLATION,
			].map((outcome, i) =>
				report({ id: `r${i}`, outcome, description: `內容-${i}` }),
			),
			store,
		},
		NOW,
	);
	assert.match(md, /## 3\. 通報（4 筆）/u);
	for (let i = 0; i < 4; i++) {
		assert.ok(md.includes(`內容-${i}`), `內容-${i} 要列出`);
	}
});

test("不放檢舉人的聯絡方式與後台連結", () => {
	const md = buildFdaExport(
		{
			case: kase({}),
			reports: [
				report({
					reporterBuyerId: "50007",
					contact: create(ReporterContactSchema, {
						phone: "0912345678",
						email: "reporter@example.com",
						name: "王小明",
					}),
					evidence: [
						create(EvidenceSchema, {
							position: 0,
							viewUri: "/api/files/admin/seller/reports/r1/evidence/0",
						}),
					],
				}),
			],
			store,
		},
		NOW,
	);
	for (const secret of [
		"0912345678",
		"reporter@example.com",
		"王小明",
		"50007",
		"/api/files",
	]) {
		assert.ok(!md.includes(secret), `${secret} 不能出現在轉報資料裡`);
	}
	assert.match(md, /佐證：1 份，另附/u);
});

test("沒有的資料：還沒結案、沒有登錄字號、沒有提供商品與訂單、沒有佐證與補充", () => {
	const md = buildFdaExport(
		{
			case: kase({}),
			reports: [report({ productId: "", orderNo: "" })],
			store: create(StoreSchema, { id: "100", storeName: "阿里山農場" }),
		},
		NOW,
	);
	assert.match(md, /- 結案時間：還沒結案/u);
	assert.match(md, /- 食品業者登錄字號：無/u);
	assert.match(md, /- 商品 ID：通報人沒有提供/u);
	assert.match(md, /- 訂單編號：通報人沒有提供/u);
	assert.match(md, /佐證：無/u);
	assert.doesNotMatch(md, /補充：/u);
});

test("檔名帶案件 ID", () => {
	assert.equal(fdaExportFileName("c1"), "fda-referral-case-c1.md");
});
