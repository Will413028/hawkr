import assert from "node:assert/strict";
import test from "node:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import { createRouterTransport, type Transport } from "@connectrpc/connect";
import {
	CaseSchema,
	CaseState,
	type ListCasesRequestSchema,
	ModerationAdminService,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { ReportTargetKind } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import {
	bodyText,
	byId,
	clickElement,
	dom,
	mount,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";

/**
 * 社群檢舉案件清單（/admin/reports）：一個案件一列、預設只看進行中、keyset 的上一頁與下一頁
 * （上一頁退回走過的前一個 page token）、換篩選回第一頁。
 */

const Page = requireFromTest("../src/app/admin/reports/page")
	.default as unknown;
const h = React.createElement;

const T0 = Date.UTC(2026, 10, 1, 0, 0, 0);

function kase(id: string, patch: MessageInitShape<typeof CaseSchema> = {}) {
	const init = {
		id,
		target: { kind: ReportTargetKind.COMMENT, id: `content-${id}` },
		state: CaseState.OPEN,
		createTime: timestampFromMs(T0),
		etag: "e1",
		reportCount: BigInt(1),
		...patch,
	};
	return create(CaseSchema, init as MessageInitShape<typeof CaseSchema>);
}

type Req = ReturnType<typeof create<typeof ListCasesRequestSchema>>;

function setup(pages: Record<string, { ids: string[]; next: string }>) {
	const requests: Req[] = [];
	const transport: Transport = createRouterTransport(({ service }) => {
		service(ModerationAdminService, {
			listCases: (req: Req) => {
				requests.push(req);
				const page = pages[req.pageToken] ?? { ids: [], next: "" };
				return {
					cases: page.ids.map((id, i) =>
						kase(id, { reportCount: BigInt(i + 4) }),
					),
					nextPageToken: page.next,
				};
			},
		} as never);
	});
	return { requests, transport };
}

async function open(transport: Transport) {
	const m = mount(h(Page), transport);
	await m.render();
	return m;
}

const pager = (label: string) =>
	[...dom.window.document.querySelectorAll("button")].find((b) =>
		b.textContent?.includes(label),
	) as HTMLButtonElement;
const click = (label: string) => clickElement(pager(label));

const rowTexts = () =>
	[...dom.window.document.querySelectorAll("tbody tr")].map((tr) =>
		[...tr.querySelectorAll("td")].map((td) => td.textContent),
	);

const PAGES = {
	"": { ids: ["c1", "c2"], next: "t2" },
	t2: { ids: ["c3"], next: "t3" },
	t3: { ids: ["c4"], next: "" },
};

test("一個案件一列：對象種類、對象、狀態；預設只看進行中、第一頁 token 為空", async () => {
	const { requests, transport } = setup(PAGES);
	await open(transport);
	const first = requests[0];
	assert.equal(first?.state, CaseState.OPEN);
	assert.equal(first?.targetKind, ReportTargetKind.UNSPECIFIED);
	assert.equal(first?.pageToken, "");
	const rows = rowTexts();
	assert.equal(rows.length, 2);
	assert.deepEqual(rows[0]?.slice(0, 4), ["留言", "content-c1", "4", "進行中"]);
	assert.equal(rows[1]?.[2], "5");
	assert.ok(
		[...dom.window.document.querySelectorAll("thead th")].some(
			(th) => th.textContent === "檢舉數",
		),
	);
	const link = dom.window.document.querySelector("tbody a");
	assert.equal(link?.getAttribute("href"), "/admin/moderation/cases/c1");
	assert.ok(!bodyText().includes("第 2 頁"));
});

test("下一頁帶回傳的 token，上一頁退回前一個 token", async () => {
	const { requests, transport } = setup(PAGES);
	await open(transport);
	await click("下一頁");
	await until(() => bodyText().includes("content-c3"), "第二頁的列");
	await click("下一頁");
	await until(() => requests.at(-1)?.pageToken === "t3", "第三頁的請求");
	await until(() => bodyText().includes("content-c4"), "第三頁的列");
	assert.equal(pager("下一頁").disabled, true);
	await click("上一頁");
	await until(() => bodyText().includes("content-c3"), "回到第二頁的列");
	assert.equal(requests.at(-1)?.pageToken, "t2");
	assert.ok(bodyText().includes("第 2 頁"));
	await click("上一頁");
	await until(() => bodyText().includes("content-c1"), "回到第一頁的列");
	assert.equal(requests.at(-1)?.pageToken, "");
});

test("換篩選回第一頁，並帶新的狀態與對象種類", async () => {
	const { requests, transport } = setup(PAGES);
	await open(transport);
	await click("下一頁");
	await until(() => requests.at(-1)?.pageToken === "t2", "第二頁的請求");
	const select = dom.window.document.querySelector(
		'select[aria-label="對象種類"]',
	);
	assert.ok(select);
	await React.act(async () => {
		setValue(select, String(ReportTargetKind.POST));
	});
	await until(
		() => requests.at(-1)?.targetKind === ReportTargetKind.POST,
		"換對象種類的請求",
	);
	assert.equal(requests.at(-1)?.pageToken, "");
	assert.equal(requests.at(-1)?.state, CaseState.OPEN);
	const state = dom.window.document.querySelector(
		'select[aria-label="案件狀態"]',
	);
	assert.ok(state);
	await React.act(async () => {
		setValue(state, String(CaseState.CLOSED));
	});
	await until(
		() => requests.at(-1)?.state === CaseState.CLOSED,
		"換狀態的請求",
	);
	assert.equal(requests.at(-1)?.pageToken, "");
});

test("沒有符合的案件顯示空狀態", async () => {
	const { transport } = setup({ "": { ids: [], next: "" } });
	await open(transport);
	assert.ok(bodyText().includes("沒有符合條件的案件"));
	assert.ok(byId);
});
