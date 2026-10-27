import assert from "node:assert/strict";
import test from "node:test";
import {
	CaseDecision,
	FDAChannel,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	MeasureState,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	assessNotice,
	bringInText,
	buildChangeSeverity,
	buildCreateCaseNote,
	buildDecide,
	buildMove,
	buildReferral,
	buildReferralReply,
	buildResolve,
	buildTriage,
	buildWaive,
	livingSourceMeasures,
	negatesAsSource,
	SOURCE_OVERFLOW_TEXT,
	sourceMeasureNoticeText,
	targetOf,
	validateAssess,
	validateDecide,
	validateReferral,
	validateResolve,
	validateSeverity,
} from "../src/app/admin/violation-reports/report-actions";
import { kase, outcome, report, target } from "./fixtures/report-fixtures";

/**
 * 通報與案件的對話框：輸入的驗證與請求組裝。能不能做、有哪些選項都來自伺服器（這裡的函式只拿伺服器給的選項比對）。
 */

const FOOD = PolicyCategory.FOOD_SAFETY;
const FRAUD = PolicyCategory.FRAUD;

// 伺服器給的去處（triage_targets）：食安已有進行中的案件（高、3 筆通報、etag c-9），詐欺會開新案件。
const TARGETS = [
	target(FOOD, {
		caseId: "c-food",
		severity: ViolationSeverity.HIGH,
		etag: "c-9",
		reportCount: 3,
	}),
	target(FRAUD),
];

test("去處：只認伺服器給的類別", () => {
	assert.equal(targetOf(TARGETS, FOOD), TARGETS[0]);
	assert.equal(targetOf(TARGETS, FRAUD), TARGETS[1]);
	assert.equal(targetOf(TARGETS, PolicyCategory.PRIVACY), undefined);
	assert.equal(targetOf([], FOOD), undefined);
});

test("分案的提示：選了類別之後說明會加入進行中的案件，或開新案件；沒選類別、伺服器沒給的類別沒有提示", () => {
	assert.deepEqual(assessNotice(FOOD, TARGETS), {
		kind: "join",
		text: "這家店的「食品安全」已有進行中的案件（嚴重度：高，3 筆通報），分案後會加入那個案件。",
	});
	assert.deepEqual(assessNotice(FRAUD, TARGETS), {
		kind: "open",
		text: "這家店的「詐欺或操縱交易」沒有進行中的案件，分案後會開一個新案件，請選擇嚴重度。",
	});
	assert.equal(assessNotice(PolicyCategory.UNSPECIFIED, TARGETS), undefined);
	assert.equal(assessNotice(PolicyCategory.OTHER, TARGETS), undefined);
});

test("分案與移動的檢查：類別必須是伺服器給的去處；去處是開新案件時才要選嚴重度", () => {
	const none = ViolationSeverity.UNSPECIFIED;
	assert.equal(
		validateAssess(
			{ category: PolicyCategory.UNSPECIFIED, severity: none },
			TARGETS,
		),
		"請選擇平台判定類別。",
	);
	assert.equal(
		validateAssess({ category: PolicyCategory.OTHER, severity: none }, TARGETS),
		"請選擇平台判定類別。",
		"伺服器沒給的類別（OTHER 只給檢舉人選）不能選",
	);
	assert.equal(
		validateAssess({ category: FRAUD, severity: none }, TARGETS),
		"請選擇嚴重度。",
	);
	assert.equal(
		validateAssess(
			{ category: FRAUD, severity: ViolationSeverity.LOW },
			TARGETS,
		),
		undefined,
	);
	assert.equal(
		validateAssess({ category: FOOD, severity: none }, TARGETS),
		undefined,
		"加入進行中的案件，嚴重度以案件為準",
	);
});

test("分案與移動的請求：原樣放回伺服器給的去處——加入帶案件 id 與 etag（不送嚴重度），開新案件帶嚴重度；帶讀到的 etag", () => {
	const r = report({ etag: "r-7", triageTargets: TARGETS });
	const join = {
		target: { case: "join", value: { caseId: "c-food", etag: "c-9" } },
	};
	assert.deepEqual(
		buildTriage(r, { category: FOOD, severity: ViolationSeverity.HIGH }),
		{ id: r.id, assessedCategory: FOOD, target: join, etag: "r-7" },
		"加入進行中的案件時，表單上殘留的嚴重度不送",
	);
	const open = (severity: ViolationSeverity) => ({
		target: { case: "openNew", value: { severity } },
	});
	assert.deepEqual(
		buildTriage(r, { category: FRAUD, severity: ViolationSeverity.MEDIUM }),
		{
			id: r.id,
			assessedCategory: FRAUD,
			target: open(ViolationSeverity.MEDIUM),
			etag: "r-7",
		},
	);
	assert.deepEqual(
		buildMove(r, { category: FRAUD, severity: ViolationSeverity.MEDIUM }),
		{
			id: r.id,
			assessedCategory: FRAUD,
			target: open(ViolationSeverity.MEDIUM),
			etag: "r-7",
		},
	);
	assert.deepEqual(
		buildMove(r, { category: FOOD, severity: ViolationSeverity.UNSPECIFIED }),
		{ id: r.id, assessedCategory: FOOD, target: join, etag: "r-7" },
	);
});

test("設定結果：結果必須是伺服器給的 available_outcomes；說明選填，去掉前後空白後最多 2000 字（以字元計）", () => {
	const available = [
		outcome(ReportOutcome.NO_VIOLATION, true),
		outcome(ReportOutcome.DUPLICATE),
	];
	assert.equal(
		validateResolve(
			{ outcome: ReportOutcome.UNSPECIFIED, note: "" },
			available,
		),
		"請選擇結果。",
	);
	assert.equal(
		validateResolve(
			{ outcome: ReportOutcome.ACTION_TAKEN, note: "" },
			available,
		),
		"請選擇結果。",
		"伺服器沒列的結果不能選",
	);
	assert.equal(
		validateResolve({ outcome: ReportOutcome.DUPLICATE, note: "" }, available),
		undefined,
	);
	assert.equal(
		validateResolve(
			{ outcome: ReportOutcome.DUPLICATE, note: `  ${"字".repeat(2000)}  ` },
			available,
		),
		undefined,
	);
	assert.equal(
		validateResolve(
			{ outcome: ReportOutcome.DUPLICATE, note: "字".repeat(2001) },
			available,
		),
		"結果說明最多 2000 字。",
	);
	assert.equal(
		validateResolve(
			{ outcome: ReportOutcome.DUPLICATE, note: "😀".repeat(2000) },
			available,
		),
		undefined,
		"一個 emoji 算一個字（伺服器以 rune 計）",
	);
});

test("設定結果的請求：說明去掉前後空白，帶 etag", () => {
	assert.deepEqual(
		buildResolve(report({ etag: "r-3" }), {
			outcome: ReportOutcome.NO_VIOLATION,
			note: "  查無此事 ",
		}),
		{
			id: report({}).id,
			outcome: ReportOutcome.NO_VIOLATION,
			note: "查無此事",
			etag: "r-3",
		},
	);
});

test("會讓通報不能再當新認定來源的結果：只看伺服器給的 negates_source（client 沒有自己的對照表）", () => {
	// 刻意給一組和伺服器現行規則不同的值：client 照伺服器給的，不照自己記的。
	const options = [
		outcome(ReportOutcome.ACTION_TAKEN, true),
		outcome(ReportOutcome.NO_VIOLATION, false),
		outcome(ReportOutcome.DUPLICATE),
	];
	assert.equal(negatesAsSource(options, ReportOutcome.ACTION_TAKEN), true);
	assert.equal(negatesAsSource(options, ReportOutcome.NO_VIOLATION), false);
	assert.equal(negatesAsSource(options, ReportOutcome.DUPLICATE), false);
	assert.equal(
		negatesAsSource(options, ReportOutcome.ABUSIVE),
		false,
		"伺服器沒列的結果不算",
	);
	assert.equal(negatesAsSource(options, ReportOutcome.UNSPECIFIED), false);
});

test("這筆通報當來源的措施：狀態不是已撤銷的才提醒；提醒的句子說明不會撤銷措施", () => {
	assert.deepEqual(
		livingSourceMeasures([
			{ id: "m1", state: MeasureState.ACTIVE },
			{ id: "m2", state: MeasureState.REVOKED },
			{ id: "m3", state: MeasureState.ENDED },
			{ id: "m4", state: MeasureState.SCHEDULED },
		]),
		["m1", "m3", "m4"],
	);
	assert.equal(
		sourceMeasureNoticeText("m1", ReportOutcome.NO_VIOLATION),
		"這筆通報已是措施 m1 的依據來源。設為未違規不會撤銷那個措施；如果它只依據這筆通報，請到措施詳情撤銷認定。",
	);
});

test("結案：決定必須是伺服器給的 decision_options；說明選填、最多 2000 字；請求帶讀到的 etag", () => {
	const options = [{ decision: CaseDecision.NO_ACTION }];
	assert.equal(
		validateDecide({ decision: CaseDecision.UNSPECIFIED, note: "" }, options),
		"請選擇結案結果。",
	);
	assert.equal(
		validateDecide({ decision: CaseDecision.ACTION_TAKEN, note: "" }, options),
		"請選擇結案結果。",
	);
	assert.equal(
		validateDecide(
			{ decision: CaseDecision.NO_ACTION, note: "字".repeat(2001) },
			options,
		),
		"結果說明最多 2000 字。",
	);
	assert.equal(
		validateDecide({ decision: CaseDecision.NO_ACTION, note: "" }, options),
		undefined,
	);
	assert.deepEqual(
		buildDecide(kase({ etag: "c-9" }), {
			decision: CaseDecision.NO_ACTION,
			note: " 查無異狀 ",
		}),
		{
			id: kase({}).id,
			decision: CaseDecision.NO_ACTION,
			note: "查無異狀",
			etag: "c-9",
		},
	);
});

test("結案的帶入說明：結果的名稱由伺服器給；沒有這樣的通報、或還沒選決定，只說不能重開", () => {
	assert.equal(
		bringInText(3, ReportOutcome.NO_VIOLATION),
		"結案後不能重開。還沒有結果的 3 筆通報會帶入「未違規」。",
	);
	assert.equal(
		bringInText(1, ReportOutcome.ACTION_TAKEN),
		"結案後不能重開。還沒有結果的 1 筆通報會帶入「已採取措施」。",
	);
	assert.equal(bringInText(0, ReportOutcome.NO_VIOLATION), "結案後不能重開。");
	assert.equal(bringInText(3, undefined), "結案後不能重開。");
	assert.equal(
		bringInText(3, ReportOutcome.UNSPECIFIED),
		"結案後不能重開。",
		"伺服器沒給有效的結果（UNSPECIFIED）不顯示「未知」",
	);
});

test("改嚴重度：要選嚴重度、不能和目前相同、原因必填且最多 2000 字", () => {
	const cur = ViolationSeverity.HIGH;
	assert.equal(
		validateSeverity(
			{ severity: ViolationSeverity.UNSPECIFIED, note: "x" },
			cur,
		),
		"請選擇嚴重度。",
	);
	assert.equal(
		validateSeverity({ severity: cur, note: "x" }, cur),
		"嚴重度和目前相同。",
	);
	assert.equal(
		validateSeverity({ severity: ViolationSeverity.LOW, note: "   " }, cur),
		"請填寫原因，最多 2000 字。",
	);
	assert.equal(
		validateSeverity(
			{ severity: ViolationSeverity.LOW, note: "字".repeat(2001) },
			cur,
		),
		"請填寫原因，最多 2000 字。",
	);
	assert.equal(
		validateSeverity(
			{ severity: ViolationSeverity.LOW, note: "字".repeat(2000) },
			cur,
		),
		undefined,
	);
	assert.deepEqual(
		buildChangeSeverity(kase({ etag: "c-2" }), {
			severity: ViolationSeverity.CRITICAL,
			note: " 又有人中毒 ",
		}),
		{
			id: kase({}).id,
			severity: ViolationSeverity.CRITICAL,
			note: "又有人中毒",
			etag: "c-2",
		},
	);
});

test("記錄轉報：管道必選、案號選填最多 100 字；請求不帶 etag，沒有案號送空字串", () => {
	assert.equal(
		validateReferral({ channel: undefined, referenceNo: "", followUp: "" }),
		"請選擇管道。",
	);
	assert.equal(
		validateReferral({
			channel: FDAChannel.FDA_CHANNEL_UNSPECIFIED,
			referenceNo: "",
			followUp: "",
		}),
		"請選擇管道。",
	);
	assert.equal(
		validateReferral({
			channel: FDAChannel.FDA_CHANNEL_HOTLINE_1919,
			referenceNo: "9".repeat(101),
			followUp: "",
		}),
		"案號最多 100 字。",
	);
	assert.equal(
		validateReferral({
			channel: FDAChannel.FDA_CHANNEL_HOTLINE_1919,
			referenceNo: "9".repeat(100),
			followUp: "",
		}),
		undefined,
	);
	assert.deepEqual(
		buildReferral("c1", {
			channel: FDAChannel.FDA_CHANNEL_ONLINE_FORM,
			referenceNo: " FDA-1 ",
			followUp: "",
		}),
		{
			caseId: "c1",
			channel: FDAChannel.FDA_CHANNEL_ONLINE_FORM,
			referenceNo: "FDA-1",
		},
	);
	assert.deepEqual(
		buildReferral("c1", {
			channel: FDAChannel.FDA_CHANNEL_OTHER,
			referenceNo: "",
			followUp: "",
		}),
		{ caseId: "c1", channel: FDAChannel.FDA_CHANNEL_OTHER, referenceNo: "" },
	);
	assert.throws(
		() =>
			buildReferral("c1", {
				channel: undefined,
				referenceNo: "",
				followUp: "",
			}),
		{ message: "請選擇管道。" },
		"萬一沒驗證就送，使用者看到的是已核准的那一句",
	);
});

test("記錄轉報的追蹤日期：選填，YYYY-MM-DD 轉成日期；沒填或不是日期就不帶這個欄位", () => {
	const base = { channel: FDAChannel.FDA_CHANNEL_OTHER, referenceNo: "" };
	assert.deepEqual(buildReferral("c1", { ...base, followUp: "2026-11-18" }), {
		caseId: "c1",
		channel: FDAChannel.FDA_CHANNEL_OTHER,
		referenceNo: "",
		followUpDate: { year: 2026, month: 11, day: 18 },
	});
	for (const bad of ["", "2026-02-30", "11/18"]) {
		assert.ok(
			!("followUpDate" in buildReferral("c1", { ...base, followUp: bad })),
			bad,
		);
	}
});

test("不轉報：原因去掉前後空白；範圍與長度由伺服器判斷", () => {
	assert.deepEqual(buildWaive("c1", "  主管機關已經介入 "), {
		caseId: "c1",
		reason: "主管機關已經介入",
	});
});

test("主管機關的回覆：日期轉成日期、內容去掉前後空白；追蹤日期與不再追蹤原樣送出（衝突由伺服器擋）", () => {
	assert.deepEqual(
		buildReferralReply(
			"c1",
			{
				receiveDate: "2026-10-19",
				body: " 已收到 ",
				nextFollowUp: "2026-11-01",
				clearFollowUp: false,
			},
			"rid-1",
		),
		{
			caseId: "c1",
			requestId: "rid-1",
			body: "已收到",
			receiveDate: { year: 2026, month: 10, day: 19 },
			nextFollowUpDate: { year: 2026, month: 11, day: 1 },
			clearFollowUp: false,
		},
	);
	assert.deepEqual(
		buildReferralReply(
			"c1",
			{
				receiveDate: "2026-10-19",
				body: "結案",
				nextFollowUp: "",
				clearFollowUp: true,
			},
			"rid-2",
		),
		{
			caseId: "c1",
			requestId: "rid-2",
			body: "結案",
			receiveDate: { year: 2026, month: 10, day: 19 },
			clearFollowUp: true,
		},
	);
});

test("案件的註記：去掉前後空白，掛在 case_id", () => {
	assert.deepEqual(buildCreateCaseNote("c1", "  已電話聯繫賣家 "), {
		caseId: "c1",
		note: { body: "已電話聯繫賣家" },
	});
});

test("預先勾選的來源通報超過一次能選的上限時的說明", () => {
	// 哪些通報可以當來源、最早的 20 筆是哪些，都由伺服器的 ListReports(citable_as_source) 決定（use-case-reports）；這裡只剩說明文字。
	assert.equal(
		SOURCE_OVERFLOW_TEXT,
		"這個案件超過 20 筆可以當來源的通報，已預先勾選最早的 20 筆。",
	);
});
