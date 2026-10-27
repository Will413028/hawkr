import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test, { afterEach } from "node:test";
import {
	type Timestamp,
	timestampDate,
	timestampFromDate,
} from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	MeasureKind,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import { QueryClient } from "@tanstack/react-query";
import { JSDOM } from "jsdom";
import { measureInitialFromSuggestion } from "../src/app/admin/merchants/[id]/violation-measure";

/**
 * 建立措施對話框（CreateMeasureDialog，違規依據的警告與停權）實際掛進 DOM 後的行為，用 router transport 回真的 protobuf 訊息：
 * 來源通報只列這家店、伺服器說可以當來源的（canCiteAsSource）；從通報詳情開啟時預先勾選；沒有第一則註記不送出；停權必填結束時間；
 * 送出的請求形狀；request_id 重送沿用、改內容換新、重開是新的。
 */

const dom = new JSDOM("<!doctype html><body></body>", {
	url: "http://localhost/",
	pretendToBeVisual: true,
});
const g = globalThis as Record<string, unknown>;
g.window = dom.window;
g.self = dom.window;
g.document = dom.window.document;
g.HTMLElement = dom.window.HTMLElement;
g.Node = dom.window.Node;
g.IS_REACT_ACT_ENVIRONMENT = true;

const requireFromTest = createRequire(__filename);
requireFromTest.extensions[".css"] = () => {};

const React = requireFromTest("react") as {
	createElement: (
		type: unknown,
		props?: unknown,
		...children: unknown[]
	) => unknown;
	act: (fn: () => void | Promise<void>) => Promise<void>;
};
const { createRoot } = requireFromTest("react-dom/client") as {
	createRoot: (el: Element) => {
		render: (node: unknown) => void;
		unmount: () => void;
	};
};
const { QueryClientProvider } = requireFromTest("@tanstack/react-query") as {
	QueryClientProvider: unknown;
};
const { TransportProvider } = requireFromTest("@connectrpc/connect-query") as {
	TransportProvider: unknown;
};
const Dialog = requireFromTest(
	"../src/app/admin/merchants/[id]/CreateMeasureDialog",
).default as unknown;

type Request = {
	requestId?: string;
	storeId?: string;
	sourceCaseId?: string;
	statement?: string;
	note?: string;
	measure?: {
		case?: string;
		value?: {
			violation?: Violation;
			basis?: { case?: string; value?: Violation };
			endTime?: unknown;
		};
	};
};
type Violation = {
	finding?: {
		category?: number;
		severity?: number;
		sourceReportIds?: string[];
	};
	citedFindingIds?: string[];
};

const STORE_ID = "100";

/**
 * 一筆通報。client 只看伺服器推導的 `canCiteAsSource`（能不能當認定的來源），不看結果；fixture 帶結果只是讓資料像真的，
 * 並且能造出「還沒有結果、但不能當來源」的通報（例如店家已關閉）來證明 client 不看結果。
 */
function report(
	id: string,
	canCiteAsSource: boolean,
	outcome = ReportOutcome.UNSPECIFIED,
	storeId = STORE_ID,
) {
	return {
		id,
		storeId,
		category: PolicyCategory.FRAUD,
		outcome,
		canCiteAsSource,
		createTime: timestampFromDate(new Date("2025-10-09T08:53:20Z")),
	};
}

/** 還沒有結果，可以當來源。 */
const NO_OUTCOME_ID = "0190a000-0000-7000-8000-000000000001";
/** 已採取措施，可以當來源。 */
const ACTION_TAKEN_ID = "0190a000-0000-7000-8000-000000000002";
/** 未違規，不能當來源。 */
const NO_VIOLATION_ID = "0190a000-0000-7000-8000-000000000003";
/** 還沒有結果、但 canCiteAsSource = false（店家已關閉）：client 不看結果，所以也不列。 */
const BLOCKED_ID = "0190a000-0000-7000-8000-000000000005";
const FINDING_ID = "0190a000-0000-7000-8000-0000000000f1";

function setValue(el: Element, value: string) {
	const proto =
		el instanceof dom.window.HTMLTextAreaElement
			? dom.window.HTMLTextAreaElement.prototype
			: el instanceof dom.window.HTMLSelectElement
				? dom.window.HTMLSelectElement.prototype
				: dom.window.HTMLInputElement.prototype;
	Object.getOwnPropertyDescriptor(proto, "value")?.set?.call(el, value);
	const type = el instanceof dom.window.HTMLSelectElement ? "change" : "input";
	el.dispatchEvent(new dom.window.Event(type, { bubbles: true }));
}

function byId(id: string): Element {
	const el = dom.window.document.getElementById(id);
	assert.ok(el, `找不到 #${id}`);
	return el;
}

function buttonByText(text: string): HTMLButtonElement | undefined {
	return [...dom.window.document.querySelectorAll("button")].find(
		(b) => b.textContent === text,
	) as HTMLButtonElement | undefined;
}

async function click(text: string) {
	const button = buttonByText(text);
	assert.ok(button, `找不到按鈕「${text}」`);
	await React.act(async () => {
		button.click();
	});
}

const bodyText = () => dom.window.document.body.textContent ?? "";

function checkbox(labelPart: string): HTMLInputElement {
	const label = [...dom.window.document.querySelectorAll("label")].find((l) =>
		l.textContent?.includes(labelPart),
	);
	const input = label?.querySelector("input");
	assert.ok(input, `找不到勾選項「${labelPart}」`);
	return input as HTMLInputElement;
}

async function toggle(labelPart: string) {
	const input = checkbox(labelPart);
	await React.act(async () => {
		input.click();
	});
}

async function settle(m: { queryClient: QueryClient }) {
	const done = () => {
		const queries = m.queryClient.getQueryCache().getAll();
		return (
			queries.length > 0 &&
			m.queryClient.isFetching() === 0 &&
			// 停用的查詢（帳號還不知道時的 skipToken）會一直停在 pending，不算還沒完成。
			queries.every(
				(q) => q.state.status !== "pending" || q.state.fetchStatus === "idle",
			)
		);
	};
	const deadline = Date.now() + 5000;
	while (!done()) {
		assert.ok(Date.now() < deadline, "查詢在 5 秒內沒有完成");
		await React.act(async () => {
			await new Promise((resolve) => setTimeout(resolve, 1));
		});
	}
	await React.act(async () => {
		await new Promise((resolve) => setTimeout(resolve, 0));
	});
}

/** 反覆讓 React 與查詢跑一輪，直到條件成立（最多 5 秒）。 */
async function until(done: () => boolean, what: string) {
	const deadline = Date.now() + 5000;
	while (!done()) {
		assert.ok(Date.now() < deadline, `5 秒內沒有等到：${what}`);
		await React.act(async () => {
			await new Promise((resolve) => setTimeout(resolve, 1));
		});
	}
}

interface Seen {
	listReports: Array<{ storeId: string }>;
	/** 每次 ListReports 帶的 page_token（第一頁是空字串）。 */
	listReportTokens: string[];
	listViolations: Array<{ accountId: string; status: number }>;
	/** GetStore 被呼叫的次數。 */
	getStore: number;
}

/** 一頁通報；nextPageToken 沒給就是最後一頁。 */
interface ReportPage {
	reports: ReturnType<typeof report>[];
	nextPageToken?: string;
}

interface Scenario {
	/** 每次 GetStore 的結果：ok 回店家、empty 回沒有店家、fail 回 unavailable、hang 一直不回應；用完之後都是 ok。 */
	store?: Array<"ok" | "empty" | "fail" | "hang">;
	/** 依 page_token 回傳的通報頁；沒給是預設的四筆通報、沒有下一頁。 */
	reportPages?: Record<string, ReportPage>;
}

const DEFAULT_REPORTS: ReportPage = {
	reports: [
		report(NO_OUTCOME_ID, true),
		report(ACTION_TAKEN_ID, true, ReportOutcome.ACTION_TAKEN),
		report(NO_VIOLATION_ID, false, ReportOutcome.NO_VIOLATION),
		report(BLOCKED_ID, false),
	],
};

function transportWith(seen: Seen, scenario: Scenario = {}) {
	const storeOutcomes = [...(scenario.store ?? [])];
	return createRouterTransport(({ service }) => {
		service(SellerAdminService, {
			getStore: (() => {
				seen.getStore++;
				const outcome = storeOutcomes.shift() ?? "ok";
				if (outcome === "fail") {
					throw new ConnectError("store down", Code.Unavailable);
				}
				if (outcome === "hang") return new Promise(() => {});
				if (outcome === "empty") return {};
				return { store: { id: STORE_ID, accountId: "acc-1" } };
			}) as never,
		});
		service(SellerGovernanceAdminService, {
			listReports: ((req: { storeId: string; pageToken: string }) => {
				seen.listReports.push({ storeId: req.storeId });
				seen.listReportTokens.push(req.pageToken);
				return scenario.reportPages?.[req.pageToken] ?? DEFAULT_REPORTS;
			}) as never,
			listViolations: ((req: { accountId: string; status: number }) => {
				seen.listViolations.push({
					accountId: req.accountId,
					status: req.status,
				});
				return {
					violations: [
						{
							id: FINDING_ID,
							accountId: "acc-1",
							createTime: timestampFromDate(new Date("2025-10-09T08:53:20Z")),
						},
					],
				};
			}) as never,
		});
	});
}

const openRoots = new Set<() => Promise<void>>();
afterEach(async () => {
	for (const unmount of [...openRoots]) await unmount();
	openRoots.clear();
});

function openDialog(
	outcomes: Array<"fail" | "ok">,
	props: Record<string, unknown> = {},
	scenario: Scenario = {},
) {
	const requests: Request[] = [];
	const seen: Seen = {
		listReports: [],
		listReportTokens: [],
		listViolations: [],
		getStore: 0,
	};
	const onConfirm = async (request: Request) => {
		requests.push(request);
		if ((outcomes.shift() ?? "ok") === "fail") {
			throw new ConnectError("unavailable", Code.Unavailable);
		}
	};
	const transport = transportWith(seen, scenario);
	const host = dom.window.document.createElement("div");
	dom.window.document.body.appendChild(host);
	const root = createRoot(host);
	const queryClient = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	let mounted = true;
	const unmount = () =>
		React.act(async () => {
			if (!mounted) return;
			mounted = false;
			root.unmount();
			host.remove();
		});
	openRoots.add(unmount);
	return {
		requests,
		seen,
		queryClient,
		unmount,
		/** settle 為 false 時不等查詢完成（有查詢一直不回應的情境）。 */
		mount: async ({ settle: wait = true }: { settle?: boolean } = {}) => {
			await React.act(async () => {
				root.render(
					React.createElement(
						TransportProvider,
						{ transport },
						React.createElement(
							QueryClientProvider,
							{ client: queryClient },
							React.createElement(Dialog, {
								storeId: STORE_ID,
								onClose: () => {},
								onConfirm,
								...props,
							}),
						),
					),
				);
			});
			if (wait) await settle({ queryClient });
		},
	};
}

async function fillCommon(
	kind: "warning" | "suspension",
	note = "客服轉來兩筆檢舉",
) {
	await React.act(async () => {
		setValue(byId("create-violation-measure-kind"), kind);
		setValue(
			byId("create-violation-measure-category"),
			String(PolicyCategory.FRAUD),
		);
		setValue(
			byId("create-violation-measure-severity"),
			String(ViolationSeverity.HIGH),
		);
		setValue(byId("create-violation-measure-statement"), "商品頁宣稱有機");
		if (note !== "") {
			setValue(byId("create-violation-measure-note"), note);
		}
	});
}

test("來源通報只列這家店、伺服器說可以當來源的（canCiteAsSource，client 不看結果）；引用的認定列帳號生效中的", async () => {
	const d = openDialog(["ok"]);
	await d.mount();
	assert.deepEqual(d.seen.listReports, [{ storeId: STORE_ID }]);
	assert.ok(checkbox(NO_OUTCOME_ID));
	assert.ok(checkbox(ACTION_TAKEN_ID));
	assert.doesNotMatch(bodyText(), new RegExp(NO_VIOLATION_ID, "u"));
	assert.doesNotMatch(
		bodyText(),
		new RegExp(BLOCKED_ID, "u"),
		"還沒有結果但 canCiteAsSource=false 的通報不列：client 不自己看結果",
	);
	assert.equal(d.seen.listViolations[0]?.accountId, "acc-1");
	assert.ok(checkbox(FINDING_ID));
	await d.unmount();
});

test("警告：多選來源通報加引用的認定，請求是 warning.violation 並帶完整新認定", async () => {
	const d = openDialog(["ok"]);
	await d.mount();
	await fillCommon("warning");
	await toggle(NO_OUTCOME_ID);
	await toggle(ACTION_TAKEN_ID);
	await toggle(FINDING_ID);
	await click("確認建立");
	assert.equal(d.requests.length, 1);
	const req = d.requests[0];
	assert.ok(req?.requestId);
	assert.equal(req.storeId, STORE_ID);
	assert.equal(req.statement, "商品頁宣稱有機");
	assert.equal(req.note, "客服轉來兩筆檢舉");
	assert.equal(req.measure?.case, "warning");
	assert.deepEqual(req.measure?.value?.violation?.finding?.sourceReportIds, [
		NO_OUTCOME_ID,
		ACTION_TAKEN_ID,
	]);
	assert.equal(
		req.measure?.value?.violation?.finding?.category,
		PolicyCategory.FRAUD,
	);
	assert.equal(
		req.measure?.value?.violation?.finding?.severity,
		ViolationSeverity.HIGH,
	);
	assert.deepEqual(req.measure?.value?.violation?.citedFindingIds, [
		FINDING_ID,
	]);
	await d.unmount();
});

test("從通報詳情開啟：預先勾選那一筆，送出的請求帶它", async () => {
	const d = openDialog(["ok"], {
		initial: { sourceReportIds: [ACTION_TAKEN_ID] },
	});
	await d.mount();
	assert.equal(checkbox(ACTION_TAKEN_ID).checked, true);
	assert.equal(checkbox(NO_OUTCOME_ID).checked, false);
	await fillCommon("warning");
	await click("確認建立");
	assert.deepEqual(
		d.requests[0]?.measure?.value?.violation?.finding?.sourceReportIds,
		[ACTION_TAKEN_ID],
	);
	await d.unmount();
});

test("沒有第一則內部註記不送出，就地說明", async () => {
	const d = openDialog(["ok"]);
	await d.mount();
	await fillCommon("warning", "");
	await click("確認建立");
	assert.equal(d.requests.length, 0, "沒有註記不得送出");
	assert.match(bodyText(), /請填寫內部註記/u);
	await d.unmount();
});

test("沒有給賣家的理由、沒選種類、類別或嚴重度都不送出", async () => {
	const d = openDialog(["ok"]);
	await d.mount();
	await click("確認建立");
	assert.match(bodyText(), /請選擇種類/u);
	await React.act(async () => {
		setValue(byId("create-violation-measure-kind"), "warning");
	});
	await click("確認建立");
	assert.match(bodyText(), /請選擇類別/u);
	await React.act(async () => {
		setValue(
			byId("create-violation-measure-category"),
			String(PolicyCategory.FRAUD),
		);
	});
	await click("確認建立");
	assert.match(bodyText(), /請選擇嚴重度/u);
	await React.act(async () => {
		setValue(
			byId("create-violation-measure-severity"),
			String(ViolationSeverity.LOW),
		);
		setValue(byId("create-violation-measure-note"), "x");
	});
	await click("確認建立");
	assert.match(bodyText(), /請填寫給賣家的理由/u);
	assert.equal(d.requests.length, 0);
	await d.unmount();
});

test("類別下拉只列平台判定與認定可用的 12 個政策分類：沒有「其他」，也沒有舊的名稱", async () => {
	const d = openDialog(["ok"]);
	await d.mount();
	const options = [
		...byId("create-violation-measure-category").querySelectorAll("option"),
	].map((option) => option.textContent);
	assert.deepEqual(options, [
		"請選擇",
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
	]);
	await d.unmount();
});

test("停權：結束時間必填且晚於現在，請求是 suspension.basis.violation 帶結束時間；警告沒有結束時間欄", async () => {
	const d = openDialog(["ok"]);
	await d.mount();
	await fillCommon("warning");
	assert.equal(
		dom.window.document.getElementById("create-violation-measure-end-time"),
		null,
		"警告沒有結束時間",
	);
	await fillCommon("suspension");
	await click("確認建立");
	assert.match(bodyText(), /請填寫結束時間/u);
	await React.act(async () => {
		setValue(byId("create-violation-measure-end-time"), "2020-01-01T00:00");
	});
	await click("確認建立");
	assert.match(bodyText(), /結束時間必須晚於現在/u);
	assert.equal(d.requests.length, 0);
	await React.act(async () => {
		setValue(byId("create-violation-measure-end-time"), "2099-01-01T00:00");
	});
	await click("確認建立");
	assert.equal(d.requests.length, 1);
	const req = d.requests[0];
	assert.equal(req?.measure?.case, "suspension");
	assert.equal(req?.measure?.value?.basis?.case, "violation");
	assert.ok(req?.measure?.value?.endTime);
	assert.equal(
		req?.measure?.value?.basis?.value?.finding?.category,
		PolicyCategory.FRAUD,
	);
	await d.unmount();
});

test("從發票的停權建議開啟：種類、結束時間與理由已預填、類別沒有預填；預填的值可以改，送出的是改過的值", async () => {
	const initial = measureInitialFromSuggestion(
		{
			suggestedSuspendDays: "14",
			suggestedReason: "90 天內第 3 件發票違規，建議停權 14 天",
		},
		new Date(),
	);
	const d = openDialog(["ok"], { initial });
	await d.mount();
	assert.equal(
		(byId("create-violation-measure-kind") as HTMLSelectElement).value,
		"suspension",
	);
	assert.equal(
		(byId("create-violation-measure-end-time") as HTMLInputElement).value,
		initial.measure?.case === "suspension"
			? initial.measure.endTime
			: undefined,
	);
	assert.equal(
		(byId("create-violation-measure-statement") as HTMLTextAreaElement).value,
		initial.statement,
	);
	// 類別與嚴重度不預填：只補上其餘必填欄位仍送不出去。
	assert.equal(
		(byId("create-violation-measure-category") as HTMLSelectElement).value,
		String(PolicyCategory.UNSPECIFIED),
	);
	await React.act(async () => {
		setValue(
			byId("create-violation-measure-severity"),
			String(ViolationSeverity.HIGH),
		);
		setValue(byId("create-violation-measure-note"), "發票申訴判定違規");
	});
	await click("確認建立");
	assert.equal(d.requests.length, 0, "沒選類別不得送出");
	assert.match(bodyText(), /請選擇類別/u);
	// 預填的每一欄都可以改，送出的是改過的值。
	await React.act(async () => {
		setValue(
			byId("create-violation-measure-category"),
			String(PolicyCategory.FRAUD),
		);
		setValue(byId("create-violation-measure-statement"), "改過的理由");
		setValue(byId("create-violation-measure-end-time"), "2099-01-01T00:00");
	});
	await click("確認建立");
	assert.equal(d.requests.length, 1);
	const req = d.requests[0];
	assert.equal(req?.statement, "改過的理由");
	assert.equal(req?.measure?.case, "suspension");
	assert.equal(
		req?.measure?.value?.basis?.value?.finding?.category,
		PolicyCategory.FRAUD,
	);
	const end = req?.measure?.value?.endTime;
	assert.ok(end);
	assert.equal(
		timestampDate(end as Timestamp).getTime(),
		Date.parse("2099-01-01T00:00:00+08:00"),
		"結束時間是改過之後的台北時間",
	);
	await d.unmount();
});

test("沒有預填時沒有任何欄位被預填（常駐的「建立措施」）", async () => {
	const d = openDialog(["ok"]);
	await d.mount();
	assert.equal(
		(byId("create-violation-measure-kind") as HTMLSelectElement).value,
		"",
	);
	assert.equal(
		(byId("create-violation-measure-statement") as HTMLTextAreaElement).value,
		"",
	);
	assert.equal(
		(byId("create-violation-measure-category") as HTMLSelectElement).value,
		String(PolicyCategory.UNSPECIFIED),
	);
	await d.unmount();
});

test("預填的內容也是 request_id 的一部分：失敗後原樣重送沿用同一個 request_id，改過預填的理由換新的", async () => {
	const initial = measureInitialFromSuggestion(
		{ suggestedSuspendDays: "7", suggestedReason: "建議理由" },
		new Date(),
	);
	const d = openDialog(["fail", "fail", "ok"], { initial });
	await d.mount();
	await React.act(async () => {
		setValue(
			byId("create-violation-measure-category"),
			String(PolicyCategory.FRAUD),
		);
		setValue(
			byId("create-violation-measure-severity"),
			String(ViolationSeverity.LOW),
		);
		setValue(byId("create-violation-measure-note"), "註記");
	});
	await click("確認建立");
	await click("確認建立");
	assert.equal(d.requests.length, 2);
	assert.equal(d.requests[1]?.requestId, d.requests[0]?.requestId);
	await React.act(async () => {
		setValue(byId("create-violation-measure-statement"), "建議理由，已修改");
	});
	await click("確認建立");
	assert.equal(d.requests.length, 3);
	assert.notEqual(d.requests[2]?.requestId, d.requests[0]?.requestId);
	await d.unmount();
});

test("同一份內容失敗後重送：request_id 不變；改過內容換新的；重新開啟是新的", async () => {
	const d = openDialog(["fail", "fail", "ok"]);
	await d.mount();
	await fillCommon("warning");
	await click("確認建立");
	await click("確認建立");
	assert.equal(d.requests.length, 2);
	assert.ok(d.requests[0]?.requestId);
	assert.equal(d.requests[1]?.requestId, d.requests[0]?.requestId);
	assert.match(bodyText(), /建立失敗|unavailable|不可用/u);
	await toggle(NO_OUTCOME_ID);
	await click("確認建立");
	assert.notEqual(
		d.requests[2]?.requestId,
		d.requests[0]?.requestId,
		"來源通報改過必須換新的 request_id",
	);
	await d.unmount();

	const again = openDialog(["ok"]);
	await again.mount();
	await fillCommon("warning");
	await click("確認建立");
	assert.notEqual(again.requests[0]?.requestId, d.requests[0]?.requestId);
	await again.unmount();
});

test("來源通報超過 20 筆不送出", async () => {
	const many = Array.from({ length: 21 }, (_, i) => `r-${i}`);
	const d = openDialog(["ok"], { initial: { sourceReportIds: many } });
	await d.mount();
	await fillCommon("warning");
	await click("確認建立");
	assert.equal(d.requests.length, 0);
	assert.match(bodyText(), /來源通報最多 20 筆/u);
	await d.unmount();
});

const OLDER_ID = "0190a000-0000-7000-8000-000000000004";

test("來源通報：已載入的通報全是不能當來源的、但還有下一頁時，仍有「載入更多」，載入後選得到較舊的通報", async () => {
	const d = openDialog(
		["ok"],
		{},
		{
			reportPages: {
				"": {
					reports: [
						report(NO_VIOLATION_ID, false, ReportOutcome.NO_VIOLATION),
						report(BLOCKED_ID, false),
					],
					nextPageToken: "page-2",
				},
				"page-2": { reports: [report(OLDER_ID, true)] },
			},
		},
	);
	await d.mount();
	assert.ok(buttonByText("載入更多"), "還有下一頁就要有「載入更多」");
	assert.doesNotMatch(
		bodyText(),
		/這家店沒有可以當來源的通報/u,
		"還有下一頁時不能說沒有",
	);
	await click("載入更多");
	await until(() => bodyText().includes(OLDER_ID), "第二頁的通報出現");
	assert.deepEqual(d.seen.listReportTokens, ["", "page-2"]);
	await toggle(OLDER_ID);
	assert.equal(checkbox(OLDER_ID).checked, true);
	assert.equal(buttonByText("載入更多"), undefined, "最後一頁之後不再有");
	assert.doesNotMatch(bodyText(), /這家店沒有可以當來源的通報/u);
	await d.unmount();
});

test("來源通報：沒有可選的通報、也沒有下一頁時，才說這家店沒有可以當來源的通報", async () => {
	const d = openDialog(
		["ok"],
		{},
		{
			reportPages: {
				"": {
					reports: [report(NO_VIOLATION_ID, false, ReportOutcome.NO_VIOLATION)],
				},
			},
		},
	);
	await d.mount();
	assert.match(bodyText(), /這家店沒有可以當來源的通報/u);
	assert.equal(buttonByText("載入更多"), undefined);
	await d.unmount();
});

const CITED_FIELD = 'fieldset[aria-describedby="cited-findings-note"]' as const;

test("依據的認定：店家資料讀取中時欄位照樣出現並說載入中，還不發認定的查詢", async () => {
	const d = openDialog(["ok"], {}, { store: ["hang"] });
	await d.mount({ settle: false });
	const field = dom.window.document.querySelector(CITED_FIELD);
	assert.ok(field, "讀取中欄位也要在");
	assert.match(field.textContent ?? "", /依據的認定/u);
	assert.match(field.textContent ?? "", /載入認定中\.\.\./u);
	assert.equal(d.seen.getStore, 1);
	assert.equal(d.seen.listViolations.length, 0, "帳號還不知道，不能查認定");
	await d.unmount();
});

test("依據的認定：店家資料讀取失敗時顯示失敗原因與「重試」，重試重讀店家詳情後列出認定", async () => {
	const d = openDialog(["ok"], {}, { store: ["fail"] });
	await d.mount();
	const field = dom.window.document.querySelector(CITED_FIELD);
	assert.ok(field, "失敗時欄位也要在");
	const alert = field.querySelector('[role="alert"]');
	assert.equal(alert?.textContent, defaultErrorTextZhTW(Code.Unavailable));
	assert.equal(d.seen.getStore, 1);
	assert.equal(d.seen.listViolations.length, 0);

	await click("重試");
	await until(() => bodyText().includes(FINDING_ID), "重試後列出認定");
	assert.equal(d.seen.getStore, 2, "重試是重讀店家詳情");
	assert.equal(d.seen.listViolations[0]?.accountId, "acc-1");
	assert.ok(checkbox(FINDING_ID));
	assert.equal(
		dom.window.document.querySelector(`${CITED_FIELD} [role="alert"]`),
		null,
	);
	await d.unmount();
});

test("依據的認定：店家資料讀到了卻沒有店家時，說認定讀取失敗並可重試，不是整欄消失", async () => {
	const d = openDialog(["ok"], {}, { store: ["empty"] });
	await d.mount();
	const field = dom.window.document.querySelector(CITED_FIELD);
	assert.ok(field);
	assert.equal(
		field.querySelector('[role="alert"]')?.textContent,
		"認定讀取失敗",
	);
	assert.ok(buttonByText("重試"));
	await click("重試");
	await until(() => bodyText().includes(FINDING_ID), "重試後列出認定");
	await d.unmount();
});

test("來源通報與引用的認定換勾選順序（取消再勾回）是同一份內容：request_id 不變，送出的 id 已排序", async () => {
	const d = openDialog(["fail", "ok"]);
	await d.mount();
	await fillCommon("warning");
	await toggle(ACTION_TAKEN_ID);
	await toggle(NO_OUTCOME_ID);
	await toggle(FINDING_ID);
	await click("確認建立");
	assert.equal(d.requests.length, 1);
	// 取消勾選再勾回：集合不變，勾選順序變成 PENDING、VERIFIED。
	await toggle(ACTION_TAKEN_ID);
	await toggle(ACTION_TAKEN_ID);
	await click("確認建立");
	assert.equal(d.requests.length, 2);
	assert.equal(
		d.requests[1]?.requestId,
		d.requests[0]?.requestId,
		"同一組選擇不能換 request_id",
	);
	for (const req of d.requests) {
		assert.deepEqual(req.measure?.value?.violation?.finding?.sourceReportIds, [
			NO_OUTCOME_ID,
			ACTION_TAKEN_ID,
		]);
		assert.deepEqual(req.measure?.value?.violation?.citedFindingIds, [
			FINDING_ID,
		]);
	}
	await d.unmount();
});

test("從案件開啟：頂端說明措施會記為這個案件導出的措施，請求帶 source_case_id", async () => {
	const d = openDialog(["ok"], { sourceCaseId: "case-77" });
	await d.mount();
	assert.match(bodyText(), /這個措施會記為案件 case-77 導出的措施。/u);
	await fillCommon("warning");
	await click("確認建立");
	assert.equal(d.requests.length, 1);
	assert.equal(d.requests[0]?.sourceCaseId, "case-77");
	await d.unmount();
});

test("不是從案件開啟：沒有頂端說明，請求不帶 source_case_id", async () => {
	const d = openDialog(["ok"]);
	await d.mount();
	assert.doesNotMatch(bodyText(), /導出的措施/u);
	await fillCommon("warning");
	await click("確認建立");
	assert.equal(d.requests[0]?.sourceCaseId ?? "", "");
	assert.equal("sourceCaseId" in (d.requests[0] ?? {}), false);
	await d.unmount();
});

test("來源通報欄下面可以帶說明（預先勾選不只一頁時）", async () => {
	const d = openDialog(["ok"], {
		sourceReportsNote:
			"這個案件超過 20 筆可以當來源的通報，已預先勾選最早的 20 筆。",
	});
	await d.mount();
	assert.match(bodyText(), /已預先勾選最早的 20 筆。/u);
	await d.unmount();

	const plain = openDialog(["ok"]);
	await plain.mount();
	assert.doesNotMatch(bodyText(), /已預先勾選/u);
	await plain.unmount();
});
