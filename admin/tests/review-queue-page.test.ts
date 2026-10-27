import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test, { mock } from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	Code,
	ConnectError,
	createRouterTransport,
	type Transport,
} from "@connectrpc/connect";
import {
	ReviewQueueAdminService,
	type ReviewQueueItem,
	ReviewQueueItemKind,
	ReviewQueueItemSchema,
} from "@hawkr/api/gen/hawkr/admin/reviewqueue/v1alpha1/review_queue_pb";
import { DeadlineSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/deadline_pb";
import {
	bodyText,
	click,
	dom,
	mount,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";
import { ts, withMessage } from "./fixtures/report-fixtures";

/**
 * 待辦頁（設計 step10e-3d §4.1、§5 #100–#114）：篩選送出的 kinds、換篩選回第一頁、欄位與店家欄、期限（固定 now）、
 * 空狀態、讀取失敗與重試、分頁、30 秒更新的設定、連結與三個錨點。
 */

const NOW = new Date("2026-10-20T04:00:00Z");
const HOUR = 60 * 60 * 1000;
const DAY = 24 * HOUR;
const h = React.createElement;

const Page = requireFromTest("../src/app/admin/queue/page").default as unknown;
const { itemHref, QUEUE_KIND_TEXT, queueKindText, queueStoreText } =
	requireFromTest(
		"../src/app/admin/queue/queue-item",
	) as typeof import("../src/app/admin/queue/queue-item");

type Init = Parameters<typeof create<typeof ReviewQueueItemSchema>>[1];
const upcoming = () =>
	create(DeadlineSchema, {
		startTime: ts(new Date(NOW.getTime() - DAY)),
		dueTime: ts(new Date(NOW.getTime() + 2 * DAY)),
	});
const overdue = () =>
	create(DeadlineSchema, {
		startTime: ts(new Date(NOW.getTime() - 5 * DAY)),
		dueTime: ts(new Date(NOW.getTime() - 3 * HOUR)),
	});
function item(patch: Init = {}): ReviewQueueItem {
	return create(ReviewQueueItemSchema, {
		kind: ReviewQueueItemKind.CASE_DECISION,
		subject: { case: "caseId", value: "c-1" },
		storeId: "55",
		storeName: "阿里山農場",
		deadline: upcoming(),
		...patch,
	});
}

type Req = { kinds: ReviewQueueItemKind[]; pageToken: string };
function transportWith(handler: (req: Req) => unknown): Transport {
	return createRouterTransport(({ service }) => {
		service(ReviewQueueAdminService, { listReviewQueue: handler } as never);
	});
}

let current: { unmount: () => Promise<void> } | undefined;
async function open(transport: Transport) {
	await current?.unmount();
	const m = mount(h(Page), transport);
	current = m;
	await m.render();
	return m;
}
async function withNow<T>(run: () => Promise<T>): Promise<T> {
	mock.timers.enable({ apis: ["Date"], now: NOW });
	try {
		return await run();
	} finally {
		mock.timers.reset();
	}
}
const doc = () => dom.window.document;
const cells = () =>
	[...doc().querySelectorAll("tbody tr")].map((tr) =>
		[...tr.querySelectorAll("td")].map((td) => td.textContent),
	);

const ALL_KIND_VALUES = (
	Object.values(ReviewQueueItemKind).filter(
		(v) => typeof v === "number",
	) as ReviewQueueItemKind[]
).filter((v) => v !== ReviewQueueItemKind.UNSPECIFIED);

test("itemHref：12 種逐種（含錨點）", () => {
	const cases: [Init, string][] = [
		[
			{
				kind: ReviewQueueItemKind.REPORT_TRIAGE,
				subject: { case: "reportId", value: "r1" },
			},
			"/admin/violation-reports/r1",
		],
		[
			{
				kind: ReviewQueueItemKind.CASE_DECISION,
				subject: { case: "caseId", value: "c1" },
			},
			"/admin/violation-reports/cases/c1",
		],
		[
			{
				kind: ReviewQueueItemKind.CASE_INQUIRY_OVERDUE,
				subject: {
					case: "caseInquiry",
					value: { caseId: "c1", inquiryId: "i1" },
				},
			},
			"/admin/violation-reports/cases/c1#inquiry-i1",
		],
		[
			{
				kind: ReviewQueueItemKind.CASE_FDA_REFERRAL,
				subject: { case: "caseId", value: "c2" },
			},
			"/admin/violation-reports/cases/c2",
		],
		[
			{
				kind: ReviewQueueItemKind.AUTHORITY_ORDER,
				subject: { case: "authorityOrderId", value: "o1" },
			},
			"/admin/authority-orders/o1",
		],
		[
			{
				kind: ReviewQueueItemKind.AUTHORITY_TARGET_NOTICE,
				subject: {
					case: "authorityTarget",
					value: { orderId: "o1", position: 2, accountId: "9" },
				},
			},
			"/admin/authority-orders/o1#target-2",
		],
		[
			{
				kind: ReviewQueueItemKind.MEASURE_APPEAL,
				subject: {
					case: "measureRequest",
					value: { measureId: "m1", requestId: "q1" },
				},
			},
			"/admin/measures/m1",
		],
		[
			{
				kind: ReviewQueueItemKind.MEASURE_LIFT_REQUEST,
				subject: {
					case: "measureRequest",
					value: { measureId: "m2", requestId: "q2" },
				},
			},
			"/admin/measures/m2",
		],
		[
			{
				kind: ReviewQueueItemKind.MEASURE_REVIEW,
				subject: { case: "measureId", value: "m3" },
			},
			"/admin/measures/m3",
		],
		[
			{
				kind: ReviewQueueItemKind.FOOD_REGISTRATION,
				subject: { case: "foodRegistrationId", value: "f1" },
			},
			"/admin/food-registrations/f1",
		],
		[
			{
				kind: ReviewQueueItemKind.KYC_APPLICATION,
				subject: {
					case: "kycApplication",
					value: { storeId: "55", applicationId: "a1" },
				},
			},
			"/admin/merchants/55#kyc",
		],
		[
			{
				kind: ReviewQueueItemKind.PAYUNI_BIND_RETRY,
				subject: {
					case: "kycApplication",
					value: { storeId: "56", applicationId: "a2" },
				},
			},
			"/admin/merchants/56#kyc",
		],
	];
	assert.equal(cases.length, ALL_KIND_VALUES.length, "每種都有一筆");
	for (const [patch, want] of cases) assert.equal(itemHref(item(patch)), want);
	assert.equal(
		itemHref(item({ subject: { case: undefined, value: undefined } })),
		undefined,
	);
});

test("種類標籤：涵蓋 enum 的每個非 UNSPECIFIED 值；UNSPECIFIED 不顯示", () => {
	for (const value of ALL_KIND_VALUES) {
		assert.ok(queueKindText(value), `${ReviewQueueItemKind[value]} 沒有文字`);
	}
	assert.equal(Object.keys(QUEUE_KIND_TEXT).length, ALL_KIND_VALUES.length);
	assert.equal(queueKindText(ReviewQueueItemKind.UNSPECIFIED), undefined);
	// §5 #104a–k 逐字：改錯任一字都要失敗。
	const expected: [ReviewQueueItemKind, string][] = [
		[ReviewQueueItemKind.REPORT_TRIAGE, "分案通報"],
		[ReviewQueueItemKind.CASE_DECISION, "決定案件"],
		[ReviewQueueItemKind.CASE_INQUIRY_OVERDUE, "跟進逾期的詢問"],
		[ReviewQueueItemKind.CASE_FDA_REFERRAL, "轉報主管機關"],
		[ReviewQueueItemKind.AUTHORITY_ORDER, "處理主管機關來函"],
		[ReviewQueueItemKind.AUTHORITY_TARGET_NOTICE, "確認是否告知被指名者"],
		[ReviewQueueItemKind.MEASURE_APPEAL, "裁定申覆"],
		[ReviewQueueItemKind.MEASURE_LIFT_REQUEST, "裁定解除請求"],
		[ReviewQueueItemKind.MEASURE_REVIEW, "複審停權"],
		[ReviewQueueItemKind.FOOD_REGISTRATION, "審核食品業者登錄"],
		[ReviewQueueItemKind.KYC_APPLICATION, "審核商家認證"],
		[ReviewQueueItemKind.PAYUNI_BIND_RETRY, "重新綁定 PayUni"],
	];
	assert.equal(expected.length, ALL_KIND_VALUES.length);
	for (const [kind, text] of expected) {
		assert.equal(queueKindText(kind), text, ReviewQueueItemKind[kind]);
	}
});

test("店家欄（#113）：店名｜店家 {id}｜-｜帳號 {id}", () => {
	assert.equal(queueStoreText(item()), "阿里山農場");
	assert.equal(queueStoreText(item({ storeName: "" })), "店家 55");
	assert.equal(
		queueStoreText(
			item({
				storeId: "",
				storeName: "",
				kind: ReviewQueueItemKind.AUTHORITY_ORDER,
				subject: { case: "authorityOrderId", value: "o1" },
			}),
		),
		"-",
	);
	assert.equal(
		queueStoreText(
			item({
				storeId: "",
				storeName: "",
				kind: ReviewQueueItemKind.AUTHORITY_TARGET_NOTICE,
				subject: {
					case: "authorityTarget",
					value: { orderId: "o1", position: 0, accountId: "101" },
				},
			}),
		),
		"帳號 101",
	);
});

test("頁面：標題、說明、欄位、列內容、連結；期限未逾期無色階、逾期 danger；title 是絕對時間", async () => {
	await withNow(async () => {
		await open(
			transportWith(() => ({
				items: [
					item(),
					item({
						kind: ReviewQueueItemKind.MEASURE_REVIEW,
						subject: { case: "measureId", value: "m3" },
						storeName: "",
						deadline: overdue(),
					}),
				],
			})),
		);
		assert.equal(doc().querySelector("h1")?.textContent, "待辦");
		assert.match(
			bodyText(),
			/通報與案件、主管機關來函、措施、食品業者登錄與商家認證中等待處理的事項，依期限由急到緩。每\s*30 秒自動更新。/u,
		);
		assert.deepEqual(
			[...doc().querySelectorAll("thead th")].map((th) => th.textContent),
			["種類", "店家", "開始時間", "期限"],
		);
		const rows = cells();
		assert.deepEqual(rows[0], [
			"決定案件",
			"阿里山農場",
			"2026/10/19 12:00",
			"剩 2 天",
		]);
		assert.deepEqual(rows[1], [
			"複審停權",
			"店家 55",
			"2026/10/15 12:00",
			"已逾期 3 小時",
		]);
		const slas = doc().querySelectorAll("tbody .adm-sla");
		assert.equal(slas[0].className.includes("adm-sla--"), false);
		assert.ok(slas[1].classList.contains("adm-sla--danger"));
		assert.equal(slas[1].getAttribute("title"), "2026/10/20 09:00");
		assert.ok(
			doc().querySelector('tbody a[href="/admin/violation-reports/cases/c-1"]'),
		);
		assert.ok(doc().querySelector('tbody a[href="/admin/measures/m3"]'));
	});
});

test("篩選：所有種類送空陣列、單選送一個；換篩選回第一頁（不帶舊 token）", async () => {
	const requests: Req[] = [];
	await open(
		transportWith((req) => {
			requests.push(req);
			return { items: [item()], nextPageToken: "t2" };
		}),
	);
	assert.deepEqual(requests[0].kinds, []);
	const select = doc().querySelector("select") as HTMLSelectElement;
	assert.deepEqual(
		[...select.options].map((o) => o.textContent),
		[
			"所有種類",
			...ALL_KIND_VALUES.map(
				(v) => QUEUE_KIND_TEXT[v as keyof typeof QUEUE_KIND_TEXT],
			),
		],
	);
	await click("下一頁 ");
	await until(() => requests.at(-1)?.pageToken === "t2", "第二頁");
	assert.match(bodyText(), /第 2 頁/u);
	await React.act(async () => {
		setValue(select, String(ReviewQueueItemKind.KYC_APPLICATION));
	});
	await until(
		() =>
			requests.at(-1)?.kinds.length === 1 && requests.at(-1)?.pageToken === "",
		"換篩選",
	);
	assert.deepEqual(requests.at(-1)?.kinds, [
		ReviewQueueItemKind.KYC_APPLICATION,
	]);
	assert.match(bodyText(), /第 1 頁/u);
	await React.act(async () => {
		setValue(select, "");
	});
	await until(() => requests.at(-1)?.kinds.length === 0, "回到所有種類");
});

test("空狀態：沒有篩選「目前沒有待辦」，有篩選「沒有這個種類的待辦」", async () => {
	await open(transportWith(() => ({ items: [] })));
	assert.match(bodyText(), /目前沒有待辦/u);
	const select = doc().querySelector("select") as HTMLSelectElement;
	await React.act(async () => {
		setValue(select, String(ReviewQueueItemKind.MEASURE_APPEAL));
	});
	await until(() => bodyText().includes("沒有這個種類的待辦"), "篩選後空狀態");
	assert.doesNotMatch(bodyText(), /目前沒有待辦/u);
});

test("讀取失敗：網路層「待辦讀取失敗」、Connect 錯誤照伺服器的句子；重試成功後列出", async () => {
	let calls = 0;
	await open(
		transportWith(() => {
			calls += 1;
			if (calls === 1) throw withMessage(Code.Internal, "伺服器說的句子");
			return { items: [item()] };
		}),
	);
	assert.match(bodyText(), /伺服器說的句子/u);
	await click("重試");
	await until(
		() => cells().length === 1 && cells()[0][0] === "決定案件",
		"重試後列出",
	);

	const inner = transportWith(() => ({ items: [] }));
	const down: Transport = {
		...inner,
		unary: (() =>
			Promise.reject(
				ConnectError.from(new TypeError("Failed to fetch")),
			)) as Transport["unary"],
		stream: inner.stream.bind(inner),
	};
	await open(down);
	assert.match(bodyText(), /待辦讀取失敗/u);
});

test("分頁：只有一頁且沒有下一頁時不顯示分頁列", async () => {
	await open(transportWith(() => ({ items: [item()] })));
	assert.doesNotMatch(bodyText(), /第 \d+ 頁/u);
});

test("30 秒更新：頁面以 LIST_REFRESH_MS 輪詢，常數是 30 秒", async () => {
	const { LIST_REFRESH_MS } = requireFromTest("../src/lib/use-now") as {
		LIST_REFRESH_MS: number;
	};
	assert.equal(LIST_REFRESH_MS, 30_000);
	const source = await readFile("src/app/admin/queue/page.tsx", "utf8");
	assert.match(source, /refetchInterval: LIST_REFRESH_MS/u);
	assert.match(source, /useNow\(LIST_REFRESH_MS\)/u);
});

test("側欄：「待辦」是「賣家 / 商品」的第一項（錨點在 scroll-to-hash.test.ts 掛載真實頁面驗證）", async () => {
	const sidebar = await readFile("src/components/Layout/Sidebar.tsx", "utf8");
	const section = sidebar.indexOf('section: "賣家 / 商品"');
	const queue = sidebar.indexOf('href: "/admin/queue"');
	assert.ok(queue > section);
	assert.ok(queue < sidebar.indexOf('href: "/admin/merchants"'), "最上面");
	assert.match(sidebar, /label: "待辦"/u);
});
