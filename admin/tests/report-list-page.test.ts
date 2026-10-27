import assert from "node:assert/strict";
import test from "node:test";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import {
	CaseCloseReason,
	CaseDecision,
	CaseState,
	ReportState,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import {
	bodyText,
	buttonByText,
	click,
	dom,
	mount,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";
import { kase, report, ts } from "./fixtures/report-fixtures";

/**
 * 商家通報清單頁（/admin/violation-reports）實際掛進 DOM 後的行為：三個分頁各自送給伺服器的請求、欄位與空狀態、
 * 讀取中與失敗（可重試）、分頁與篩選（換分頁或篩選回到第一頁）、案件列的標記。
 */

const Page = requireFromTest("../src/app/admin/violation-reports/page")
	.default as unknown;
const h = React.createElement;

type Req = Record<string, unknown>;

function transportWith(handlers: {
	listReports?: (req: Req) => unknown;
	listCases?: (req: Req) => unknown;
}) {
	return createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, handlers as never);
	});
}

function cellTexts(): string[][] {
	return [...dom.window.document.querySelectorAll("tbody tr")].map((tr) =>
		[...tr.querySelectorAll("td")].map((td) => td.textContent ?? ""),
	);
}

function headerTexts(): string[] {
	return [...dom.window.document.querySelectorAll("thead th")].map(
		(th) => th.textContent ?? "",
	);
}

function selectByLabel(label: string): HTMLSelectElement {
	const el = dom.window.document.querySelector(`select[aria-label="${label}"]`);
	assert.ok(el, `找不到篩選「${label}」`);
	return el as HTMLSelectElement;
}

async function choose(label: string, value: number | string) {
	await React.act(async () => {
		setValue(selectByLabel(label), String(value));
	});
}

async function openList(handlers: Parameters<typeof transportWith>[0]) {
	const m = mount(h(Page), transportWith(handlers));
	await m.render();
	return m;
}

const HOUR = 60 * 60 * 1000;

test("標題、說明與三個分頁；預設是待分案", async () => {
	await openList({ listReports: () => ({ reports: [] }) });
	assert.equal(
		dom.window.document.querySelector("h1")?.textContent,
		"商家通報",
	);
	assert.match(
		bodyText(),
		/買家與公眾對店家的通報。先分案：同一家店、同一個類別的通報歸到同一個案件；再在案件裡決定是否採取措施。每 30 秒自動更新。/u,
	);
	assert.deepEqual(
		[...dom.window.document.querySelectorAll(".filter-tab")].map(
			(b) => b.textContent,
		),
		["待分案", "案件", "所有通報"],
	);
	assert.ok(
		dom.window.document.querySelector(".filter-tab.active")?.textContent ===
			"待分案",
	);
});

test("待分案：ListReports 只列 AWAITING_TRIAGE；欄位是店家、檢舉人選的類別、分案期限、建立時間；期限相對現在，絕對時間在 title", async () => {
	const requests: Req[] = [];
	const now = Date.now();
	await openList({
		listReports: (req) => {
			requests.push(req);
			return {
				reports: [
					report({
						id: "r-overdue",
						storeName: "阿里山農場",
						category: PolicyCategory.FOOD_SAFETY,
						createTime: ts(new Date(now - 5.5 * HOUR)),
						triageDeadline: {
							startTime: ts(new Date(now - 5.5 * HOUR)),
							dueTime: ts(new Date(now - 1.5 * HOUR)),
						},
					}),
					report({
						id: "r-none",
						storeName: "",
						storeId: "777",
						category: PolicyCategory.FRAUD,
						createTime: ts(new Date(now - HOUR)),
					}),
				],
			};
		},
	});
	assert.equal(requests.length, 1);
	assert.equal(requests[0].state, ReportState.AWAITING_TRIAGE);
	assert.equal(requests[0].pageSize, 20);
	assert.equal(requests[0].pageToken, "");
	assert.deepEqual(headerTexts(), [
		"店家",
		"檢舉人選的類別",
		"分案期限",
		"建立時間",
	]);
	const rows = cellTexts();
	assert.equal(rows[0][0], "阿里山農場");
	assert.equal(rows[0][1], "食品安全");
	assert.equal(rows[0][2], "已逾期 1 小時 30 分");
	assert.equal(rows[1][0], "777", "沒有店名時是店家 ID");
	assert.equal(rows[1][2], "-", "沒有期限");
	const link = dom.window.document.querySelector(
		'tbody a[href="/admin/violation-reports/r-overdue"]',
	);
	assert.ok(link, "店家連到通報詳情");
	const deadline = dom.window.document.querySelector(".adm-sla--danger");
	assert.ok(deadline, "逾期是 danger");
	assert.match(
		deadline.getAttribute("title") ?? "",
		/^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}$/u,
	);
});

test("待分案：沒有資料時說「沒有待分案的通報」；讀取失敗顯示原因並可重試", async () => {
	let calls = 0;
	await openList({
		listReports: () => {
			calls++;
			if (calls === 1) throw new ConnectError("down", Code.Unavailable);
			return { reports: [] };
		},
	});
	assert.ok(bodyText().includes(defaultErrorTextZhTW(Code.Unavailable)));
	await click("重試");
	await until(
		() => bodyText().includes("沒有待分案的通報"),
		"重試後顯示空狀態",
	);
	assert.equal(calls, 2);
});

test("案件：ListCases 的欄位與狀態、有新的補充的標記、狀態篩選全部｜進行中｜等賣家｜已結案", async () => {
	const requests: Req[] = [];
	await openList({
		listReports: () => ({ reports: [] }),
		listCases: (req) => {
			requests.push(req);
			return {
				cases: [
					kase({
						id: "c-open",
						storeName: "阿里山農場",
						category: PolicyCategory.FOOD_SAFETY,
						severity: ViolationSeverity.HIGH,
						reportCount: 3,
						unseenSupplementReportCount: 1,
						awaitingSeller: true,
					}),
					kase({
						id: "c-closed",
						storeName: "玉山果園",
						category: PolicyCategory.FRAUD,
						severity: ViolationSeverity.LOW,
						state: CaseState.CLOSED,
						closeReason: CaseCloseReason.DECIDED,
						decision: CaseDecision.NO_ACTION,
						reportCount: 1,
					}),
				],
			};
		},
	});
	await click("案件");
	await until(
		() => requests.length > 0 && cellTexts().length === 2,
		"案件列出現",
	);
	assert.equal(requests[0].state, CaseState.UNSPECIFIED);
	assert.deepEqual(headerTexts(), [
		"店家",
		"類別",
		"嚴重度",
		"狀態",
		"通報數",
		"開案時間",
	]);
	const rows = cellTexts();
	assert.deepEqual(rows[0].slice(0, 5), [
		"阿里山農場",
		"食品安全",
		"高",
		"進行中等賣家有新的補充",
		"3",
	]);
	assert.deepEqual(rows[1].slice(0, 5), [
		"玉山果園",
		"詐欺或操縱交易",
		"低",
		"已結案：不採取措施",
		"1",
	]);
	assert.ok(
		dom.window.document.querySelector(
			'tbody a[href="/admin/violation-reports/cases/c-open"]',
		),
	);
	assert.deepEqual(
		[...selectByLabel("狀態").options].map((o) => o.textContent),
		["全部", "進行中", "等賣家", "已結案"],
	);
	await choose("狀態", "open");
	await until(() => requests.length === 2, "改狀態篩選後重讀");
	assert.equal(requests[1].state, CaseState.OPEN);
	assert.equal(requests[1].awaitingSeller, false);
	assert.equal(requests[1].pageToken, "");
	await choose("狀態", "awaiting-seller");
	await until(() => requests.length === 3, "改成等賣家後重讀");
	assert.equal(requests[2].state, CaseState.OPEN);
	assert.equal(requests[2].awaitingSeller, true);
});

test("案件：沒有案件說「尚無案件」，有篩選時說「沒有符合條件的案件」", async () => {
	await openList({
		listReports: () => ({ reports: [] }),
		listCases: () => ({ cases: [] }),
	});
	await click("案件");
	await until(() => bodyText().includes("尚無案件"), "空狀態");
	await choose("狀態", "closed");
	await until(
		() => bodyText().includes("沒有符合條件的案件"),
		"篩選後的空狀態",
	);
});

test("所有通報：不篩狀態；欄位是店家、類別（生效的）、狀態、案件、建立時間；結果當狀態、案件連到案件詳情", async () => {
	const requests: Req[] = [];
	await openList({
		listReports: (req) => {
			requests.push(req);
			return {
				reports: [
					report({
						id: "r-done",
						state: ReportState.RESOLVED,
						outcome: ReportOutcome.ACTION_TAKEN,
						category: PolicyCategory.OTHER,
						assessedCategory: PolicyCategory.PRIVACY,
						effectiveCategory: PolicyCategory.PRIVACY,
						caseId: "c-9",
					}),
					report({
						id: "r-wait",
						state: ReportState.AWAITING_TRIAGE,
						category: PolicyCategory.OTHER,
						effectiveCategory: PolicyCategory.OTHER,
					}),
				],
			};
		},
	});
	await click("所有通報");
	await until(
		() => requests.length === 2 && cellTexts()[0]?.[0] === "阿里山農場",
		"切到所有通報",
	);
	assert.equal(requests[1].state, ReportState.UNSPECIFIED);
	assert.deepEqual(headerTexts(), ["店家", "類別", "狀態", "案件", "建立時間"]);
	const rows = cellTexts();
	assert.deepEqual(rows[0].slice(0, 4), [
		"阿里山農場",
		"不當使用個人資料",
		"已採取措施",
		"案件 c-9",
	]);
	assert.deepEqual(rows[1].slice(0, 4), ["阿里山農場", "其他", "待分案", "-"]);
	assert.ok(
		dom.window.document.querySelector(
			'tbody a[href="/admin/violation-reports/cases/c-9"]',
		),
	);
});

test("類別篩選的選項：待分案 12 類、案件 12 類（沒有其他）、所有通報 13 類，第一個是「所有類別」；換分頁清掉篩選", async () => {
	const requests: Req[] = [];
	await openList({
		listReports: (req) => {
			requests.push(req);
			return { reports: [] };
		},
		listCases: () => ({ cases: [] }),
	});
	const texts = () =>
		[...selectByLabel("類別").options].map((o) => o.textContent);
	assert.equal(texts()[0], "所有類別");
	assert.equal(texts().length, 13);
	assert.ok(texts().includes("其他"));
	assert.ok(!texts().includes("干擾或濫用平台"));
	await choose("類別", PolicyCategory.FRAUD);
	await until(() => requests.length === 2, "篩選後重讀");
	assert.equal(requests[1].category, PolicyCategory.FRAUD);
	await click("案件");
	await until(
		() => texts().length === 13 && !texts().includes("其他"),
		"案件分頁的選項",
	);
	assert.ok(texts().includes("干擾或濫用平台"));
	assert.equal(
		selectByLabel("類別").value,
		String(PolicyCategory.UNSPECIFIED),
		"換分頁清掉類別篩選",
	);
	await click("所有通報");
	await until(() => texts().length === 14, "所有通報分頁的選項");
	assert.ok(texts().includes("其他") && texts().includes("干擾或濫用平台"));
});

test("分頁：有下一頁才顯示；下一頁帶 page token，上一頁回到前一頁；換篩選回到第一頁", async () => {
	const tokens: string[] = [];
	await openList({
		listReports: (req) => {
			tokens.push(req.pageToken as string);
			return {
				reports: [report({ id: `r-${tokens.length}` })],
				nextPageToken: req.pageToken === "" ? "t2" : "",
			};
		},
	});
	assert.match(bodyText(), /第 1 頁/u);
	assert.ok(buttonByText(" 上一頁"), "第一頁也要有上一頁鈕（停用）");
	assert.equal(buttonByText(" 上一頁")?.disabled, true);
	await click("下一頁 ");
	await until(
		() =>
			dom.window.document.querySelector("tbody a")?.getAttribute("href") ===
			"/admin/violation-reports/r-2",
		"下一頁",
	);
	assert.equal(tokens[1], "t2");
	assert.match(bodyText(), /第 2 頁/u);
	assert.equal(
		dom.window.document.querySelector("tbody a")?.getAttribute("href"),
		"/admin/violation-reports/r-2",
	);
	await choose("類別", PolicyCategory.FRAUD);
	await until(() => tokens.length === 3, "換篩選");
	assert.equal(tokens[2], "", "換篩選回到第一頁");
});
