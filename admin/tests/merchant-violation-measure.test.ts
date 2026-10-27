import assert from "node:assert/strict";
import test from "node:test";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	MeasureKind,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { requestIdFor } from "../src/app/admin/merchants/[id]/measure";
import {
	buildCreateViolationMeasure,
	measureInitialFromSuggestion,
	type ViolationMeasureForm,
	violationMeasureRequestKey,
} from "../src/app/admin/merchants/[id]/violation-measure";

/**
 * 建立措施（違規依據的警告與停權）的請求與 request_id 的鍵：來源通報與引用的認定是集合，
 * 同一組選擇不論勾選順序（取消再勾回會換順序）都是同一個請求、同一個鍵。
 */

function form(patch: Partial<ViolationMeasureForm> = {}): ViolationMeasureForm {
	return {
		measure: { case: "warning" },
		category: PolicyCategory.FRAUD,
		severity: ViolationSeverity.HIGH,
		sourceReportIds: [],
		citedFindingIds: [],
		statement: "商品頁宣稱有機",
		note: "客服轉來兩筆檢舉",
		...patch,
	};
}

const KEY = (f: ViolationMeasureForm) => violationMeasureRequestKey("100", f);

test("來源通報換勾選順序：送出同一個請求（id 排序）、同一個鍵、沿用同一個 request_id", () => {
	const permutations = [
		["r1", "r2", "r3"],
		["r3", "r1", "r2"],
		["r2", "r3", "r1"],
		["r3", "r2", "r1"],
	];
	const first = form({ sourceReportIds: permutations[0] });
	for (const ids of permutations) {
		const f = form({ sourceReportIds: ids });
		assert.equal(KEY(f), KEY(first), `${ids.join(",")} 的鍵`);
		assert.deepEqual(
			buildCreateViolationMeasure("rid", "100", f),
			buildCreateViolationMeasure("rid", "100", first),
			`${ids.join(",")} 的請求`,
		);
	}
	const request = buildCreateViolationMeasure(
		"rid",
		"100",
		form({ sourceReportIds: ["r3", "r1", "r2"] }),
	);
	assert.deepEqual(
		request.measure?.case === "warning"
			? request.measure.value?.violation?.finding?.sourceReportIds
			: undefined,
		["r1", "r2", "r3"],
	);

	// 第一次送出（回應遺失）之後取消勾選再勾回：集合不變、順序變了，必須沿用同一個 request_id。
	let n = 0;
	const generate = () => `id-${++n}`;
	const sent = requestIdFor(
		null,
		KEY(form({ sourceReportIds: ["r1", "r2"] })),
		generate,
	);
	assert.equal(
		requestIdFor(sent, KEY(form({ sourceReportIds: ["r2", "r1"] })), generate),
		sent,
	);
});

test("引用的認定換勾選順序：停權與警告都是同一個請求、同一個鍵", () => {
	const endTime = "2099-01-01T00:00";
	for (const measure of [
		{ case: "warning" },
		{ case: "suspension", endTime },
	] as const) {
		const a = form({
			measure,
			citedFindingIds: ["f1", "f2"],
		});
		const b = { ...a, citedFindingIds: ["f2", "f1"] };
		assert.equal(KEY(a), KEY(b));
		assert.deepEqual(
			buildCreateViolationMeasure("rid", "100", a),
			buildCreateViolationMeasure("rid", "100", b),
		);
	}
	const suspension = buildCreateViolationMeasure(
		"rid",
		"100",
		form({
			measure: { case: "suspension", endTime },
			sourceReportIds: ["r2", "r1"],
			citedFindingIds: ["f2", "f1"],
		}),
	);
	assert.equal(suspension.measure?.case, "suspension");
	const basis =
		suspension.measure?.case === "suspension"
			? suspension.measure.value?.basis
			: undefined;
	assert.equal(basis?.case, "violation");
	const violation = basis?.case === "violation" ? basis.value : undefined;
	assert.deepEqual(violation?.finding?.sourceReportIds, ["r1", "r2"]);
	assert.deepEqual(violation?.citedFindingIds, ["f1", "f2"]);
});

test("不同的集合還是不同的請求：少一筆或換一筆，鍵就不同", () => {
	assert.notEqual(
		KEY(form({ sourceReportIds: ["r1", "r2"] })),
		KEY(form({ sourceReportIds: ["r1"] })),
	);
	assert.notEqual(
		KEY(form({ sourceReportIds: ["r1", "r2"] })),
		KEY(form({ sourceReportIds: ["r1", "r3"] })),
	);
	assert.notEqual(
		KEY(form({ citedFindingIds: ["f1", "f2"] })),
		KEY(form({ citedFindingIds: ["f1"] })),
	);
	// 來源通報與引用的認定是兩個欄位：同一個 id 放在不同欄位不是同一個請求。
	assert.notEqual(
		KEY(form({ sourceReportIds: ["x1"] })),
		KEY(form({ citedFindingIds: ["x1"] })),
	);
	assert.notEqual(
		KEY(form({ sourceReportIds: ["r1"] })),
		violationMeasureRequestKey("101", form({ sourceReportIds: ["r1"] })),
		"換店家是新請求",
	);
});

test("排序不改動呼叫端的選取清單", () => {
	const reports = ["r3", "r1", "r2"];
	const findings = ["f2", "f1"];
	const f = form({ sourceReportIds: reports, citedFindingIds: findings });
	KEY(f);
	buildCreateViolationMeasure("rid", "100", f);
	assert.deepEqual(reports, ["r3", "r1", "r2"]);
	assert.deepEqual(findings, ["f2", "f1"]);
});

/**
 * 發票申訴的停權建議帶進建立措施對話框的初始值（3b 修訂 6-1）：種類是停權、結束時間是現在加建議天數（台北時間）、
 * 理由是建議的原文；類別不預填（發票沒有對應的政策分類，由管理員選）。
 */
test("發票停權建議的預填：停權、結束時間是 now 加建議天數、理由是建議的原文，類別不預填", () => {
	const now = new Date("2026-10-18T02:00:00Z"); // 台北 2026/10/18 10:00
	const initial = measureInitialFromSuggestion(
		{ suggestedSuspendDays: "7", suggestedReason: "90 天內第 3 件發票違規" },
		now,
	);
	assert.equal(initial.category, undefined, "類別不預填，由管理員選");
	assert.deepEqual(initial, {
		measure: { case: "suspension", endTime: "2026-10-25T10:00" },
		statement: "90 天內第 3 件發票違規",
	});
	// 跨月：台北 10/30 23:30 加 3 天是 11/02 23:30。
	assert.deepEqual(
		measureInitialFromSuggestion(
			{ suggestedSuspendDays: "3", suggestedReason: "x" },
			new Date("2026-10-30T15:30:00Z"),
		).measure,
		{ case: "suspension", endTime: "2026-11-02T23:30" },
	);
});

test("建議天數是空字串、0 或不是正整數：不預填結束時間（送出時由「請填寫結束時間。」擋下）；理由是空字串：不預填理由", () => {
	const now = new Date("2026-10-18T02:00:00Z");
	for (const days of [
		"",
		"0",
		"-3",
		"1.5",
		"7天",
		" 7",
		"abc",
		"99999999999999999999",
	]) {
		assert.deepEqual(
			measureInitialFromSuggestion(
				{ suggestedSuspendDays: days, suggestedReason: "理由" },
				now,
			),
			{ measure: { case: "suspension" }, statement: "理由" },
			`天數 ${JSON.stringify(days)}`,
		);
	}
	assert.deepEqual(
		measureInitialFromSuggestion(
			{ suggestedSuspendDays: "7", suggestedReason: "" },
			now,
		),
		{ measure: { case: "suspension", endTime: "2026-10-25T10:00" } },
	);
});

/**
 * 天數有效、算出來的日期也是有效的 Date，但年份超過四位數：toLocalInput 得到五位數的年份，datetime-local 的值
 * （fromLocalInput 的格式）接不了，預填會變成一開啟就送不出去、而且管理員看不出為什麼的值。
 * 這條走的是 measureInitialFromSuggestion 的 `fromLocalInput(text)` 守門，不是前一個測試涵蓋的「天數不是正整數」或「Date 無效」。
 */
test("建議天數大到結束時間的年份超過四位數：不預填結束時間；四位數年份的最大端照常預填", () => {
	const now = new Date("2026-10-18T02:00:00Z");
	// 3,000,000 天後是西元 10240 年。
	assert.deepEqual(
		measureInitialFromSuggestion(
			{ suggestedSuspendDays: "3000000", suggestedReason: "理由" },
			now,
		),
		{ measure: { case: "suspension" }, statement: "理由" },
	);
	// 2,900,000 天後是 9966 年，仍是四位數：守門不能把有效的值一起擋掉。
	assert.deepEqual(
		measureInitialFromSuggestion(
			{ suggestedSuspendDays: "2900000", suggestedReason: "理由" },
			now,
		),
		{
			measure: { case: "suspension", endTime: "9966-09-23T10:00" },
			statement: "理由",
		},
	);
});

test("違規依據的措施：從案件建立時請求帶 source_case_id（警告與停權都是），不是就不帶；來源案件算進 request_id 的鍵", () => {
	const warning = buildCreateViolationMeasure(
		"rid",
		"100",
		form({ sourceCaseId: "case-1" }),
	);
	assert.equal(warning.sourceCaseId, "case-1");
	const suspension = buildCreateViolationMeasure(
		"rid",
		"100",
		form({
			measure: { case: "suspension", endTime: "2026-12-01T10:00" },
			sourceCaseId: "case-1",
		}),
	);
	assert.equal(suspension.sourceCaseId, "case-1");
	assert.equal(
		"sourceCaseId" in buildCreateViolationMeasure("rid", "100", form()),
		false,
	);

	assert.equal(
		KEY(form({ sourceCaseId: undefined })),
		KEY(form({ sourceCaseId: "" })),
	);
	assert.notEqual(KEY(form({ sourceCaseId: "case-1" })), KEY(form()));
	assert.notEqual(
		KEY(form({ sourceCaseId: "case-1" })),
		KEY(form({ sourceCaseId: "case-2" })),
	);
});

test("從主管機關來函建立：請求帶 source_authority_order_id，新認定另帶來源來函（集合，排序）；沒有來函時兩者都不出現；來函進 request_id 的鍵", () => {
	const warning = buildCreateViolationMeasure(
		"rid",
		"100",
		form({
			sourceAuthorityOrderId: "o-1",
			sourceAuthorityOrderIds: ["o-2", "o-1"],
		}),
	) as {
		sourceAuthorityOrderId?: string;
		measure: {
			value: { violation: { finding: { sourceAuthorityOrderIds: string[] } } };
		};
	};
	assert.equal(warning.sourceAuthorityOrderId, "o-1");
	assert.deepEqual(
		warning.measure.value.violation.finding.sourceAuthorityOrderIds,
		["o-1", "o-2"],
	);
	assert.ok(
		!(
			"sourceAuthorityOrderId" in
			buildCreateViolationMeasure("rid", "100", form())
		),
	);
	assert.equal(KEY(form({ sourceAuthorityOrderId: "" })), KEY(form()));
	assert.notEqual(KEY(form({ sourceAuthorityOrderId: "o-1" })), KEY(form()));
	assert.notEqual(KEY(form({ sourceAuthorityOrderIds: ["o-1"] })), KEY(form()));
	assert.equal(
		KEY(form({ sourceAuthorityOrderIds: ["o-2", "o-1"] })),
		KEY(form({ sourceAuthorityOrderIds: ["o-1", "o-2"] })),
	);
});
