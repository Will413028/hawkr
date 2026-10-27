import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	FDAChannel,
	FDAReferralSchema,
	ReporterContactSchema,
	ReportMoveSchema,
	ReportSupplementSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { EvidenceSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	MeasureKind,
	MeasureState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	actorTimeText,
	derivedMeasureText,
	evidenceItems,
	moveRecordText,
	referralText,
	reporterView,
	reportSummary,
	supplementSummary,
	supplementTitle,
	timeText,
} from "../src/app/admin/violation-reports/report-view";
import { report, T0, ts } from "./fixtures/report-fixtures";

/** 通報與案件詳情頁的顯示邏輯（台北時間：2026-10-18T02:00Z 是 2026/10/18 10:00）。 */

const evidence = (position: number, file = `f${position}`) =>
	create(EvidenceSchema, {
		position,
		viewUri: `/api/files/admin/seller/reports/r1/evidence/${file}`,
	});

test("時間顯示台北時間到分鐘；沒有時間是「-」", () => {
	assert.equal(timeText(ts(T0)), "2026/10/18 10:00");
	assert.equal(timeText(undefined), "-");
	assert.equal(actorTimeText("9001", ts(T0)), "9001・2026/10/18 10:00");
	assert.equal(actorTimeText("", undefined), "-");
});

test("檢舉人：登入的是買家 ID；匿名的列出有提供的電話、聯絡信箱、姓名；聯絡方式已清除時說明清除的時間", () => {
	assert.deepEqual(
		reporterView({
			reporterBuyerId: "50007",
			contact: undefined,
			contactEraseTime: undefined,
		}),
		{ kind: "buyer", buyerId: "50007" },
	);
	assert.deepEqual(
		reporterView({
			reporterBuyerId: "",
			contact: create(ReporterContactSchema, {
				phone: "0912345678",
				email: "",
				name: "王小明",
			}),
			contactEraseTime: undefined,
		}),
		{
			kind: "anonymous",
			contact: [
				{ label: "電話", value: "0912345678" },
				{ label: "姓名", value: "王小明" },
			],
			erasedText: undefined,
		},
	);
	assert.deepEqual(
		reporterView({
			reporterBuyerId: "",
			contact: undefined,
			contactEraseTime: ts(T0),
		}),
		{
			kind: "anonymous",
			contact: [],
			erasedText: "聯絡方式已於 2026/10/18 10:00 依保留期限清除",
		},
	);
});

test("補充的標題「補充 {n}・{時間}」；通報的摘要與補充的摘要", () => {
	assert.equal(supplementTitle(0, ts(T0)), "補充 1・2026/10/18 10:00");
	assert.equal(supplementTitle(2, undefined), "補充 3・-");
	const r = report({ id: "r9", category: PolicyCategory.FRAUD });
	assert.equal(
		reportSummary(r),
		"通報 r9・詐欺或操縱交易・建立 2026/10/18 10:00",
	);
	assert.equal(supplementSummary(report({ supplementCount: 0 })), undefined);
	assert.equal(
		supplementSummary(
			report({ supplementCount: 2, latestSupplementTime: ts(T0) }),
		),
		"補充 2 則・最新 2026/10/18 10:00",
	);
});

test("佐證：原通報的是「佐證 {n}」，補充帶來的是「補充 {m} 的佐證 {n}」；連結只收伺服器給的相對路徑", () => {
	const r = report({
		evidence: [
			evidence(0),
			evidence(1),
			evidence(2),
			evidence(3),
			create(EvidenceSchema, {
				position: 4,
				viewUri: "https://evil.example/x",
			}),
		],
		supplements: [
			create(ReportSupplementSchema, { id: "s1", evidencePositions: [2] }),
			create(ReportSupplementSchema, { id: "s2", evidencePositions: [3, 4] }),
		],
	});
	assert.deepEqual(evidenceItems(r), [
		{
			label: "佐證 1",
			href: "/api/bff/api/files/admin/seller/reports/r1/evidence/f0",
		},
		{
			label: "佐證 2",
			href: "/api/bff/api/files/admin/seller/reports/r1/evidence/f1",
		},
		{
			label: "補充 1 的佐證 1",
			href: "/api/bff/api/files/admin/seller/reports/r1/evidence/f2",
		},
		{
			label: "補充 2 的佐證 1",
			href: "/api/bff/api/files/admin/seller/reports/r1/evidence/f3",
		},
		{ label: "補充 2 的佐證 2", href: undefined },
	]);
	assert.deepEqual(evidenceItems(report({})), []);
});

test("移動紀錄：站在案件看，移進來的與移出去的各是一句", () => {
	const move = (from: string, to: string) =>
		create(ReportMoveSchema, {
			reportId: "r1",
			fromCaseId: from,
			toCaseId: to,
			movedByAdminId: "9001",
			moveTime: ts(T0),
		});
	assert.equal(
		moveRecordText(move("c-old", "c-here"), "c-here"),
		"通報 r1 於 2026/10/18 10:00 從案件 c-old 移入（9001）",
	);
	assert.equal(
		moveRecordText(move("c-here", "c-new"), "c-here"),
		"通報 r1 於 2026/10/18 10:00 移到案件 c-new（9001）",
	);
});

test("轉報紀錄：已於 {時間} 記錄轉報（{管道}，案號 {no}）；沒有案號時沒有後半", () => {
	const referral = (referenceNo: string) =>
		create(FDAReferralSchema, {
			channel: FDAChannel.FDA_CHANNEL_HOTLINE_1919,
			referenceNo,
			reportedByAdminId: "9001",
			reportTime: ts(T0),
		});
	assert.equal(
		referralText(referral("FDA-123")),
		"已於 2026/10/18 10:00 記錄轉報（1919 食安專線，案號 FDA-123）",
	);
	assert.equal(
		referralText(referral("")),
		"已於 2026/10/18 10:00 記錄轉報（1919 食安專線）",
	);
});

test("導出的措施：「{種類}・{狀態}・措施 {id}」", () => {
	assert.equal(
		derivedMeasureText({
			id: "m1",
			kind: MeasureKind.SUSPENSION,
			state: MeasureState.ACTIVE,
		}),
		"停權・停權中・措施 m1",
	);
});
