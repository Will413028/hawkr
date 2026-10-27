import assert from "node:assert/strict";
import test from "node:test";
import {
	Code,
	ConnectError,
	createRouterTransport,
	type Transport,
} from "@connectrpc/connect";
import {
	CaseDecision,
	FDAChannel,
	ReportState,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	MeasureState,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	ASSESSABLE_CATEGORIES,
	policyCategoryText,
} from "../src/lib/policy-category";
import {
	bodyText,
	buttonByText,
	byId,
	click,
	dom,
	maybeById,
	mount,
	pressKey,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";
import { kase, outcome, report, target } from "./fixtures/report-fixtures";

/**
 * 商家通報的六個對話框實際掛進 DOM 後的行為（router transport 回真的 protobuf 訊息）：開啟時讀什麼、讀完之前能不能送出、
 * 選項只來自伺服器、驗證訊息、送出的請求形狀、失敗時保留輸入並就地顯示原因、Escape 關閉。
 */

const Triage = requireFromTest(
	"../src/app/admin/violation-reports/[id]/TriageReportDialog",
).default as unknown;
const Move = requireFromTest(
	"../src/app/admin/violation-reports/[id]/MoveReportDialog",
).default as unknown;
const Resolve = requireFromTest(
	"../src/app/admin/violation-reports/[id]/ResolveReportDialog",
).default as unknown;
const Decide = requireFromTest(
	"../src/app/admin/violation-reports/cases/[id]/DecideCaseDialog",
).default as unknown;
const Severity = requireFromTest(
	"../src/app/admin/violation-reports/cases/[id]/ChangeCaseSeverityDialog",
).default as unknown;
const Referral = requireFromTest(
	"../src/app/admin/violation-reports/cases/[id]/RecordFDAReferralDialog",
).default as unknown;

type Handlers = Partial<
	Record<"listCases" | "listMeasures" | "getCase", (req: never) => unknown>
>;

function transportWith(handlers: Handlers): Transport {
	return createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, handlers as never);
	});
}

/** 前 n 次呼叫以網路層失敗回應（像 fetch 斷線：connect-es 把 TypeError 包成 ConnectError），之後交給原本的 transport。 */
function failingFirst(inner: Transport, times: number): Transport {
	let remaining = times;
	return {
		...inner,
		unary: ((...args: Parameters<Transport["unary"]>) => {
			if (remaining > 0) {
				remaining--;
				return Promise.reject(
					ConnectError.from(new TypeError("Failed to fetch")),
				);
			}
			return inner.unary(...args);
		}) as Transport["unary"],
		stream: inner.stream.bind(inner),
	};
}

const requestOf = <T>(requests: T[]) => requests.at(-1) as T;

async function open(
	element: unknown,
	transport: Transport,
	opts: { settle?: boolean } = {},
) {
	const m = mount(element, transport);
	await m.render(opts);
	return m;
}

const h = React.createElement;

function select(id: string): HTMLSelectElement {
	return byId<HTMLSelectElement>(id);
}

function optionTexts(id: string): string[] {
	return [...select(id).options].map((o) => o.textContent ?? "");
}

function pick(id: string, value: number | string) {
	return React.act(async () => {
		setValue(byId(id), String(value));
	});
}

// ---------------------------------------------------------------------------
// 分案

const FOOD = PolicyCategory.FOOD_SAFETY;
const FRAUD = PolicyCategory.FRAUD;

/**
 * 伺服器隨通報給的去處（triage_targets）：每個平台判定類別一筆；食安已有進行中的案件（高、3 筆通報、etag c-9），其餘會開新案件。
 * `except` 是移動時不含的目前類別。
 */
function serverTargets(except?: PolicyCategory) {
	return ASSESSABLE_CATEGORIES.filter((c) => c !== except).map((c) =>
		c === FOOD
			? target(c, {
					caseId: "c-food",
					severity: ViolationSeverity.HIGH,
					etag: "c-9",
					reportCount: 3,
				})
			: target(c),
	);
}

function triageProps(overrides: Record<string, unknown> = {}) {
	const requests: unknown[] = [];
	const closed: string[] = [];
	return {
		requests,
		closed,
		props: {
			report: report({ etag: "r-5", triageTargets: serverTargets() }),
			onClose: () => closed.push("close"),
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
			...overrides,
		},
	};
}

/** 分案與移動不再讀案件清單：去處隨通報來，對話框一開啟就能用，沒有載入與讀取失敗的狀態。 */
const caseListReads: unknown[] = [];
const noReads = transportWith({
	listCases: ((req: unknown) => {
		caseListReads.push(req);
		return { cases: [] };
	}) as never,
});

test("分案：不讀案件清單；一開啟就能送出，類別選項只列伺服器給的去處", async () => {
	const t = triageProps();
	await open(h(Triage, t.props), noReads, { settle: false });
	assert.deepEqual(caseListReads, [], "分案不讀 ListCases");
	assert.doesNotMatch(bodyText(), /載入中/u);
	assert.equal(buttonByText("確認分案")?.disabled, false);
	assert.deepEqual(
		optionTexts("triage-report-category").slice(1),
		ASSESSABLE_CATEGORIES.map((c) => policyCategoryText(c)),
	);
});

test("分案：伺服器只給部分去處時，類別選項就只有這些", async () => {
	const few = triageProps({
		report: report({
			etag: "r-5",
			triageTargets: [target(FRAUD), target(FOOD)],
		}),
	});
	await open(h(Triage, few.props), noReads, { settle: false });
	assert.deepEqual(optionTexts("triage-report-category").slice(1), [
		policyCategoryText(FRAUD),
		policyCategoryText(FOOD),
	]);
});

test("分案：選的類別已有進行中的案件，說明會加入它且不要選嚴重度，請求是加入（帶案件 id 與 etag）", async () => {
	const t = triageProps();
	await open(h(Triage, t.props), noReads, { settle: false });
	assert.match(bodyText(), /檢舉人選的是 食品安全/u);
	await pick("triage-report-category", FOOD);
	assert.match(
		bodyText(),
		/這家店的「食品安全」已有進行中的案件（嚴重度：高，3 筆通報），分案後會加入那個案件。/u,
	);
	assert.ok(maybeById("triage-report-severity") === null);
	await click("確認分案");
	assert.deepEqual(t.requests, [
		{
			id: t.props.report.id,
			assessedCategory: FOOD,
			target: {
				target: { case: "join", value: { caseId: "c-food", etag: "c-9" } },
			},
			etag: "r-5",
		},
	]);
});

test("分案：選的類別沒有進行中的案件，說明會開新案件並要選嚴重度；沒選類別與沒選嚴重度各有驗證訊息；請求是開新案件", async () => {
	const t = triageProps();
	await open(h(Triage, t.props), noReads, { settle: false });
	assert.equal(
		optionTexts("triage-report-category").length,
		13,
		"請選擇＋伺服器給的 12 個平台判定類別",
	);
	assert.ok(
		!optionTexts("triage-report-category").includes("其他"),
		"其他只給檢舉人選，不能當平台判定類別",
	);
	await click("確認分案");
	assert.match(bodyText(), /請選擇平台判定類別。/u);
	await pick("triage-report-category", FRAUD);
	assert.match(
		bodyText(),
		/這家店的「詐欺或操縱交易」沒有進行中的案件，分案後會開一個新案件，請選擇嚴重度。/u,
	);
	await click("確認分案");
	assert.match(bodyText(), /請選擇嚴重度。/u);
	assert.equal(t.requests.length, 0);
	await pick("triage-report-severity", ViolationSeverity.MEDIUM);
	await click("確認分案");
	assert.deepEqual(requestOf(t.requests), {
		id: t.props.report.id,
		assessedCategory: FRAUD,
		target: {
			target: {
				case: "openNew",
				value: { severity: ViolationSeverity.MEDIUM },
			},
		},
		etag: "r-5",
	});
});

test("分案：送出失敗留在原地、保留輸入並就地顯示原因，再送一次可以成功；Escape 關閉", async () => {
	let fail = true;
	const requests: unknown[] = [];
	const closed: string[] = [];
	await open(
		h(Triage, {
			report: report({ triageTargets: serverTargets() }),
			onClose: () => closed.push("close"),
			onConfirm: async (request: unknown) => {
				requests.push(request);
				if (fail) throw new TypeError("Failed to fetch");
			},
		}),
		noReads,
		{ settle: false },
	);
	await pick("triage-report-category", FRAUD);
	await pick("triage-report-severity", ViolationSeverity.LOW);
	await click("確認分案");
	assert.match(
		bodyText(),
		/分案失敗，通報沒有變動；請確認網路狀態後再試一次。/u,
	);
	assert.equal(select("triage-report-category").value, String(FRAUD));
	assert.equal(
		select("triage-report-severity").value,
		String(ViolationSeverity.LOW),
	);
	await pressKey("Escape");
	assert.deepEqual(closed, ["close"], "失敗之後不在送出中，Escape 可以關");
	fail = false;
	await click("確認分案");
	assert.equal(requests.length, 2);
	assert.deepEqual(requests[0], requests[1], "重送的是同一份輸入");
});

test("分案：送出中不能用 Escape 或取消關閉", async () => {
	const closed: string[] = [];
	await open(
		h(Triage, {
			report: report({ triageTargets: serverTargets() }),
			onClose: () => closed.push("close"),
			onConfirm: () => new Promise(() => {}),
		}),
		noReads,
		{ settle: false },
	);
	await pick("triage-report-category", FOOD);
	await click("確認分案");
	assert.ok(buttonByText("送出中…")?.disabled);
	await pressKey("Escape");
	assert.deepEqual(closed, []);
	assert.ok(buttonByText("取消")?.disabled);
});

// ---------------------------------------------------------------------------
// 移到其他案件

test("移動：新的類別只列伺服器給的去處（不含目前的類別）；已有進行中的案件就加入、沒有就要選嚴重度；請求帶 etag", async () => {
	const requests: unknown[] = [];
	const targets = [
		target(FRAUD, {
			caseId: "c-fraud",
			severity: ViolationSeverity.LOW,
			etag: "c-4",
			reportCount: 1,
		}),
		...serverTargets(FOOD).filter((t) => t.category !== FRAUD),
	];
	await open(
		h(Move, {
			report: report({
				state: ReportState.IN_CASE,
				caseId: "c-food",
				assessedCategory: FOOD,
				etag: "r-8",
				triageTargets: targets,
			}),
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
		}),
		noReads,
		{ settle: false },
	);
	const options = optionTexts("move-report-category");
	assert.equal(options.length, 12, "請選擇＋伺服器給的 11 個去處");
	assert.ok(!options.includes("食品安全"));
	await click("確認移動");
	assert.match(bodyText(), /請選擇平台判定類別。/u);
	await pick("move-report-category", FRAUD);
	assert.match(bodyText(), /已有進行中的案件（嚴重度：低，1 筆通報）/u);
	await click("確認移動");
	assert.deepEqual(requests, [
		{
			id: report({}).id,
			assessedCategory: FRAUD,
			target: {
				target: { case: "join", value: { caseId: "c-fraud", etag: "c-4" } },
			},
			etag: "r-8",
		},
	]);
	await pick("move-report-category", PolicyCategory.PRIVACY);
	assert.match(bodyText(), /沒有進行中的案件，分案後會開一個新案件/u);
	assert.ok(maybeById("move-report-severity"));
});

// ---------------------------------------------------------------------------
// 設定結果

function resolveReport(
	patch: Parameters<typeof report>[0] = {},
): ReturnType<typeof report> {
	return report({
		etag: "r-2",
		state: ReportState.IN_CASE,
		availableOutcomes: [
			outcome(ReportOutcome.ACTION_TAKEN),
			outcome(ReportOutcome.NO_VIOLATION, true),
			outcome(ReportOutcome.DUPLICATE),
		],
		...patch,
	});
}

async function openResolve(
	r: ReturnType<typeof report>,
	measures: (req: unknown) => unknown,
	requests: unknown[] = [],
	transportWrap: (t: Transport) => Transport = (t) => t,
) {
	return open(
		h(Resolve, {
			report: r,
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
		}),
		transportWrap(transportWith({ listMeasures: measures as never })),
	);
}

test("設定結果：選項只列伺服器給的 available_outcomes（照它的順序）；案件裡的通報說明會在結案時帶入", async () => {
	await openResolve(resolveReport(), () => ({ measures: [] }));
	assert.deepEqual(optionTexts("resolve-report-outcome"), [
		"請選擇",
		"已採取措施",
		"未違規",
		"重複",
	]);
	assert.match(
		bodyText(),
		/結果設定後不能修改。還沒分案的通報只能直接設為未違規、內容不足、重複或濫報。/u,
	);
	assert.match(
		bodyText(),
		/沒有個別設定結果的通報，會在案件結案時帶入案件的決定。/u,
	);
});

test("設定結果：還沒分案的通報不顯示結案帶入的說明；沒選結果、說明太長各有驗證訊息；請求去掉說明前後空白並帶 etag", async () => {
	const requests: unknown[] = [];
	await openResolve(
		resolveReport({
			state: ReportState.AWAITING_TRIAGE,
			availableOutcomes: [outcome(ReportOutcome.NO_VIOLATION, true)],
		}),
		() => ({ measures: [] }),
		requests,
	);
	assert.doesNotMatch(bodyText(), /結案時帶入案件的決定/u);
	await click("確認設定");
	assert.match(bodyText(), /請選擇結果。/u);
	await pick("resolve-report-outcome", ReportOutcome.NO_VIOLATION);
	await React.act(async () => {
		setValue(byId("resolve-report-note"), "字".repeat(2001));
	});
	await click("確認設定");
	assert.match(bodyText(), /結果說明最多 2000 字。/u);
	await React.act(async () => {
		setValue(byId("resolve-report-note"), "  查無此事 ");
	});
	await click("確認設定");
	assert.deepEqual(requests, [
		{
			id: report({}).id,
			outcome: ReportOutcome.NO_VIOLATION,
			note: "查無此事",
			etag: "r-2",
		},
	]);
});

test("設定結果：選了未違規、內容不足或濫報時說明不能當新認定的來源；這筆通報已是未撤銷措施的來源時並排提醒不會撤銷措施", async () => {
	const requested: unknown[] = [];
	await openResolve(
		resolveReport({
			availableOutcomes: [
				outcome(ReportOutcome.ACTION_TAKEN),
				outcome(ReportOutcome.NO_VIOLATION, true),
				outcome(ReportOutcome.DUPLICATE),
			],
		}),
		(req: unknown) => {
			requested.push(req);
			return {
				measures: [
					{ id: "m-live", state: MeasureState.ACTIVE },
					{ id: "m-revoked", state: MeasureState.REVOKED },
				],
			};
		},
	);
	assert.equal(requested.length, 1, "開啟時讀一次這筆通報當來源的措施");
	assert.equal(
		(requested[0] as { sourceReportId: string }).sourceReportId,
		report({}).id,
	);
	await pick("resolve-report-outcome", ReportOutcome.ACTION_TAKEN);
	assert.doesNotMatch(bodyText(), /不能當新認定的來源/u);
	await pick("resolve-report-outcome", ReportOutcome.DUPLICATE);
	assert.doesNotMatch(bodyText(), /不能當新認定的來源/u);
	await pick("resolve-report-outcome", ReportOutcome.NO_VIOLATION);
	assert.match(
		bodyText(),
		/結果是未違規、內容不足或濫報的通報，不能當新認定的來源。/u,
	);
	assert.match(
		bodyText(),
		/這筆通報已是措施 m-live 的依據來源。設為未違規不會撤銷那個措施；如果它只依據這筆通報，請到措施詳情撤銷認定。/u,
	);
	assert.doesNotMatch(bodyText(), /m-revoked/u, "已撤銷的措施不提醒");
});

test("設定結果：這筆通報當來源的措施超過一頁時讀完所有頁，第二頁的措施也提醒（不悄悄少掉）", async () => {
	const tokens: string[] = [];
	await openResolve(resolveReport(), (req: unknown) => {
		const token = (req as { pageToken: string }).pageToken;
		tokens.push(token);
		return token === ""
			? {
					measures: Array.from({ length: 100 }, (_, i) => ({
						id: `m-old${i}`,
						state: MeasureState.REVOKED,
					})),
					nextPageToken: "p2",
				}
			: {
					measures: [{ id: "m-page2", state: MeasureState.ACTIVE }],
					nextPageToken: "",
				};
	});
	assert.deepEqual(tokens, ["", "p2"]);
	await pick("resolve-report-outcome", ReportOutcome.NO_VIOLATION);
	assert.match(bodyText(), /這筆通報已是措施 m-page2 的依據來源。/u);
});

test("設定結果：讀措施失敗時說明無法確認，仍可送出", async () => {
	const requests: unknown[] = [];
	await openResolve(
		resolveReport({
			availableOutcomes: [outcome(ReportOutcome.NO_VIOLATION, true)],
		}),
		() => {
			throw new ConnectError("down", Code.Unavailable);
		},
		requests,
	);
	assert.match(bodyText(), /措施讀取失敗，無法確認這筆通報是不是措施的來源。/u);
	await pick("resolve-report-outcome", ReportOutcome.NO_VIOLATION);
	await click("確認設定");
	assert.equal(requests.length, 1, "讀取失敗不擋送出");
});

test("設定結果：措施還在讀的時候不能送出", async () => {
	const requests: unknown[] = [];
	await open(
		h(Resolve, {
			report: resolveReport(),
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
		}),
		transportWith({ listMeasures: (() => new Promise(() => {})) as never }),
		{ settle: false },
	);
	assert.ok(buttonByText("確認設定")?.disabled);
});

// ---------------------------------------------------------------------------
// 結案

function decideCase(patch: Parameters<typeof kase>[0] = {}) {
	return kase({
		etag: "c-reread",
		unresolvedReportCount: 2,
		decisionOptions: [
			{
				decision: CaseDecision.ACTION_TAKEN,
				defaultOutcome: ReportOutcome.ACTION_TAKEN,
			},
			{
				decision: CaseDecision.NO_ACTION,
				defaultOutcome: ReportOutcome.NO_VIOLATION,
			},
		],
		canDecide: true,
		...patch,
	});
}

test("結案：開啟時重讀案件，讀完之前不能送出；選項只列重讀到的 decision_options，帶入的結果名稱由伺服器給", async () => {
	const reads: unknown[] = [];
	const requests: unknown[] = [];
	const m = await open(
		h(Decide, {
			caseId: "c-food",
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
		}),
		transportWith({
			getCase: ((req: unknown) => {
				reads.push(req);
				return {
					case: decideCase({
						decisionOptions: [
							{
								decision: CaseDecision.NO_ACTION,
								defaultOutcome: ReportOutcome.NO_VIOLATION,
							},
						],
					}),
				};
			}) as never,
		}),
	);
	assert.equal(reads.length, 1, "開啟時重讀一次案件");
	assert.equal((reads[0] as { id: string }).id, "c-food");
	assert.deepEqual(optionTexts("decide-case-decision"), [
		"請選擇",
		"不採取措施",
	]);
	assert.match(
		bodyText(),
		/採取措施：要有從這個案件建立的措施（已撤銷的也算），或已有通報的結果是已採取措施。不採取措施：從這個案件建立的措施都已撤銷，而且沒有通報的結果是已採取措施。/u,
	);
	assert.match(bodyText(), /^[\s\S]*結案後不能重開。(?!還沒有結果)/u);
	await click("確認結案");
	assert.match(bodyText(), /請選擇結案結果。/u);
	await pick("decide-case-decision", CaseDecision.NO_ACTION);
	assert.match(
		bodyText(),
		/結案後不能重開。還沒有結果的 2 筆通報會帶入「未違規」。/u,
	);
	await React.act(async () => {
		setValue(byId("decide-case-note"), " 查無異狀 ");
	});
	await click("確認結案");
	assert.deepEqual(requests, [
		{
			id: decideCase().id,
			decision: CaseDecision.NO_ACTION,
			note: "查無異狀",
			etag: "c-reread",
		},
	]);
	await m.unmount();
});

test("結案：重讀失敗顯示原因並可重試，之前不能送出", async () => {
	const requests: unknown[] = [];
	await open(
		h(Decide, {
			caseId: "c-food",
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
		}),
		failingFirst(
			transportWith({ getCase: (() => ({ case: decideCase() })) as never }),
			1,
		),
	);
	assert.match(bodyText(), /案件讀取失敗/u);
	assert.ok(buttonByText("確認結案")?.disabled);
	assert.ok(maybeById("decide-case-decision") === null);
	await click("重試");
	await until(
		() => maybeById("decide-case-decision") !== null,
		"重試後出現選項",
	);
	assert.equal(buttonByText("確認結案")?.disabled, false);
});

test("結案：案件在上一次處理之後有新的補充時提醒結案前先看過，不擋送出", async () => {
	const requests: unknown[] = [];
	await open(
		h(Decide, {
			caseId: "c-food",
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
		}),
		transportWith({
			getCase: (() => ({
				case: decideCase({ unseenSupplementReportCount: 2 }),
			})) as never,
		}),
	);
	assert.match(bodyText(), /有 2 筆通報有還沒看過的補充。/u);
	await pick("decide-case-decision", CaseDecision.ACTION_TAKEN);
	await click("確認結案");
	assert.equal(requests.length, 1);
});

test("結案：沒有還沒結果的通報時，帶入說明只說不能重開", async () => {
	await open(
		h(Decide, {
			caseId: "c-food",
			onClose: () => {},
			onConfirm: async () => {},
		}),
		transportWith({
			getCase: (() => ({
				case: decideCase({ unresolvedReportCount: 0 }),
			})) as never,
		}),
	);
	await pick("decide-case-decision", CaseDecision.ACTION_TAKEN);
	assert.doesNotMatch(bodyText(), /還沒有結果的 0 筆/u);
	assert.match(bodyText(), /結案後不能重開。/u);
});

// ---------------------------------------------------------------------------
// 改嚴重度

test("改嚴重度：要選嚴重度、不能和目前相同、原因必填；請求帶原因與 etag", async () => {
	const requests: unknown[] = [];
	await open(
		h(Severity, {
			kase: kase({ severity: ViolationSeverity.HIGH, etag: "c-4" }),
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
		}),
		transportWith({}),
		{ settle: false },
	);
	assert.deepEqual(optionTexts("change-case-severity-value"), [
		"請選擇",
		"低",
		"中",
		"高",
		"嚴重",
	]);
	assert.match(bodyText(), /原因會寫成一則內部註記，只有後台看得到。/u);
	await click("確認修改");
	assert.match(bodyText(), /請選擇嚴重度。/u);
	await pick("change-case-severity-value", ViolationSeverity.HIGH);
	await click("確認修改");
	assert.match(bodyText(), /嚴重度和目前相同。/u);
	await pick("change-case-severity-value", ViolationSeverity.CRITICAL);
	await click("確認修改");
	assert.match(bodyText(), /請填寫原因，最多 2000 字。/u);
	await React.act(async () => {
		setValue(byId("change-case-severity-note"), " 又有人送醫 ");
	});
	await click("確認修改");
	assert.deepEqual(requests, [
		{
			id: kase({}).id,
			severity: ViolationSeverity.CRITICAL,
			note: "又有人送醫",
			etag: "c-4",
		},
	]);
});

test("改嚴重度：送出失敗保留輸入並顯示原因", async () => {
	await open(
		h(Severity, {
			kase: kase({}),
			onClose: () => {},
			onConfirm: async () => {
				throw new TypeError("Failed to fetch");
			},
		}),
		transportWith({}),
		{ settle: false },
	);
	await pick("change-case-severity-value", ViolationSeverity.LOW);
	await React.act(async () => {
		setValue(byId("change-case-severity-note"), "風險降低");
	});
	await click("確認修改");
	assert.match(
		bodyText(),
		/改嚴重度失敗，案件沒有變動；請確認網路狀態後再試一次。/u,
	);
	assert.equal(
		byId<HTMLTextAreaElement>("change-case-severity-note").value,
		"風險降低",
	);
});

// ---------------------------------------------------------------------------
// 記錄已轉報主管機關

test("記錄轉報：管道必選、案號選填最多 100 字；請求不帶 etag；標題是「記錄已轉報主管機關」、送出鍵是「記錄轉報」（Will 2026-10-18）", async () => {
	const requests: unknown[] = [];
	await open(
		h(Referral, {
			caseId: "c-food",
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
			},
		}),
		transportWith({}),
		{ settle: false },
	);
	assert.deepEqual(optionTexts("record-fda-referral-channel"), [
		"請選擇",
		"1919 食安專線",
		"線上表單",
		"傳真衛生局",
		"親自送件",
		"其他",
	]);
	assert.match(
		bodyText(),
		/轉報本身在系統外完成；這裡只記一次，記過之後不能修改。/u,
	);
	assert.match(bodyText(), /主管機關的案號（選填）/u);
	assert.equal(
		dom.window.document.querySelector(".modal-header h3")?.textContent,
		"記錄已轉報主管機關",
	);
	assert.ok(buttonByText("記錄轉報"), "送出鍵");
	await click("記錄轉報");
	assert.match(bodyText(), /請選擇管道。/u);
	await pick(
		"record-fda-referral-channel",
		FDAChannel.FDA_CHANNEL_FAX_HEALTH_DEPT,
	);
	await React.act(async () => {
		setValue(byId("record-fda-referral-reference-no"), "9".repeat(101));
	});
	await click("記錄轉報");
	assert.match(bodyText(), /案號最多 100 字。/u);
	await React.act(async () => {
		setValue(byId("record-fda-referral-reference-no"), " FAX-7 ");
	});
	await click("記錄轉報");
	assert.deepEqual(requests, [
		{
			caseId: "c-food",
			channel: FDAChannel.FDA_CHANNEL_FAX_HEALTH_DEPT,
			referenceNo: "FAX-7",
		},
	]);
});

test("記錄轉報：沒有案號也能記（送空字串）；失敗保留輸入", async () => {
	let fail = true;
	const requests: unknown[] = [];
	await open(
		h(Referral, {
			caseId: "c-food",
			onClose: () => {},
			onConfirm: async (request: unknown) => {
				requests.push(request);
				if (fail) throw new TypeError("Failed to fetch");
			},
		}),
		transportWith({}),
		{ settle: false },
	);
	await pick(
		"record-fda-referral-channel",
		FDAChannel.FDA_CHANNEL_HOTLINE_1919,
	);
	await click("記錄轉報");
	assert.match(
		bodyText(),
		/記錄轉報失敗，案件沒有變動；請確認網路狀態後再試一次。/u,
	);
	assert.equal(
		select("record-fda-referral-channel").value,
		String(FDAChannel.FDA_CHANNEL_HOTLINE_1919),
	);
	fail = false;
	await click("記錄轉報");
	assert.deepEqual(requestOf(requests), {
		caseId: "c-food",
		channel: FDAChannel.FDA_CHANNEL_HOTLINE_1919,
		referenceNo: "",
	});
});

test("六個對話框都是 role=dialog、aria-modal，標題用 aria-labelledby 指到；Escape 關閉", async () => {
	const closed: string[] = [];
	const onClose = () => closed.push("x");
	const cases: [unknown, Record<string, unknown>, string, Handlers][] = [
		[Triage, { report: report({}), onConfirm: async () => {} }, "分案", {}],
		[
			Move,
			{
				report: report({ assessedCategory: PolicyCategory.FRAUD }),
				onConfirm: async () => {},
			},
			"移到其他案件",
			{},
		],
		[
			Resolve,
			{ report: resolveReport(), onConfirm: async () => {} },
			"設定結果",
			{ listMeasures: (() => ({ measures: [] })) as never },
		],
		[
			Decide,
			{ caseId: "c1", onConfirm: async () => {} },
			"結案",
			{ getCase: (() => ({ case: decideCase() })) as never },
		],
		[Severity, { kase: kase({}), onConfirm: async () => {} }, "改嚴重度", {}],
		[
			Referral,
			{ caseId: "c1", onConfirm: async () => {} },
			"記錄已轉報主管機關",
			{},
		],
	];
	for (const [Dialog, props, title, handlers] of cases) {
		const m = await open(
			h(Dialog, { ...props, onClose }),
			transportWith(handlers),
			{
				settle: Object.keys(handlers).length > 0,
			},
		);
		const dialog = dom.window.document.querySelector('[role="dialog"]');
		assert.ok(dialog, `${title}：找不到 dialog`);
		assert.equal(dialog.getAttribute("aria-modal"), "true");
		const labelled = dom.window.document.getElementById(
			dialog.getAttribute("aria-labelledby") ?? "",
		);
		assert.equal(labelled?.textContent, title);
		const before = closed.length;
		await pressKey("Escape");
		assert.equal(closed.length, before + 1, `${title}：Escape 要關閉`);
		await m.unmount();
	}
});
