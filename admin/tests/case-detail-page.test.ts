import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	Code,
	ConnectError,
	createRouterTransport,
	type Transport,
} from "@connectrpc/connect";
import {
	CaseCloseReason,
	CaseDecision,
	CaseState,
	FDAChannel,
	FDAReferralSchema,
	FDAReferralWaiverSchema,
	GovernanceNoteSchema,
	ReportMoveSchema,
	ReportState,
	SellerGovernanceAdminService,
	SeverityChangeSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	MeasureKind,
	MeasureState,
	RiskGround,
	TerminationGround,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import {
	bodyText,
	buttonByText,
	byId,
	click,
	dom,
	downloads,
	maybeById,
	mount,
	navigation,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";
import {
	CASE_ID,
	kase,
	outcome,
	report,
	STORE_ID,
	T0,
	target,
	ts,
	versionConflict,
	withMessage,
} from "./fixtures/report-fixtures";

/**
 * 案件詳情頁（/admin/violation-reports/cases/[id]）實際掛進 DOM 後的行為：讀取中與失敗、各區塊、按鈕只看伺服器的 can_*、
 * 從案件開三種建立措施（頂端說明、來源案件進請求、預先勾選最早的 20 筆來源通報）、結案、記錄轉報、下載轉報資料、
 * 案件裡通報的設定結果與版本衝突。
 */

const Page = requireFromTest(
	"../src/app/admin/violation-reports/cases/[id]/page",
).default as unknown;
const h = React.createElement;

type Req = Record<string, unknown>;
type Handler = (req: Req) => unknown;

interface Services {
	gov?: Record<string, Handler>;
	seller?: Record<string, Handler>;
}

function transportWith(services: Services): Transport {
	return createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listReports: () => ({ reports: [] }),
			listMeasures: () => ({ measures: [] }),
			listCaseNotes: () => ({ notes: [] }),
			listCaseInquiries: () => ({ inquiries: [] }),
			listViolations: () => ({ violations: [] }),
			...services.gov,
		} as never);
		service(SellerAdminService, {
			getStore: () => ({
				store: { id: STORE_ID, accountId: "acc-1" },
				canCreateTermination: false,
				availableTerminations: [],
			}),
			...services.seller,
		} as never);
	});
}

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

let current: { unmount: () => Promise<void> } | undefined;

/** 掛載頁面；同一個測試開第二個頁面時，前一個先卸載（畫面上同時只有一個頁面）。 */
async function openPage(transport: Transport, opts: { settle?: boolean } = {}) {
	await current?.unmount();
	navigation.params = { id: CASE_ID };
	const m = mount(h(Page), transport);
	current = m;
	await m.render(opts);
	return m;
}

const ACTION_LABELS = [
	"結案",
	"建立措施",
	"建立風險措施",
	"終止合作",
	"改嚴重度",
	"下載轉報資料（.md）",
	"記錄已轉報主管機關",
];

const actionButtons = () =>
	[...dom.window.document.querySelectorAll(".header-actions button")].map(
		(b) => b.textContent,
	);

const ALL_CAN = {
	canDecide: true,
	canChangeSeverity: true,
	canLinkMeasure: true,
	canCreateMeasure: true,
	canRecordFdaReferral: true,
	canExportFda: true,
};

const TERMINATION_STORE = {
	store: { id: STORE_ID, accountId: "acc-1" },
	canCreateTermination: true,
	availableTerminations: [
		{
			ground: TerminationGround.NOTICE,
			explanation: { locale: "zh-TW", message: "三十天後生效。" },
		},
	],
};

const sourceBoxes = () =>
	[
		...dom.window.document.querySelectorAll('fieldset input[type="checkbox"]'),
	] as HTMLInputElement[];

/** 建立措施對話框的來源通報清單讀完（載入中的提示消失）。 */
const sourcesLoaded = () =>
	maybeById("create-violation-measure-kind") !== null &&
	!bodyText().includes("載入通報中...");

function pick(id: string, value: number | string) {
	return React.act(async () => {
		setValue(byId(id), String(value));
	});
}

function fill(id: string, value: string) {
	return React.act(async () => {
		setValue(byId(id), value);
	});
}

test("讀取中顯示「載入案件中...」；讀取失敗顯示伺服器的句子（網路層失敗是「案件讀取失敗」）並可重試", async () => {
	await openPage(
		transportWith({ gov: { getCase: () => new Promise(() => {}) } }),
		{
			settle: false,
		},
	);
	assert.match(bodyText(), /載入案件中\.\.\./u);

	let calls = 0;
	await openPage(
		transportWith({
			gov: {
				getCase: () => {
					calls++;
					if (calls === 1) throw withMessage(Code.NotFound, "找不到這個案件。");
					return { case: kase({}) };
				},
			},
		}),
	);
	assert.match(bodyText(), /找不到這個案件。/u);
	await click("重試");
	await until(() => bodyText().includes("案件詳情"), "重試後出現內容");

	await openPage(
		failingFirst(
			transportWith({ gov: { getCase: () => ({ case: kase({}) }) } }),
			1,
		),
	);
	assert.match(bodyText(), /案件讀取失敗/u);
});

test("標題與案件區塊：店家、類別、嚴重度、狀態、開案與結案、給檢舉人的結果說明", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({
						state: CaseState.CLOSED,
						closeReason: CaseCloseReason.DECIDED,
						decision: CaseDecision.ACTION_TAKEN,
						decisionNote: "已停權並轉報",
						closedByAdminId: "9002",
						closeTime: ts(new Date(T0.getTime() + 3600_000)),
					}),
				}),
			},
		}),
	);
	const text = bodyText();
	assert.match(text, /案件詳情/u);
	assert.match(
		text,
		new RegExp(`查看店家・案件 ${CASE_ID}・開案於 2026/10/18 10:00`, "u"),
	);
	assert.match(text, /店家阿里山農場/u);
	assert.match(text, /類別食品安全/u);
	assert.match(text, /嚴重度高/u);
	assert.match(text, /狀態已結案：採取措施/u);
	assert.match(text, /開案9001・2026\/10\/18 10:00/u);
	assert.match(text, /結案9002・2026\/10\/18 11:00/u);
	assert.match(text, /給檢舉人的結果說明已停權並轉報/u);
});

test("通報都移走而自動關閉的案件說明不算一次決定；進行中的案件結案欄是「-」", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({
						state: CaseState.CLOSED,
						closeReason: CaseCloseReason.NO_REPORTS,
						reportCount: 0,
					}),
				}),
			},
		}),
	);
	assert.match(bodyText(), /已結案：沒有通報/u);
	assert.match(
		bodyText(),
		/這個案件的通報都已移到其他案件，案件已自動關閉，不算一次決定。/u,
	);
	assert.match(bodyText(), /通報（0 筆）/u);
});

test("按鈕只看 can_*：全部可以時七個入口都在", async () => {
	await openPage(
		transportWith({
			gov: { getCase: () => ({ case: kase(ALL_CAN) }) },
			seller: { getStore: () => TERMINATION_STORE },
		}),
	);
	await until(
		() => actionButtons().includes("終止合作"),
		"店家讀完後出現終止合作",
	);
	assert.deepEqual(actionButtons(), ACTION_LABELS);
});

test("按鈕只看 can_*：什麼都不能做的案件沒有任何入口，也沒有「為什麼不能」的說明", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({
						state: CaseState.CLOSED,
						closeReason: CaseCloseReason.DECIDED,
						decision: CaseDecision.NO_ACTION,
					}),
				}),
			},
			seller: { getStore: () => TERMINATION_STORE },
		}),
	);
	assert.deepEqual(actionButtons(), []);
	assert.doesNotMatch(bodyText(), /不能操作|無法/u);
});

test("終止合作：店家已關閉（can_create_measure 為 false）時仍可從案件終止合作，只看 can_link_measure 與店家的 can_create_termination", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({ canLinkMeasure: true, canCreateMeasure: false }),
				}),
			},
			seller: { getStore: () => TERMINATION_STORE },
		}),
	);
	await until(() => actionButtons().includes("終止合作"), "終止合作");
	assert.deepEqual(actionButtons(), ["終止合作"]);
});

test("終止合作：店家資料讀取中按鈕不出現；店家不能終止（can_create_termination 為 false）也不出現", async () => {
	await openPage(
		transportWith({
			gov: { getCase: () => ({ case: kase(ALL_CAN) }) },
			seller: { getStore: () => new Promise(() => {}) },
		}),
		{ settle: false },
	);
	await until(() => buttonByText("結案") !== undefined, "案件讀完");
	assert.ok(buttonByText("終止合作") === undefined);

	await openPage(
		transportWith({
			gov: { getCase: () => ({ case: kase(ALL_CAN) }) },
			seller: {
				getStore: () => ({
					store: { id: STORE_ID, accountId: "acc-1" },
					canCreateTermination: false,
					availableTerminations: [],
				}),
			},
		}),
	);
	assert.ok(buttonByText("終止合作") === undefined);
});

test("終止合作：店家資料讀取失敗顯示「店家資料讀取失敗」並可重試，重試成功後出現入口", async () => {
	await openPage(
		failingFirstOn(
			transportWith({
				gov: { getCase: () => ({ case: kase(ALL_CAN) }) },
				seller: { getStore: () => TERMINATION_STORE },
			}),
			"GetStore",
			1,
		),
	);
	await until(() => bodyText().includes("店家資料讀取失敗"), "店家讀取失敗");
	assert.ok(buttonByText("終止合作") === undefined);
	const retry = [
		...dom.window.document.querySelectorAll(".header-actions button"),
	].find((b) => b.textContent === "重試");
	assert.ok(retry);
	await React.act(async () => {
		(retry as HTMLElement).click();
	});
	await until(
		() => buttonByText("終止合作") !== undefined,
		"重試後出現終止合作",
	);
	assert.doesNotMatch(bodyText(), /店家資料讀取失敗/u);
});

function failingFirstOn(
	inner: Transport,
	method: string,
	times: number,
): Transport {
	let remaining = times;
	return {
		...inner,
		unary: ((...args: Parameters<Transport["unary"]>) => {
			if (remaining > 0 && args[0].name === method) {
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

test("有通報的補充還沒看過時頁首提示筆數（伺服器的 unseen_supplement_report_count）；沒有就不顯示", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({ unseenSupplementReportCount: 2 }) }),
			},
		}),
	);
	assert.match(bodyText(), /有 2 筆通報有還沒看過的補充。/u);
	assert.doesNotMatch(bodyText(), /上一次處理之後/u, "舊的說法已移除");
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({ unseenSupplementReportCount: 0 }) }),
			},
		}),
	);
	assert.doesNotMatch(bodyText(), /還沒看過的補充/u);
});

test("案件裡的通報列：只有伺服器說有還沒看過的補充（has_unseen_supplements）的那一筆有「新的補充」", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({ reportCount: 2, unseenSupplementReportCount: 1 }),
				}),
				listReports: () => ({
					reports: [
						report({
							id: "r-a",
							state: ReportState.IN_CASE,
							caseId: CASE_ID,
							hasUnseenSupplements: true,
							supplementCount: 1,
						}),
						report({
							id: "r-b",
							state: ReportState.IN_CASE,
							caseId: CASE_ID,
							supplementCount: 1,
						}),
					],
				}),
			},
		}),
	);
	await until(
		() => dom.window.document.querySelectorAll("li").length >= 2,
		"通報列",
	);
	const rows = [...dom.window.document.querySelectorAll("li")];
	const badged = rows.filter((li) => li.textContent?.includes("新的補充"));
	assert.equal(badged.length, 1);
	assert.ok(
		badged[0].querySelector('a[href$="/r-a"]') !== null,
		"標在 r-a 那一列",
	);
});

test("案件裡的通報：每筆有摘要、狀態、補充與連結，設定結果與移到其他案件看那筆的 can_*", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({ reportCount: 2 }) }),
				listReports: () => ({
					reports: [
						report({
							id: "r-a",
							state: ReportState.IN_CASE,
							caseId: CASE_ID,
							canResolve: true,
							canMove: true,
							supplementCount: 2,
							latestSupplementTime: ts(T0),
						}),
						report({
							id: "r-b",
							state: ReportState.RESOLVED,
							outcome: ReportOutcome.ACTION_TAKEN,
							caseId: CASE_ID,
						}),
					],
				}),
			},
		}),
	);
	const rows = [...dom.window.document.querySelectorAll("#case-reports-title")]
		.length;
	assert.equal(rows, 1);
	const items = [
		...dom.window.document.querySelectorAll(
			'section[aria-labelledby="case-reports-title"] li',
		),
	];
	assert.equal(items.length, 2);
	assert.match(
		items[0].textContent ?? "",
		/通報 r-a・食品安全・建立 2026\/10\/18 10:00處理中補充 2 則・最新 2026\/10\/18 10:00設定結果移到其他案件/u,
	);
	assert.match(
		items[1].textContent ?? "",
		/通報 r-b・食品安全・建立 2026\/10\/18 10:00已採取措施$/u,
	);
	assert.equal(
		items[0].querySelector("a")?.getAttribute("href"),
		"/admin/violation-reports/r-a",
	);
	assert.match(bodyText(), /通報（2 筆）/u);
});

test("案件裡通報的設定結果：開對話框送出；版本衝突時重讀案件裡的通報並顯示通報的衝突句", async () => {
	let reads = 0;
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({}) }),
				listReports: () => {
					reads++;
					return {
						reports: [
							report({
								id: "r-a",
								state: ReportState.IN_CASE,
								caseId: CASE_ID,
								canResolve: true,
								availableOutcomes: [
									outcome(ReportOutcome.ACTION_TAKEN),
									outcome(ReportOutcome.DUPLICATE),
								],
								etag: `r-${reads}`,
							}),
						],
					};
				},
				resolveReport: () => {
					throw versionConflict();
				},
			},
		}),
	);
	await click("設定結果");
	await until(() => maybeById("resolve-report-outcome") !== null, "對話框");
	await until(() => buttonByText("確認設定")?.disabled === false, "措施讀完");
	await pick("resolve-report-outcome", ReportOutcome.DUPLICATE);
	const before = reads;
	await click("確認設定");
	await until(
		() =>
			bodyText().includes(
				"這筆通報剛被更新，頁面已重新載入；請確認最新狀態後再操作。",
			),
		"通報的衝突句",
	);
	assert.ok(reads > before, "衝突時重讀案件裡的通報");
	assert.equal(
		byId<HTMLSelectElement>("resolve-report-outcome").value,
		String(ReportOutcome.DUPLICATE),
	);
});

test("移動紀錄：站在案件看，移進來的與移出去的；沒有移動時沒有這個區塊", async () => {
	const move = (from: string, to: string, report: string) =>
		create(ReportMoveSchema, {
			reportId: report,
			fromCaseId: from,
			toCaseId: to,
			movedByAdminId: "9001",
			moveTime: ts(T0),
		});
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({
						moves: [
							move("c-old", CASE_ID, "r-in"),
							move(CASE_ID, "c-new", "r-out"),
						],
					}),
				}),
			},
		}),
	);
	assert.match(
		bodyText(),
		/通報 r-in 於 2026\/10\/18 10:00 從案件 c-old 移入（9001）/u,
	);
	assert.match(
		bodyText(),
		/通報 r-out 於 2026\/10\/18 10:00 移到案件 c-new（9001）/u,
	);

	await openPage(
		transportWith({ gov: { getCase: () => ({ case: kase({}) }) } }),
	);
	assert.ok(maybeById("case-moves-title") === null);
});

test("轉報主管機關：有紀錄或伺服器允許某個動作才顯示，不看案件類別；還沒記錄、已記錄（管道與案號）", async () => {
	await openPage(
		transportWith({
			gov: { getCase: () => ({ case: kase({ canRecordFdaReferral: true }) }) },
		}),
	);
	assert.match(bodyText(), /轉報主管機關/u);
	assert.match(bodyText(), /還沒有記錄轉報/u);

	// 食安案件，但伺服器什麼都不能做、也沒有紀錄：不顯示空的區塊。
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({ category: PolicyCategory.FOOD_SAFETY }),
				}),
			},
		}),
	);
	assert.ok(
		maybeById("case-referral-title") === null,
		"食安但沒有 can_* 與紀錄：不顯示區塊",
	);

	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({
						fdaReferral: create(FDAReferralSchema, {
							channel: FDAChannel.FDA_CHANNEL_ONLINE_FORM,
							referenceNo: "FDA-9",
							reportTime: ts(T0),
						}),
					}),
				}),
			},
		}),
	);
	assert.match(
		bodyText(),
		/已於 2026\/10\/18 10:00 記錄轉報（線上表單，案號 FDA-9）/u,
	);

	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({ category: PolicyCategory.FRAUD }) }),
			},
		}),
	);
	assert.ok(
		maybeById("case-referral-title") === null,
		"沒有 can_* 與紀錄的案件沒有轉報區塊",
	);
});

test("轉報區塊的顯示條件：以下任何一項單獨成立就顯示（不看案件類別），全部沒有才不顯示", async () => {
	const base = { category: PolicyCategory.FRAUD };
	for (const [name, patch] of [
		[
			"只有轉報紀錄",
			{
				fdaReferral: create(FDAReferralSchema, {
					channel: FDAChannel.FDA_CHANNEL_ONLINE_FORM,
					referenceNo: "FDA-1",
					reportTime: ts(T0),
				}),
			},
		],
		[
			"只有不轉報紀錄",
			{
				fdaReferralWaiver: create(FDAReferralWaiverSchema, {
					reason: "已介入",
					waiveTime: ts(T0),
				}),
			},
		],
		["只能記錄轉報", { canRecordFdaReferral: true }],
		["只能標記不轉報", { canWaiveFdaReferral: true }],
		["只能記下回覆", { canRecordFdaReferralReply: true }],
		["只能匯出", { canExportFda: true }],
	] as const) {
		await openPage(
			transportWith({
				gov: { getCase: () => ({ case: kase({ ...base, ...patch }) }) },
			}),
		);
		assert.ok(maybeById("case-referral-title"), `${name}：顯示轉報區塊`);
	}
	await openPage(
		transportWith({ gov: { getCase: () => ({ case: kase(base) }) } }),
	);
	assert.ok(maybeById("case-referral-title") === null, "全部沒有：不顯示");
});

test("導出的措施：列出「{種類}・{狀態}・措施 {id}」連到措施詳情；沒有就說還沒有；讀取失敗可重試", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({}) }),
				listMeasures: (req) => {
					assert.equal(req.sourceCaseId, CASE_ID);
					return {
						measures: [
							{
								id: "m-1",
								kind: MeasureKind.SUSPENSION,
								state: MeasureState.ACTIVE,
							},
							{
								id: "m-2",
								kind: MeasureKind.WARNING,
								state: MeasureState.REVOKED,
							},
						],
					};
				},
			},
		}),
	);
	await until(
		() =>
			dom.window.document.querySelectorAll('a[href^="/admin/measures/"]')
				.length === 2,
		"導出的措施列出來",
	);
	assert.match(
		bodyText(),
		/包含從這個案件建立的措施，以及依據的認定引用了這個案件的通報的措施。/u,
	);
	const links = [
		...dom.window.document.querySelectorAll('a[href^="/admin/measures/"]'),
	].map((a) => [a.textContent, a.getAttribute("href")]);
	assert.deepEqual(links, [
		["停權・停權中・措施 m-1", "/admin/measures/m-1"],
		["警告・已撤銷・措施 m-2", "/admin/measures/m-2"],
	]);

	await openPage(
		transportWith({ gov: { getCase: () => ({ case: kase({}) }) } }),
	);
	await until(() => bodyText().includes("還沒有導出的措施"), "沒有導出的措施");

	let calls = 0;
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({}) }),
				listMeasures: () => {
					calls++;
					if (calls === 1) throw new ConnectError("down", Code.Unavailable);
					return { measures: [] };
				},
			},
		}),
	);
	await until(
		() => bodyText().includes(defaultErrorTextZhTW(Code.Unavailable)),
		"導出的措施讀取失敗",
	);
	const retry = [
		...dom.window.document.querySelectorAll(
			'section[aria-labelledby="case-measures-title"] button',
		),
	].find((b) => b.textContent === "重試");
	assert.ok(retry);
});

test("導出的措施超過一頁：把所有頁讀完，不悄悄少掉第 100 筆以後的", async () => {
	const pages: Record<string, { measures: Req[]; nextPageToken: string }> = {
		"": {
			measures: Array.from({ length: 100 }, (_, i) => ({
				id: `m-a${i}`,
				kind: MeasureKind.WARNING,
				state: MeasureState.ACTIVE,
			})),
			nextPageToken: "p2",
		},
		p2: {
			measures: [
				{
					id: "m-b0",
					kind: MeasureKind.SUSPENSION,
					state: MeasureState.ACTIVE,
				},
			],
			nextPageToken: "",
		},
	};
	const tokens: string[] = [];
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({}) }),
				listMeasures: (req) => {
					tokens.push(req.pageToken as string);
					return pages[req.pageToken as string];
				},
			},
		}),
	);
	await until(
		() =>
			dom.window.document.querySelectorAll('a[href^="/admin/measures/"]')
				.length === 101,
		"兩頁的措施都列出來",
	);
	assert.deepEqual(tokens, ["", "p2"]);
	assert.ok(
		dom.window.document.querySelector('a[href="/admin/measures/m-b0"]'),
		"第二頁的措施也列出",
	);
});

test("案件裡通報的移到其他案件：先讀這筆通報（去處只有單筆讀取才帶）才開對話框，請求把去處原樣送回；讀取失敗顯示在頁面上", async () => {
	const moved: Req[] = [];
	let detailCalls = 0;
	const row = report({
		id: "r-a",
		state: ReportState.IN_CASE,
		caseId: CASE_ID,
		assessedCategory: PolicyCategory.FOOD_SAFETY,
		canMove: true,
		etag: "r-list",
	});
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({ reportCount: 1 }) }),
				listReports: () => ({ reports: [row] }),
				getReport: () => {
					detailCalls++;
					if (detailCalls === 1)
						throw new ConnectError("down", Code.Unavailable);
					return {
						report: report({
							id: "r-a",
							state: ReportState.IN_CASE,
							caseId: CASE_ID,
							assessedCategory: PolicyCategory.FOOD_SAFETY,
							canMove: true,
							etag: "r-detail",
							triageTargets: [
								target(PolicyCategory.FRAUD, {
									caseId: "c-fraud",
									severity: ViolationSeverity.LOW,
									etag: "c-3",
									reportCount: 2,
								}),
								target(PolicyCategory.PRIVACY),
							],
						}),
					};
				},
				moveReport: (req) => {
					moved.push(req);
					return { report: row, sourceCaseClosed: false };
				},
			},
		}),
	);
	await click("移到其他案件");
	await until(
		() => bodyText().includes(defaultErrorTextZhTW(Code.Unavailable)),
		"讀這筆通報失敗：顯示原因，不開對話框",
	);
	assert.ok(maybeById("move-report-category") === null);
	await click("移到其他案件");
	await until(() => maybeById("move-report-category") !== null, "對話框");
	assert.deepEqual(
		[...byId<HTMLSelectElement>("move-report-category").options].map(
			(o) => o.textContent,
		),
		["請選擇", "詐欺或操縱交易", "不當使用個人資料"],
		"類別選項只有伺服器給的去處",
	);
	await pick("move-report-category", PolicyCategory.FRAUD);
	await click("確認移動");
	await until(() => moved.length === 1, "送出");
	const sent = moved[0].target as {
		target: { case: string; value: { caseId: string; etag: string } };
	};
	assert.equal(moved[0].etag, "r-detail", "用單筆讀到的 etag，不是清單那筆的");
	assert.equal(sent.target.case, "join");
	assert.equal(sent.target.value.caseId, "c-fraud");
	assert.equal(sent.target.value.etag, "c-3");
});

/** 案件裡一筆通報的移動：單筆讀取的去處依 etag 版本而變，moveReport 只收最新的 etag。 */
function moveConflictServices(opts: {
	rereadFails?: boolean;
	conflictTarget?: "report" | "case" | "target_case";
}) {
	const moved: Req[] = [];
	let detailCalls = 0;
	const row = report({
		id: "r-a",
		state: ReportState.IN_CASE,
		caseId: CASE_ID,
		assessedCategory: PolicyCategory.FOOD_SAFETY,
		canMove: true,
		etag: "r-list",
	});
	const detail = (etag: string, targets: ReturnType<typeof target>[]) =>
		report({
			id: "r-a",
			state: ReportState.IN_CASE,
			caseId: CASE_ID,
			assessedCategory: PolicyCategory.FOOD_SAFETY,
			canMove: true,
			etag,
			triageTargets: targets,
		});
	const services = transportWith({
		gov: {
			getCase: () => ({ case: kase({ reportCount: 1 }) }),
			listReports: () => ({ reports: [row] }),
			getReport: () => {
				detailCalls++;
				if (detailCalls === 1) {
					// 讀到的時候這個類別還沒有案件：開新案件。
					return { report: detail("r-1", [target(PolicyCategory.FRAUD)]) };
				}
				if (opts.rereadFails) throw new ConnectError("down", Code.Unavailable);
				// 衝突之後再讀：別人先開了詐欺案件，去處變成加入它，通報的 etag 也前進了。
				return {
					report: detail("r-2", [
						target(PolicyCategory.FRAUD, {
							caseId: "c-fraud",
							severity: ViolationSeverity.LOW,
							etag: "c-9",
							reportCount: 1,
						}),
					]),
				};
			},
			moveReport: (req) => {
				moved.push(req);
				if (req.etag !== "r-2" || opts.conflictTarget) {
					throw versionConflict(opts.conflictTarget ?? "report");
				}
				return { report: row, sourceCaseClosed: false };
			},
		},
	});
	return { services, moved };
}

test("案件頁移到其他案件遇到版本衝突：重讀對話框那一筆通報（新的 etag 與去處），同一個對話框重送成功；衝突句如實", async () => {
	const { services, moved } = moveConflictServices({});
	await openPage(services);
	await click("移到其他案件");
	await until(() => maybeById("move-report-category") !== null, "對話框");
	await pick("move-report-category", PolicyCategory.FRAUD);
	assert.match(bodyText(), /沒有進行中的案件/u, "讀到的去處是開新案件");
	await pick("move-report-severity", ViolationSeverity.LOW);
	await click("確認移動");
	await until(
		() => bodyText().includes("這筆通報剛被更新，頁面已重新載入"),
		"通報的衝突句",
	);
	assert.equal(moved.length, 1);
	assert.equal(moved[0].etag, "r-1");
	assert.ok(maybeById("move-report-category") !== null, "對話框留著");
	assert.match(
		bodyText(),
		/已有進行中的案件/u,
		"對話框的去處說明跟著重讀的通報更新",
	);
	// 使用者看到新的去處之後重新確認：重送帶的是重讀後的 etag 與新的去處。
	await pick("move-report-category", PolicyCategory.FRAUD);
	await click("確認移動");
	await until(() => maybeById("move-report-category") === null, "對話框關閉");
	assert.equal(moved.length, 2);
	assert.equal(moved[1].etag, "r-2");
	const sent = moved[1].target as {
		target: { case: string; value: { caseId: string; etag: string } };
	};
	assert.equal(sent.target.case, "join");
	assert.equal(sent.target.value.caseId, "c-fraud");
	assert.equal(sent.target.value.etag, "c-9");
});

test("案件頁移到其他案件遇到版本衝突而重讀失敗：不宣稱已重新載入，改用伺服器自己的句子", async () => {
	const { services } = moveConflictServices({ rereadFails: true });
	await openPage(services);
	await click("移到其他案件");
	await until(() => maybeById("move-report-category") !== null, "對話框");
	await pick("move-report-category", PolicyCategory.FRAUD);
	await pick("move-report-severity", ViolationSeverity.LOW);
	await click("確認移動");
	await until(
		() => bodyText().includes(defaultErrorTextZhTW(Code.Aborted)),
		"伺服器的衝突句（沒有 LocalizedMessage 時是目錄的預設句）",
	);
	assert.doesNotMatch(bodyText(), /已重新載入/u);
});

test("案件頁移到其他案件：要加入的案件版本過時（伺服器說 conflictTarget 是 target_case）時說要移入的案件剛被更新", async () => {
	const { services } = moveConflictServices({ conflictTarget: "target_case" });
	await openPage(services);
	await click("移到其他案件");
	await until(() => maybeById("move-report-category") !== null, "對話框");
	await pick("move-report-category", PolicyCategory.FRAUD);
	await pick("move-report-severity", ViolationSeverity.LOW);
	await click("確認移動");
	await until(
		() => bodyText().includes("要移入的案件剛被更新，請重新選擇。"),
		"要移入的案件的衝突句",
	);
	assert.doesNotMatch(bodyText(), /這筆通報剛被更新|這個案件剛被更新/u);
});

test("導出的措施背景重讀（寫入之後失效更新）時保留上一份、不閃「載入中」", async () => {
	let listCalls = 0;
	let release: () => void = () => {};
	const gate = new Promise<void>((resolve) => {
		release = resolve;
	});
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({ canChangeSeverity: true, etag: "c-7" }),
				}),
				listMeasures: async () => {
					listCalls++;
					if (listCalls > 1) await gate;
					return {
						measures: [
							{
								id: "m-1",
								kind: MeasureKind.SUSPENSION,
								state: MeasureState.ACTIVE,
							},
						],
					};
				},
				changeCaseSeverity: () => ({ case: kase({}) }),
			},
		}),
	);
	await until(
		() =>
			dom.window.document.querySelectorAll('a[href^="/admin/measures/"]')
				.length === 1,
		"措施列出來",
	);
	await click("改嚴重度");
	await pick("change-case-severity-value", ViolationSeverity.CRITICAL);
	await fill("change-case-severity-note", "又有人送醫");
	await click("確認修改");
	await until(() => listCalls > 1, "寫入成功後措施清單背景重讀");
	const section = dom.window.document.querySelector(
		'section[aria-labelledby="case-measures-title"]',
	);
	assert.ok(section);
	assert.doesNotMatch(
		section.textContent ?? "",
		/載入中/u,
		"重讀期間不閃載入中",
	);
	assert.ok(
		section.querySelector('a[href="/admin/measures/m-1"]') !== null,
		"重讀期間仍顯示上一份",
	);
	release();
});

test("內部註記：改嚴重度寫的註記在標頭列出改前與改後；新增註記掛在案件", async () => {
	const created: Req[] = [];
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({}) }),
				listCaseNotes: () => ({
					notes: [
						create(GovernanceNoteSchema, {
							id: "n1",
							body: "又有人送醫",
							createdByAdminId: "9001",
							createTime: ts(T0),
							severityChange: create(SeverityChangeSchema, {
								fromSeverity: ViolationSeverity.HIGH,
								toSeverity: ViolationSeverity.CRITICAL,
							}),
						}),
						create(GovernanceNoteSchema, {
							id: "n2",
							body: "已電話聯繫賣家",
							createdByAdminId: "9001",
							createTime: ts(T0),
						}),
					],
				}),
				createCaseNote: (req) => {
					created.push(req);
					return { note: { id: "n3" } };
				},
			},
		}),
	);
	await until(() => bodyText().includes("改嚴重度：高 → 嚴重"), "註記列出來");
	assert.match(bodyText(), /又有人送醫/u);
	assert.equal(
		bodyText().match(/改嚴重度：/gu)?.length,
		1,
		"沒有改嚴重度的註記沒有標頭",
	);
	await fill("case-note-body", " 已寄通知 ");
	await click("新增註記");
	await until(() => created.length === 1, "新增註記");
	assert.equal(created[0].caseId, CASE_ID);
	assert.equal((created[0].note as { body: string }).body, "已寄通知");
});

test("結案：開啟時重讀案件，用重讀到的版本送出，成功後關閉並重讀", async () => {
	let reads = 0;
	const decided: Req[] = [];
	await openPage(
		transportWith({
			gov: {
				getCase: () => {
					reads++;
					return {
						case: kase({
							...ALL_CAN,
							etag: `c-${reads}`,
							unresolvedReportCount: 1,
							decisionOptions: [
								{
									decision: CaseDecision.ACTION_TAKEN,
									defaultOutcome: ReportOutcome.ACTION_TAKEN,
								},
							],
						}),
					};
				},
				decideCase: (req) => {
					decided.push(req);
					return { case: kase({}) };
				},
			},
			seller: { getStore: () => TERMINATION_STORE },
		}),
	);
	const readsBefore = reads;
	await click("結案");
	await until(() => maybeById("decide-case-decision") !== null, "重讀完成");
	assert.ok(reads > readsBefore, "開啟時重讀案件");
	await pick("decide-case-decision", CaseDecision.ACTION_TAKEN);
	await click("確認結案");
	await until(() => maybeById("decide-case-decision") === null, "對話框關閉");
	assert.equal(decided.length, 1);
	assert.equal(
		decided[0].etag,
		`c-${readsBefore + 1}`,
		"帶的是開啟對話框時重讀到的 etag",
	);
	assert.equal(decided[0].decision, CaseDecision.ACTION_TAKEN);
});

test("結案遇到版本衝突：重讀案件、對話框留著並顯示案件的衝突句", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({
						...ALL_CAN,
						decisionOptions: [
							{
								decision: CaseDecision.NO_ACTION,
								defaultOutcome: ReportOutcome.NO_VIOLATION,
							},
						],
					}),
				}),
				decideCase: () => {
					throw versionConflict();
				},
			},
			seller: { getStore: () => TERMINATION_STORE },
		}),
	);
	await click("結案");
	await until(() => maybeById("decide-case-decision") !== null, "重讀完成");
	await pick("decide-case-decision", CaseDecision.NO_ACTION);
	await click("確認結案");
	await until(
		() =>
			bodyText().includes(
				"這個案件剛被更新，頁面已重新載入；請確認最新狀態後再操作。",
			),
		"案件的衝突句",
	);
	assert.ok(maybeById("decide-case-decision"), "對話框留在原地");
});

test("改嚴重度：開對話框送出，請求帶案件的 etag", async () => {
	const changed: Req[] = [];
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({
						canChangeSeverity: true,
						severity: ViolationSeverity.HIGH,
						etag: "c-7",
					}),
				}),
				changeCaseSeverity: (req) => {
					changed.push(req);
					return { case: kase({}) };
				},
			},
		}),
	);
	await click("改嚴重度");
	await pick("change-case-severity-value", ViolationSeverity.CRITICAL);
	await fill("change-case-severity-note", "又有人送醫");
	await click("確認修改");
	await until(
		() => maybeById("change-case-severity-value") === null,
		"對話框關閉",
	);
	assert.equal(changed[0].etag, "c-7");
	assert.equal(changed[0].severity, ViolationSeverity.CRITICAL);
	assert.equal(changed[0].note, "又有人送醫");
});

test("記錄轉報：成功時沒有提示；伺服器說之前已經記過時顯示橫幅，說明這次輸入沒有寫入", async () => {
	for (const alreadyRecorded of [false, true]) {
		await openPage(
			transportWith({
				gov: {
					getCase: () => ({ case: kase({ canRecordFdaReferral: true }) }),
					recordFDAReferral: () => ({ case: kase({}), alreadyRecorded }),
				},
			}),
		);
		await click("記錄已轉報主管機關");
		await pick(
			"record-fda-referral-channel",
			FDAChannel.FDA_CHANNEL_HOTLINE_1919,
		);
		const submit = [
			...dom.window.document.querySelectorAll(".modal-footer button"),
		].find((b) => b.textContent === "記錄轉報");
		assert.ok(submit);
		await React.act(async () => {
			(submit as HTMLElement).click();
		});
		await until(
			() => maybeById("record-fda-referral-channel") === null,
			"對話框關閉",
		);
		const banner =
			"這個案件先前已記錄轉報，以下是原本的紀錄，這次輸入的內容沒有寫入。";
		assert.equal(bodyText().includes(banner), alreadyRecorded);
	}
});

test("下載轉報資料：讀 GetFDAExport 在 client 組成 .md 下載；失敗顯示原因", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({ canExportFda: true }) }),
				getFDAExport: (req) => {
					assert.equal(req.caseId, CASE_ID);
					return {
						case: kase({ openTime: ts(T0) }),
						reports: [report({ id: "r-1", description: "吃了便當之後腹瀉" })],
						store: { id: STORE_ID, storeName: "阿里山農場" },
					};
				},
			},
		}),
	);
	await click("下載轉報資料（.md）");
	await until(() => downloads.length === 1, "下載");
	assert.equal(downloads[0].fileName, `fda-referral-case-${CASE_ID}.md`);
	const md = await downloads[0].blob.text();
	assert.match(md, new RegExp(`^# 食安通報轉報資料（案件 ${CASE_ID}）`, "u"));
	assert.match(md, /吃了便當之後腹瀉/u);
	assert.match(
		md,
		/本檔由後台於 \d{4}\/\d{2}\/\d{2} \d{2}:\d{2} 產生，供轉報主管機關使用。/u,
	);

	await openPage(
		failingFirstOn(
			transportWith({
				gov: {
					getCase: () => ({ case: kase({ canExportFda: true }) }),
					getFDAExport: () => ({ case: kase({}), reports: [] }),
				},
			}),
			"GetFDAExport",
			1,
		),
	);
	await click("下載轉報資料（.md）");
	await until(
		() => bodyText().includes("轉報資料讀取失敗，請再試一次。"),
		"網路層失敗的句子",
	);
});

// ---------------------------------------------------------------------------
// 從案件建立措施

function citableReports(count: number, prefix = "r") {
	return Array.from({ length: count }, (_, i) =>
		report({
			id: `${prefix}${String(i).padStart(2, "0")}`,
			state: ReportState.IN_CASE,
			caseId: CASE_ID,
			canCiteAsSource: true,
			createTime: ts(new Date(T0.getTime() + i * 1000)),
		}),
	);
}

/** 伺服器的 ListReports：citable_as_source 只留此刻可以當來源的通報（Report.can_cite_as_source），並照 page_size 截。 */
function serverListReports(reports: ReturnType<typeof report>[]) {
	return (req: { citableAsSource?: boolean; pageSize?: number }) => ({
		reports: (req.citableAsSource
			? reports.filter((r) => r.canCiteAsSource)
			: reports
		).slice(0, req.pageSize),
	});
}

test("建立措施：頂端說明記為案件導出的措施；類別預填案件的類別；來源通報預先勾選最早的 20 筆並說明；請求帶 source_case_id", async () => {
	const created: Req[] = [];
	const reports = citableReports(25);
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({
					case: kase({ ...ALL_CAN, category: PolicyCategory.FRAUD }),
				}),
				listReports: serverListReports(reports),
				createMeasure: (req) => {
					created.push(req);
					return { measure: { id: "m-new" } };
				},
			},
		}),
	);
	await until(
		() => buttonByText("建立措施")?.disabled === false,
		"來源通報讀完",
	);
	await click("建立措施");
	assert.match(
		bodyText(),
		new RegExp(`這個措施會記為案件 ${CASE_ID} 導出的措施。`, "u"),
	);
	assert.equal(
		byId<HTMLSelectElement>("create-violation-measure-category").value,
		String(PolicyCategory.FRAUD),
	);
	assert.match(
		bodyText(),
		/這個案件超過 20 筆可以當來源的通報，已預先勾選最早的 20 筆。/u,
	);
	await until(sourcesLoaded, "來源通報清單讀完");
	assert.equal(sourceBoxes().filter((box) => box.checked).length, 20);
	await pick("create-violation-measure-kind", "warning");
	await pick("create-violation-measure-severity", ViolationSeverity.HIGH);
	await fill("create-violation-measure-statement", "標示不實");
	await fill("create-violation-measure-note", "三筆通報屬實");
	await click("確認建立");
	await until(() => created.length === 1, "送出");
	const req = created[0] as {
		sourceCaseId: string;
		measure: {
			case: string;
			value: {
				violation: { finding: { category: number; sourceReportIds: string[] } };
			};
		};
	};
	assert.equal(req.sourceCaseId, CASE_ID);
	assert.equal(req.measure.case, "warning");
	assert.equal(
		req.measure.value.violation.finding.category,
		PolicyCategory.FRAUD,
	);
	assert.deepEqual(
		req.measure.value.violation.finding.sourceReportIds,
		reports
			.slice(0, 20)
			.map((r) => r.id)
			.sort(),
		"最早的 20 筆（依建立時間舊到新）",
	);
	await until(() => bodyText().includes("已建立措施。"), "成功提示");
	assert.ok(maybeById("create-violation-measure-kind") === null);
});

test("建立措施：可以當來源的通報不超過 20 筆時不顯示超過的說明；不能當來源的不勾", async () => {
	const reports = [
		...citableReports(3),
		report({
			id: "r-blocked",
			state: ReportState.IN_CASE,
			caseId: CASE_ID,
			canCiteAsSource: false,
		}),
	];
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase(ALL_CAN) }),
				listReports: serverListReports(reports),
			},
		}),
	);
	await until(
		() => buttonByText("建立措施")?.disabled === false,
		"來源通報讀完",
	);
	await click("建立措施");
	await until(sourcesLoaded, "來源通報清單讀完");
	assert.doesNotMatch(bodyText(), /超過 20 筆/u);
	assert.equal(sourceBoxes().length, 3, "不能當來源的通報不列");
	assert.equal(sourceBoxes().filter((box) => box.checked).length, 3);
});

test("預先勾選的來源通報用伺服器的篩選：案件的通報前 100 筆都不能當來源也不翻頁，只多讀一次 citable_as_source（21 筆）", async () => {
	const blocked = Array.from({ length: 100 }, (_, i) =>
		report({
			id: `blocked-${i}`,
			state: ReportState.IN_CASE,
			caseId: CASE_ID,
			canCiteAsSource: false,
		}),
	);
	const all = [...blocked, ...citableReports(5, "late")];
	const requests: {
		caseId: string;
		citableAsSource: boolean;
		pageSize: number;
		pageToken: string;
		storeId: string;
	}[] = [];
	const created: Req[] = [];
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({ ...ALL_CAN, reportCount: 105 }) }),
				listReports: (req) => {
					if (req.storeId) return { reports: [] };
					requests.push(req as (typeof requests)[number]);
					if (req.citableAsSource) return serverListReports(all)(req);
					return { reports: blocked, nextPageToken: "p2" };
				},
				createMeasure: (req) => {
					created.push(req);
					return { measure: { id: "m-new" } };
				},
			},
		}),
	);
	await until(
		() => buttonByText("建立措施")?.disabled === false,
		"來源通報讀完",
	);
	const citable = requests.filter((r) => r.citableAsSource);
	assert.equal(citable.length, 1, "篩選的那一次只讀一頁");
	assert.equal(citable[0].caseId, CASE_ID);
	assert.equal(citable[0].pageSize, 21, "多讀一筆就知道有沒有超過 20 筆");
	assert.equal(citable[0].pageToken, "");
	assert.deepEqual(
		requests.filter((r) => !r.citableAsSource).map((r) => r.pageToken),
		[""],
		"案件的通報列表只讀第一頁，不為了找來源翻頁",
	);
	await click("建立措施");
	await until(sourcesLoaded, "對話框的清單讀完");
	assert.doesNotMatch(bodyText(), /超過 20 筆/u);
	await pick("create-violation-measure-kind", "warning");
	await pick("create-violation-measure-severity", ViolationSeverity.LOW);
	await fill("create-violation-measure-statement", "x");
	await fill("create-violation-measure-note", "y");
	await click("確認建立");
	await until(() => created.length === 1, "送出");
	assert.deepEqual(
		(
			created[0] as {
				measure: {
					value: { violation: { finding: { sourceReportIds: string[] } } };
				};
			}
		).measure.value.violation.finding.sourceReportIds,
		citableReports(5, "late").map((r) => r.id),
	);
});

test("案件的通報超過一頁時有上一頁與下一頁；下一頁讀下一頁，已讀過的頁不重讀", async () => {
	const pages: Record<
		string,
		{ reports: ReturnType<typeof report>[]; nextPageToken: string }
	> = {
		"": { reports: citableReports(100, "a"), nextPageToken: "p2" },
		p2: { reports: citableReports(3, "b"), nextPageToken: "" },
	};
	const tokens: string[] = [];
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase({ reportCount: 103 }) }),
				listReports: (req) => {
					tokens.push(req.pageToken as string);
					return pages[req.pageToken as string];
				},
			},
		}),
	);
	const itemCount = () =>
		dom.window.document.querySelectorAll(
			'section[aria-labelledby="case-reports-title"] li',
		).length;
	assert.equal(itemCount(), 100);
	assert.match(bodyText(), /第 1 頁/u);
	await click("下一頁");
	await until(() => itemCount() === 3, "第二頁");
	assert.match(bodyText(), /第 2 頁/u);
	assert.ok(buttonByText("下一頁")?.disabled);
	await click("上一頁");
	await until(() => itemCount() === 100, "回到第一頁");
	assert.deepEqual(tokens, ["", "p2"], "回到第一頁不重讀");
});

test("建立風險措施：頂端說明記為案件導出的措施，請求帶 source_case_id", async () => {
	const created: Req[] = [];
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase(ALL_CAN) }),
				createMeasure: (req) => {
					created.push(req);
					return { measure: { id: "m-risk" } };
				},
			},
		}),
	);
	await click("建立風險措施");
	assert.match(
		bodyText(),
		new RegExp(`這個措施會記為案件 ${CASE_ID} 導出的措施。`, "u"),
	);
	await pick("create-measure-ground", RiskGround.FOOD_SAFETY);
	await fill("create-measure-statement", "疑似食物中毒，先行停權");
	await fill("create-measure-note", "三人就醫");
	await click("確認建立");
	await until(() => created.length === 1, "送出");
	assert.equal(created[0].sourceCaseId, CASE_ID);
	assert.equal((created[0].measure as { case: string }).case, "suspension");
	await until(
		() => bodyText().includes("已建立風險措施，賣家已停權。"),
		"成功提示",
	);
});

test("終止合作：頂端說明記為案件導出的措施，事由由店家讀到的選項決定，請求帶 source_case_id", async () => {
	const created: Req[] = [];
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase(ALL_CAN) }),
				createMeasure: (req) => {
					created.push(req);
					return { measure: { id: "m-term" } };
				},
			},
			seller: { getStore: () => TERMINATION_STORE },
		}),
	);
	await until(() => buttonByText("終止合作") !== undefined, "店家讀完");
	await click("終止合作");
	assert.match(
		bodyText(),
		new RegExp(`這個措施會記為案件 ${CASE_ID} 導出的措施。`, "u"),
	);
	await pick("create-termination-ground", TerminationGround.NOTICE);
	await fill("create-termination-statement", "屢次違規");
	await fill("create-termination-note", "經主管核准");
	await click("確認建立");
	await until(() => created.length === 1, "送出");
	assert.equal(created[0].sourceCaseId, CASE_ID);
	assert.equal((created[0].measure as { case: string }).case, "termination");
	assert.equal(
		(created[0].measure as { value: { ground: number } }).value.ground,
		TerminationGround.NOTICE,
	);
	await until(() => bodyText().includes("已建立終止合作"), "成功提示");
});

test("建立措施的送出失敗：留在原地顯示原因，不關閉", async () => {
	await openPage(
		transportWith({
			gov: {
				getCase: () => ({ case: kase(ALL_CAN) }),
				createMeasure: () => {
					throw withMessage(
						Code.FailedPrecondition,
						"案件已結案，不能再從它建立措施。",
					);
				},
			},
		}),
	);
	await click("建立風險措施");
	await pick("create-measure-ground", RiskGround.FOOD_SAFETY);
	await fill("create-measure-statement", "x");
	await fill("create-measure-note", "y");
	await click("確認建立");
	await until(
		() => bodyText().includes("案件已結案，不能再從它建立措施。"),
		"伺服器的句子",
	);
	assert.ok(maybeById("create-measure-ground"));
});

test("返回鈕回到清單", async () => {
	await openPage(
		transportWith({ gov: { getCase: () => ({ case: kase({}) }) } }),
	);
	const back = dom.window.document.querySelector(
		'button[aria-label="返回列表"]',
	);
	assert.ok(back);
	await React.act(async () => {
		(back as HTMLElement).click();
	});
	assert.deepEqual(navigation.pushed, ["/admin/violation-reports"]);
});
