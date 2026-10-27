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
	AuthorityHoldExtensionCause,
	AuthorityOrderKind,
	AuthorityOrderSchema,
	AuthorityOrderState,
	AuthorityStoreCoverage,
	AuthorityTargetNoticeKind,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { DeadlineSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/deadline_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	MeasureKind,
	RiskGround,
	TerminationGround,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	bodyText,
	buttonByText,
	byId,
	click,
	dom,
	maybeById,
	mount,
	navigation,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";
import {
	T0,
	ts,
	versionConflict,
	withMessage,
} from "./fixtures/report-fixtures";

/**
 * 主管機關來函的三個頁面（3c-b-5）：清單（分頁籤、欄位、空狀態、讀取失敗）、登記頁（期限預設、店家下拉、請求、導向詳情）、
 * 詳情（各區塊、涵蓋狀態、告知、保存與延長歷史、完成與延長與記下告知、版本衝突、從來函建立三種措施並帶 source_authority_order_id）。
 */

const ListPage = requireFromTest("../src/app/admin/authority-orders/page")
	.default as unknown;
const NewPage = requireFromTest("../src/app/admin/authority-orders/new/page")
	.default as unknown;
const DetailPage = requireFromTest(
	"../src/app/admin/authority-orders/[id]/page",
).default as unknown;
const h = React.createElement;

type Req = Record<string, unknown>;
type Handler = (req: Req) => unknown;

const ORDER_ID = "0190b000-0000-7000-8000-0000000000d1";

const plain = (value: unknown) =>
	JSON.parse(
		JSON.stringify(value, (key, v) =>
			key === "$typeName" ? undefined : typeof v === "bigint" ? Number(v) : v,
		),
	);

function order(
	patch: Parameters<typeof create<typeof AuthorityOrderSchema>>[1] = {},
) {
	return create(AuthorityOrderSchema, {
		id: ORDER_ID,
		agency: "新北市衛生局",
		documentNo: "北衛食字第 1150001 號",
		receiveTime: ts(T0),
		kind: AuthorityOrderKind.SUSPEND_ACCOUNT,
		summary: "請暫停該帳號",
		state: AuthorityOrderState.OPEN,
		deadline: create(DeadlineSchema, {
			startTime: ts(T0),
			dueTime: ts(new Date(T0.getTime() + 2 * 86400000)),
		}),
		holdUntil: ts(new Date("2027-04-21T02:00:00Z")),
		holdActive: true,
		etag: "o-1",
		targets: [
			{
				accountId: "101",
				storeId: "55",
				storeName: "阿里山農場",
				coveredStores: [
					{
						storeId: "55",
						storeName: "阿里山農場",
						coverage: AuthorityStoreCoverage.HELD,
						canCreateMeasure: true,
						canCreateTermination: true,
					},
				],
			},
		],
		...patch,
	});
}

function transportWith(
	gov: Record<string, Handler>,
	seller: Record<string, Handler> = {},
): Transport {
	return createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listAuthorityOrders: () => ({ orders: [] }),
			getAuthorityOrder: () => ({ order: order() }),
			listMeasures: () => ({ measures: [] }),
			listReports: () => ({ reports: [] }),
			...gov,
		} as never);
		service(SellerAdminService, {
			getStore: () => ({ store: { id: "55", accountId: "101" } }),
			listStores: () => ({ stores: [] }),
			...seller,
		} as never);
	});
}

let current: { unmount: () => Promise<void> } | undefined;
async function open(page: unknown, transport: Transport, params = {}) {
	await current?.unmount();
	navigation.params = params;
	const m = mount(h(page), transport);
	current = m;
	await m.render();
	return m;
}

const footerButton = (text: string) =>
	[...dom.window.document.querySelectorAll(".modal-footer button")].find(
		(b) => b.textContent === text,
	) as HTMLElement | undefined;
async function submit(text: string) {
	const button = footerButton(text);
	assert.ok(button, `找不到對話框的按鈕「${text}」`);
	await React.act(async () => {
		button.click();
	});
}
const fill = (id: string, value: string) =>
	React.act(async () => {
		setValue(byId(id), value);
	});
const headers = () =>
	[...dom.window.document.querySelectorAll("thead th")].map(
		(th) => th.textContent,
	);
const rows = () =>
	[...dom.window.document.querySelectorAll("tbody tr")].map((tr) =>
		[...tr.querySelectorAll("td")].map((td) => td.textContent),
	);

// ---------------------------------------------------------------------------
// 清單

test("清單：處理中分頁的欄位與請求（state OPEN）、指名的店家、期限用相對時間、文號連到詳情；頁首登記來函連到 /new", async () => {
	const requests: Req[] = [];
	await open(
		ListPage,
		transportWith({
			listAuthorityOrders: (req) => {
				requests.push(req);
				return {
					orders: [
						order(),
						order({
							id: "o-2",
							documentNo: "第二號",
							targets: [
								{ accountId: "102", storeId: "", coveredStores: [] },
								{ accountId: "103", storeId: "", coveredStores: [] },
							],
						}),
					],
				};
			},
		}),
	);
	assert.equal(requests[0].state, AuthorityOrderState.OPEN);
	assert.deepEqual(headers(), [
		"機關",
		"文號",
		"種類",
		"指名的店家",
		"期限",
		"收文時間",
	]);
	const first = rows()[0];
	assert.deepEqual(first.slice(0, 4), [
		"新北市衛生局",
		"北衛食字第 1150001 號",
		"暫停帳號",
		"阿里山農場",
	]);
	assert.match(first[4] ?? "", /^(剩|已逾期) /u);
	assert.equal(
		rows()[1][3],
		"帳號 102 的全部店家",
		"沒有店家資料：第一個對象的名稱",
	);
	const link = dom.window.document.querySelector(
		`tbody a[href="/admin/authority-orders/${ORDER_ID}"]`,
	);
	assert.ok(link);
	assert.equal(
		dom.window.document.querySelector('a[href="/admin/authority-orders/new"]')
			?.textContent,
		"登記來函",
	);
	assert.match(bodyText(), /主管機關來函/u);
	assert.match(
		bodyText(),
		/來函指名的帳號或店家，資料會保存到保存期限，不會被關店後的清理刪除。每\s*30 秒自動更新。/u,
	);
});

test("清單：已完成分頁換欄位（完成時間、保存到）並請求 COMPLETED；兩個分頁各自的空狀態", async () => {
	const requests: Req[] = [];
	await open(
		ListPage,
		transportWith({
			listAuthorityOrders: (req) => {
				requests.push(req);
				return { orders: [] };
			},
		}),
	);
	assert.match(bodyText(), /沒有處理中的來函/u);
	await click("已完成");
	await until(() => requests.length === 2, "換分頁後重讀");
	assert.equal(requests[1].state, AuthorityOrderState.COMPLETED);
	assert.equal(requests[1].pageToken, "");
	assert.deepEqual(headers(), [
		"機關",
		"文號",
		"種類",
		"指名的店家",
		"完成時間",
		"保存到",
	]);
	await until(() => bodyText().includes("尚無來函"), "已完成分頁的空狀態");
});

test("清單：讀取失敗顯示伺服器的句子與重試；重試成功後列出", async () => {
	let calls = 0;
	await open(
		ListPage,
		transportWith({
			listAuthorityOrders: () => {
				calls++;
				if (calls === 1)
					throw withMessage(Code.Unavailable, "來函服務暫時無法使用。");
				return { orders: [order()] };
			},
		}),
	);
	assert.match(bodyText(), /來函服務暫時無法使用。/u);
	await click("重試");
	await until(() => bodyText().includes("北衛食字第 1150001 號"), "重試後列出");
});

test("清單：有下一頁才顯示分頁；下一頁帶 page token", async () => {
	const tokens: string[] = [];
	await open(
		ListPage,
		transportWith({
			listAuthorityOrders: (req) => {
				tokens.push(req.pageToken as string);
				return { orders: [order()], nextPageToken: req.pageToken ? "" : "p2" };
			},
		}),
	);
	await click("下一頁 ");
	await until(() => tokens.length === 2, "第二頁");
	assert.deepEqual(tokens, ["", "p2"]);
	assert.ok(buttonByText("下一頁 ")?.disabled);
});

// ---------------------------------------------------------------------------
// 登記頁

test("登記頁：期限一開始是空的，選種類也不預填；沒填就不帶 due_time（伺服器取預設），填了才送；成功導向詳情", async () => {
	const created: Req[] = [];
	await open(
		NewPage,
		transportWith({
			createAuthorityOrder: (req) => {
				created.push(req);
				return { order: order() };
			},
		}),
	);
	assert.match(bodyText(), /登記來函/u);
	assert.equal(byId<HTMLInputElement>("authority-new-due").value, "");
	await fill("authority-new-receive-time", "2026-10-19T09:00");
	await fill("authority-new-kind", String(AuthorityOrderKind.SUSPEND_ACCOUNT));
	assert.equal(
		byId<HTMLInputElement>("authority-new-due").value,
		"",
		"選種類不預填期限",
	);
	assert.match(bodyText(), /依種類預設：限制或移除商品、暫停帳號 48 小時/u);
	await fill("authority-new-kind", String(AuthorityOrderKind.OTHER));
	await fill("authority-new-agency", " 新北市衛生局 ");
	await fill("authority-new-document-no", "北衛食字第 1150001 號");
	await fill("authority-new-summary", "請暫停該帳號");
	await fill("authority-target-0-account", "101");
	await click("登記");
	await until(() => created.length === 1, "送出");
	assert.equal("dueTime" in plain(created[0]), false, "沒填期限就不帶");
	await until(
		() => navigation.pushed.includes(`/admin/authority-orders/${ORDER_ID}`),
		"導向詳情",
	);

	await open(
		NewPage,
		transportWith({
			createAuthorityOrder: (req) => {
				created.push(req);
				return { order: order() };
			},
		}),
	);
	created.length = 0;
	await fill("authority-new-receive-time", "2026-10-19T09:00");
	await fill("authority-new-kind", String(AuthorityOrderKind.OTHER));
	await fill("authority-new-due", "2026-10-20T12:00");
	await fill("authority-new-agency", " 新北市衛生局 ");
	await fill("authority-new-document-no", "北衛食字第 1150001 號");
	await fill("authority-new-summary", "請暫停該帳號");
	await fill("authority-target-0-account", "101");
	await fill("authority-target-0-products", "p1, p2");
	await click("登記");
	await until(() => created.length === 1, "送出");
	const request = plain(created[0]);
	assert.equal(request.agency, "新北市衛生局");
	assert.equal(request.kind, AuthorityOrderKind.OTHER);
	assert.equal(
		request.receiveTime.seconds,
		Date.parse("2026-10-19T09:00:00+08:00") / 1000,
	);
	assert.equal(
		request.dueTime.seconds,
		Date.parse("2026-10-20T12:00:00+08:00") / 1000,
	);
	assert.deepEqual(
		request.targets.map(
			(t: { accountId: string; storeId: string; productIds: string[] }) => [
				t.accountId,
				t.storeId,
				t.productIds,
			],
		),
		[["101", "", ["p1", "p2"]]],
	);
	assert.match(request.requestId, /^[0-9a-f-]{36}$/u);
	await until(
		() => navigation.pushed.includes(`/admin/authority-orders/${ORDER_ID}`),
		"導向詳情",
	);
});

test("登記頁：帳號 ID 輸入完（離開欄位）才讀店家；下拉含營運中與已關閉的店；選了店家請求就帶 store_id；換帳號清掉選擇", async () => {
	const listed: Req[] = [];
	const created: Req[] = [];
	await open(
		NewPage,
		transportWith(
			{
				createAuthorityOrder: (req) => {
					created.push(req);
					return { order: order() };
				},
			},
			{
				listStores: (req) => {
					listed.push(req);
					return req.status === 3
						? { stores: [{ store: { id: "56", storeName: "舊店（已關閉）" } }] }
						: { stores: [{ store: { id: "55", storeName: "阿里山農場" } }] };
				},
			},
		),
	);
	assert.equal(
		byId<HTMLSelectElement>("authority-target-0-store").disabled,
		true,
	);
	await fill("authority-target-0-account", "101");
	assert.equal(listed.length, 0, "還沒離開欄位不讀");
	await React.act(async () => {
		byId("authority-target-0-account").dispatchEvent(
			new dom.window.FocusEvent("focusout", { bubbles: true }),
		);
	});
	await until(() => listed.length === 2, "讀營運中與已關閉的店");
	assert.deepEqual(
		listed.map((r) => r.accountId),
		["101", "101"],
	);
	await until(
		() =>
			byId<HTMLSelectElement>("authority-target-0-store").options.length === 3,
		"下拉出現兩家店",
	);
	assert.deepEqual(
		[...byId<HTMLSelectElement>("authority-target-0-store").options].map(
			(o) => o.textContent,
		),
		["請選擇", "阿里山農場", "舊店（已關閉）"],
	);
	await fill("authority-target-0-store", "56");
	await fill("authority-new-kind", String(AuthorityOrderKind.OTHER));
	await click("登記");
	await until(() => created.length === 1, "送出");
	assert.equal(plain(created[0]).targets[0].storeId, "56");

	await fill("authority-target-0-account", "999");
	assert.equal(byId<HTMLSelectElement>("authority-target-0-store").value, "");
	assert.equal(
		byId<HTMLSelectElement>("authority-target-0-store").disabled,
		true,
	);
});

test("登記頁：可以新增與移除對象（至少一列）；伺服器的句子顯示在頁面上、不導向；失敗後重送沿用同一個 request_id", async () => {
	const created: Req[] = [];
	await open(
		NewPage,
		transportWith({
			createAuthorityOrder: (req) => {
				created.push(req);
				throw withMessage(Code.InvalidArgument, "指名的對象要有 1 到 20 個。");
			},
		}),
	);
	assert.ok(maybeById("authority-target-1-account") === null);
	await click("新增對象");
	assert.ok(maybeById("authority-target-1-account"));
	assert.equal(buttonByText("移除") !== undefined, true);
	await click("移除");
	assert.ok(maybeById("authority-target-1-account") === null);
	assert.equal(buttonByText("移除"), undefined, "只剩一列不能移除");
	await click("登記");
	await until(
		() => bodyText().includes("指名的對象要有 1 到 20 個。"),
		"伺服器句子",
	);
	await click("登記");
	await until(() => created.length === 2, "重送");
	assert.equal(created[0].requestId, created[1].requestId);
	assert.deepEqual(navigation.pushed, []);
});

// ---------------------------------------------------------------------------
// 詳情

const PARAMS = { id: ORDER_ID };

test("詳情：來函、指名的對象（逐家涵蓋狀態、四個標籤）、處理、保存、導出的措施；沒有內部註記", async () => {
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({
				order: order({
					targets: [
						{
							accountId: "101",
							storeId: "",
							productIds: ["p1", "p2"],
							coveredStores: [
								{
									storeId: "55",
									storeName: "阿里山農場",
									coverage: AuthorityStoreCoverage.HELD,
								},
								{
									storeId: "56",
									storeName: "",
									coverage: AuthorityStoreCoverage.PURGED_BEFORE_REGISTRATION,
								},
								{
									storeId: "57",
									storeName: "甲",
									coverage: AuthorityStoreCoverage.HOLD_ENDED,
								},
								{
									storeId: "58",
									storeName: "乙",
									coverage: AuthorityStoreCoverage.PURGED_AFTER_HOLD_ENDED,
								},
							],
						},
						{ accountId: "202", noSellerData: true },
					],
					holdExtensions: [
						{
							fromTime: ts(new Date("2027-04-21T02:00:00Z")),
							toTime: ts(new Date("2027-10-19T02:00:00Z")),
							cause: AuthorityHoldExtensionCause.ADMIN,
							reason: "進入訴訟",
							adminId: "9001",
							createTime: ts(T0),
						},
						{
							fromTime: ts(new Date("2027-10-19T02:00:00Z")),
							toTime: ts(new Date("2028-01-19T02:00:00Z")),
							cause: AuthorityHoldExtensionCause.DATA_PROVIDED,
							adminId: "9002",
							createTime: ts(T0),
						},
					],
				}),
			}),
			listMeasures: (req) => {
				assert.equal(req.sourceAuthorityOrderId, ORDER_ID);
				return { measures: [{ id: "m-1", kind: MeasureKind.SUSPENSION }] };
			},
		}),
		PARAMS,
	);
	const text = bodyText();
	for (const title of ["來函", "指名的對象", "處理", "保存", "導出的措施"]) {
		assert.ok(
			[...dom.window.document.querySelectorAll("h2")].some(
				(e) => e.textContent === title,
			),
			`區塊「${title}」`,
		);
	}
	assert.doesNotMatch(text, /內部註記/u);
	assert.match(text, /北衛食字第 1150001 號/u);
	assert.match(text, /請暫停該帳號/u);
	assert.match(text, /帳號 101・商品 p1、p2/u);
	assert.match(text, /阿里山農場・保存中/u);
	assert.match(text, /56・登記之前已依保留期限清除/u);
	assert.match(text, /甲・保存期限已過/u);
	assert.match(text, /乙・保存期限已過，個資已依保留期限清除/u);
	assert.match(
		text,
		/這個帳號在平台沒有店家資料；買家與帳號資料的保存尚未支援/u,
	);
	assert.match(text, /保存到 2027\/04\/21 10:00（保存中）/u);
	assert.match(
		text,
		/9001：保存到 2027\/04\/21 10:00 → 2027\/10\/19 10:00（進入訴訟）/u,
	);
	assert.match(text, /（自動延長：提供資料之後 6 個月）/u);
	assert.ok(
		dom.window.document.querySelector('a[href="/admin/merchants/55"]'),
		"店家連到商家頁",
	);
	await until(() => bodyText().includes("m-1"), "導出的措施");
});

test("詳情：按鈕只看 can_*；沒有 can_* 時只有建立措施的入口", async () => {
	await open(DetailPage, transportWith({}), PARAMS);
	assert.ok(buttonByText("完成") === undefined);
	assert.ok(buttonByText("延長保存") === undefined);
	assert.ok(buttonByText("建立措施"));
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({
				order: order({ canComplete: true, canExtendHold: true }),
			}),
		}),
		PARAMS,
	);
	assert.ok(buttonByText("完成"));
	assert.ok(buttonByText("延長保存"));
});

test("詳情：讀取失敗顯示失敗句與重試", async () => {
	let calls = 0;
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => {
				calls++;
				if (calls === 1) throw new ConnectError("x", Code.NotFound);
				return { order: order() };
			},
		}),
		PARAMS,
	);
	assert.ok(buttonByText("重試"));
	await click("重試");
	await until(() => bodyText().includes("北衛食字第 1150001 號"), "重試後");
});

test("完成來函：每個對象都要選告知方式；請求帶 etag、處理摘要與依位置的告知；調取資料帶提供資料的時間", async () => {
	const completed: Req[] = [];
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({
				order: order({
					kind: AuthorityOrderKind.DATA_REQUEST,
					canComplete: true,
					targets: [
						{
							accountId: "101",
							storeId: "55",
							storeName: "阿里山農場",
							coveredStores: [],
						},
						{ accountId: "102", storeId: "", coveredStores: [] },
					],
				}),
			}),
			completeAuthorityOrder: (req) => {
				completed.push(req);
				return { order: order() };
			},
		}),
		PARAMS,
	);
	await click("完成");
	assert.match(
		bodyText(),
		/寫下做了什麼、何時回覆機關。完成之後保存期限不變。/u,
	);
	assert.match(bodyText(), /保存期限會延到提供後 6 個月。/u);
	await fill("complete-authority-summary", " 已於 10/19 提供 ");
	await submit("完成");
	assert.match(bodyText(), /請為每個指名的對象選擇告知的方式。/u);
	assert.equal(completed.length, 0);

	await fill(
		"complete-authority-notice-0-kind",
		String(AuthorityTargetNoticeKind.NOTIFIED),
	);
	await fill("complete-authority-notice-0-time", "2026-10-19T10:00");
	await fill("complete-authority-notice-0-note", "公文");
	await fill(
		"complete-authority-notice-1-kind",
		String(AuthorityTargetNoticeKind.WITHHELD),
	);
	await fill("complete-authority-notice-1-recheck", "2026-11-19");
	await fill("complete-authority-provide-time", "2026-10-19T08:00");
	await submit("完成");
	await until(() => completed.length === 1, "送出");
	const request = plain(completed[0]);
	assert.equal(request.id, ORDER_ID);
	assert.equal(request.etag, "o-1");
	assert.equal(request.resolutionSummary, "已於 10/19 提供");
	assert.equal(
		request.provideTime.seconds,
		Date.parse("2026-10-19T08:00:00+08:00") / 1000,
	);
	assert.deepEqual(
		request.targetNotices.map(
			(n: { targetPosition: number; notice: { kind: number } }) => [
				n.targetPosition,
				n.notice.kind,
			],
		),
		[
			[0, AuthorityTargetNoticeKind.NOTIFIED],
			[1, AuthorityTargetNoticeKind.WITHHELD],
		],
	);
	assert.deepEqual(request.targetNotices[1].notice.recheckDate, {
		year: 2026,
		month: 11,
		day: 19,
	});
	await until(
		() => maybeById("complete-authority-summary") === null,
		"成功後關閉",
	);
});

test("完成來函：其他種類沒有提供資料的時間欄位", async () => {
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({ order: order({ canComplete: true }) }),
		}),
		PARAMS,
	);
	await click("完成");
	assert.ok(maybeById("complete-authority-provide-time") === null);
});

test("版本衝突：重讀來函並顯示衝突句，對話框留著、輸入還在", async () => {
	let reads = 0;
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => {
				reads++;
				return { order: order({ canExtendHold: true, etag: `o-${reads}` }) };
			},
			extendAuthorityOrderHold: () => {
				throw versionConflict();
			},
		}),
		PARAMS,
	);
	await click("延長保存");
	await fill("extend-hold-until", "2027-10-19T00:00");
	await fill("extend-hold-reason", "進入訴訟");
	const before = reads;
	await submit("延長保存");
	await until(
		() =>
			bodyText().includes(
				"這筆來函剛被更新，頁面已重新載入；請確認最新狀態後再操作。",
			),
		"衝突句",
	);
	assert.ok(reads > before, "重讀了來函");
	assert.equal(
		byId<HTMLTextAreaElement>("extend-hold-reason").value,
		"進入訴訟",
	);
});

test("延長保存：請求帶新的保存期限（台北）、原因與 etag；伺服器的句子顯示在對話框", async () => {
	const extended: Req[] = [];
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({ order: order({ canExtendHold: true }) }),
			extendAuthorityOrderHold: (req) => {
				extended.push(req);
				throw withMessage(
					Code.InvalidArgument,
					"新的保存期限必須晚於目前的保存期限。",
				);
			},
		}),
		PARAMS,
	);
	await click("延長保存");
	assert.match(
		bodyText(),
		/只能延長。例如進入訴訟時，延長到裁判確定後 3\s*個月。每次延長會留下前後的期限與原因。/u,
	);
	await fill("extend-hold-until", "2027-10-19T00:00");
	await fill("extend-hold-reason", " 進入訴訟 ");
	await submit("延長保存");
	await until(() => bodyText().includes("新的保存期限必須晚於"), "伺服器句子");
	const request = plain(extended[0]);
	assert.equal(request.etag, "o-1");
	assert.equal(request.reason, "進入訴訟");
	assert.equal(
		request.holdUntil.seconds,
		Date.parse("2027-10-19T00:00:00+08:00") / 1000,
	);
	assert.ok(maybeById("extend-hold-until"), "對話框還在");
});

test("暫不告知的對象：顯示確認日期與後續說明；記下告知改成已告知或寫新的確認日期，請求帶位置與 etag", async () => {
	const recorded: Req[] = [];
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({
				order: order({
					state: AuthorityOrderState.COMPLETED,
					completeTime: ts(T0),
					completedByAdminId: "9001",
					resolutionSummary: "已回覆機關",
					targets: [
						{
							accountId: "101",
							storeId: "55",
							storeName: "阿里山農場",
							coveredStores: [],
							canRecordNotice: true,
							notice: {
								kind: AuthorityTargetNoticeKind.WITHHELD,
								recheckDate: { year: 2026, month: 11, day: 19 },
							},
						},
						{
							// 暫不告知，但伺服器說現在不能記（例如來函已撤銷）：不顯示按鈕，也不自己從告知方式推。
							accountId: "103",
							storeId: "",
							coveredStores: [],
							canRecordNotice: false,
							notice: {
								kind: AuthorityTargetNoticeKind.WITHHELD,
								recheckDate: { year: 2026, month: 11, day: 20 },
							},
						},
						{
							accountId: "102",
							storeId: "",
							coveredStores: [],
							notice: {
								kind: AuthorityTargetNoticeKind.NOTIFIED,
								notifyTime: ts(T0),
								note: "公文",
							},
						},
					],
				}),
			}),
			recordAuthorityTargetNotice: (req) => {
				recorded.push(req);
				return { order: order() };
			},
		}),
		PARAMS,
	);
	assert.match(bodyText(), /機關要求暫不告知（2026\/11\/19）/u);
	assert.match(bodyText(), /已告知（2026\/10\/18 10:00・公文）/u);
	assert.match(
		bodyText(),
		/機關要求暫不告知；到了確認日期請向機關確認能否告知。/u,
	);
	assert.equal(
		[...dom.window.document.querySelectorAll("button")].filter(
			(b) => b.textContent === "記下告知",
		).length,
		1,
		"只有伺服器說可以記（can_record_notice）的對象有按鈕，暫不告知但不能記的沒有",
	);
	await click("記下告知");
	assert.deepEqual(
		[...byId<HTMLSelectElement>("record-authority-notice-kind").options].map(
			(o) => o.textContent,
		),
		["請選擇", "已告知", "機關要求暫不告知"],
		"由措施告知不能在後續選",
	);
	await fill(
		"record-authority-notice-kind",
		String(AuthorityTargetNoticeKind.WITHHELD),
	);
	await fill("record-authority-notice-recheck", "2026-12-01");
	await submit("記下告知");
	await until(() => recorded.length === 1, "送出");
	const request = plain(recorded[0]);
	assert.equal(request.targetPosition, 0);
	assert.equal(request.etag, "o-1");
	assert.equal(request.notice.kind, AuthorityTargetNoticeKind.WITHHELD);
	assert.deepEqual(request.notice.recheckDate, {
		year: 2026,
		month: 12,
		day: 1,
	});
});

const TERMINATION_STORE = {
	store: { id: "55", accountId: "101" },
	canCreateTermination: true,
	availableTerminations: [
		{
			ground: TerminationGround.NOTICE,
			explanation: { locale: "zh-TW", message: "三十天後生效。" },
		},
	],
};

test("從來函建立措施（違規依據）：頂端說明來函文號，請求帶 source_authority_order_id，新認定另帶來源來函", async () => {
	const created: Req[] = [];
	await open(
		DetailPage,
		transportWith(
			{
				createMeasure: (req) => {
					created.push(req);
					return { measure: { id: "m-new" } };
				},
			},
			{ getStore: () => TERMINATION_STORE },
		),
		PARAMS,
	);
	await click("建立措施");
	assert.match(
		bodyText(),
		/這個措施會記為主管機關來函 北衛食字第 1150001 號 導出的措施。/u,
	);
	await until(() => !bodyText().includes("載入通報中..."), "來源通報清單讀完");
	await fill("create-violation-measure-kind", "warning");
	await fill("create-violation-measure-category", String(PolicyCategory.FRAUD));
	await fill(
		"create-violation-measure-severity",
		String(ViolationSeverity.HIGH),
	);
	await fill("create-violation-measure-statement", "依來函辦理");
	await fill("create-violation-measure-note", "公文");
	await click("確認建立");
	await until(() => created.length === 1, "送出");
	const request = plain(created[0]);
	assert.equal(request.sourceAuthorityOrderId, ORDER_ID);
	assert.equal(request.storeId, "55");
	assert.ok(!request.sourceCaseId, "不是從案件建立的");
	assert.deepEqual(
		request.measure.value.violation.finding.sourceAuthorityOrderIds,
		[ORDER_ID],
	);
	await until(() => bodyText().includes("已建立措施。"), "成功提示");
});

test("從來函建立風險措施與終止合作：頂端說明來函文號，請求都帶 source_authority_order_id", async () => {
	const created: Req[] = [];
	await open(
		DetailPage,
		transportWith(
			{
				createMeasure: (req) => {
					created.push(req);
					return { measure: { id: "m-new" } };
				},
			},
			{ getStore: () => TERMINATION_STORE },
		),
		PARAMS,
	);
	await click("建立風險措施");
	assert.match(
		bodyText(),
		/這個措施會記為主管機關來函 北衛食字第 1150001 號 導出的措施。/u,
	);
	await fill("create-measure-ground", String(RiskGround.FOOD_SAFETY));
	await fill("create-measure-statement", "依來函辦理");
	await fill("create-measure-note", "公文");
	await click("確認建立");
	await until(() => created.length === 1, "風險措施送出");
	assert.equal(plain(created[0]).sourceAuthorityOrderId, ORDER_ID);
	assert.equal(plain(created[0]).measure.case, "suspension");

	await until(
		() => buttonByText("終止合作")?.disabled === false,
		"店家讀完，可以終止",
	);
	await click("終止合作");
	assert.match(
		bodyText(),
		/這個措施會記為主管機關來函 北衛食字第 1150001 號 導出的措施。/u,
	);
	await fill("create-termination-ground", String(TerminationGround.NOTICE));
	await fill("create-termination-statement", "屢次違規");
	await fill("create-termination-note", "公文");
	await click("確認建立");
	await until(() => created.length === 2, "終止合作送出");
	assert.equal(plain(created[1]).sourceAuthorityOrderId, ORDER_ID);
	assert.equal(plain(created[1]).measure.case, "termination");
});

const coveredStore = (
	storeId: string,
	storeName: string,
	flags: { canCreateMeasure?: boolean; canCreateTermination?: boolean },
	coverage = AuthorityStoreCoverage.HELD,
) => ({ storeId, storeName, coverage, ...flags });

test("多家可建立措施的店：沒有預設，先選（選項「請選擇」開頭，只列旗標為真的店）；選之前建立措施停用；終止合作只列 can_create_termination 的店", async () => {
	const stores: string[] = [];
	const created: Req[] = [];
	await open(
		DetailPage,
		transportWith(
			{
				getAuthorityOrder: () => ({
					order: order({
						targets: [
							{
								accountId: "101",
								storeId: "",
								coveredStores: [
									coveredStore("55", "阿里山農場", {
										canCreateMeasure: true,
										canCreateTermination: false,
									}),
									coveredStore("56", "舊店（已關閉）", {}),
									coveredStore(
										"57",
										"",
										{},
										AuthorityStoreCoverage.PURGED_AFTER_HOLD_ENDED,
									),
								],
							},
							{
								accountId: "102",
								storeId: "",
								coveredStores: [
									coveredStore("60", "玉山果園", {
										canCreateMeasure: true,
										canCreateTermination: false,
									}),
								],
							},
						],
					}),
				}),
				createMeasure: (req) => {
					created.push(req);
					return { measure: { id: "m-new" } };
				},
			},
			{
				getStore: (req) => {
					stores.push(req.id as string);
					return {
						store: { id: req.id, accountId: "x" },
						canCreateTermination: false,
					};
				},
			},
		),
		PARAMS,
	);
	const select = byId<HTMLSelectElement>("authority-measure-store");
	assert.deepEqual(
		[...select.options].map((o) => o.textContent),
		["請選擇", "101・阿里山農場", "102・玉山果園"],
		"已關閉與已清除的店不在選項裡",
	);
	assert.equal(select.value, "", "沒有預設");
	assert.match(bodyText(), /先選帳號/u);
	assert.equal(buttonByText("建立措施")?.disabled, true);
	assert.equal(buttonByText("建立風險措施")?.disabled, true);
	assert.ok(
		buttonByText("終止合作") === undefined,
		"沒有任何店 can_create_termination",
	);
	assert.deepEqual(stores, [], "沒選之前不讀店家");
	await fill("authority-measure-store", "60");
	assert.equal(buttonByText("建立風險措施")?.disabled, false);
	await click("建立風險措施");
	await fill("create-measure-ground", String(RiskGround.FOOD_SAFETY));
	await fill("create-measure-statement", "x");
	await fill("create-measure-note", "y");
	await click("確認建立");
	await until(() => created.length === 1, "送出");
	assert.equal(plain(created[0]).storeId, "60");
});

test("只有一家可建立措施的店（其他已關閉或已清除）就直接用它，不出現選單", async () => {
	const created: Req[] = [];
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({
				order: order({
					targets: [
						{
							accountId: "101",
							storeId: "",
							coveredStores: [
								coveredStore("56", "舊店（已關閉）", {}),
								coveredStore(
									"57",
									"",
									{},
									AuthorityStoreCoverage.PURGED_BEFORE_REGISTRATION,
								),
								coveredStore("55", "阿里山農場", { canCreateMeasure: true }),
							],
						},
					],
				}),
			}),
			createMeasure: (req) => {
				created.push(req);
				return { measure: { id: "m-new" } };
			},
		}),
		PARAMS,
	);
	assert.ok(maybeById("authority-measure-store") === null, "只有一家不需要選");
	assert.equal(buttonByText("建立風險措施")?.disabled, false);
	assert.ok(buttonByText("終止合作") === undefined);
	await click("建立風險措施");
	await fill("create-measure-ground", String(RiskGround.FOOD_SAFETY));
	await fill("create-measure-statement", "x");
	await fill("create-measure-note", "y");
	await click("確認建立");
	await until(() => created.length === 1, "送出");
	assert.equal(plain(created[0]).storeId, "55");
});

test("終止合作的選項只列 can_create_termination 的店；選到的店才讀 GetStore", async () => {
	const stores: string[] = [];
	await open(
		DetailPage,
		transportWith(
			{
				getAuthorityOrder: () => ({
					order: order({
						targets: [
							{
								accountId: "101",
								storeId: "",
								coveredStores: [
									coveredStore("55", "甲", { canCreateMeasure: true }),
									coveredStore("56", "乙", {
										canCreateMeasure: true,
										canCreateTermination: true,
									}),
									coveredStore("57", "丙", { canCreateTermination: true }),
								],
							},
						],
					}),
				}),
			},
			{
				getStore: (req) => {
					stores.push(req.id as string);
					return {
						...TERMINATION_STORE,
						store: { id: req.id, accountId: "101" },
					};
				},
			},
		),
		PARAMS,
	);
	assert.deepEqual(
		[...byId<HTMLSelectElement>("authority-measure-store").options].map(
			(o) => o.textContent,
		),
		["請選擇", "101・甲", "101・乙", "101・丙"],
	);
	assert.equal(buttonByText("終止合作")?.disabled, true, "還沒選");
	await fill("authority-measure-store", "55");
	assert.equal(buttonByText("終止合作")?.disabled, true, "甲不能終止");
	assert.equal(buttonByText("建立措施")?.disabled, false);
	await fill("authority-measure-store", "57");
	assert.equal(buttonByText("建立措施")?.disabled, true, "丙不能建立措施");
	await until(() => buttonByText("終止合作")?.disabled === false, "丙可以終止");
	assert.deepEqual(stores, ["57"], "只讀選到的店");
});

test("來函已撤銷（旗標全假）或店家都不能建立：沒有三個建立措施的入口", async () => {
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({
				order: order({
					state: AuthorityOrderState.REVOKED,
					targets: [
						{
							accountId: "101",
							storeId: "",
							coveredStores: [coveredStore("55", "阿里山農場", {})],
						},
					],
				}),
			}),
		}),
		PARAMS,
	);
	assert.ok(buttonByText("建立措施") === undefined);
	assert.ok(buttonByText("建立風險措施") === undefined);
	assert.ok(buttonByText("終止合作") === undefined);
	assert.ok(maybeById("authority-measure-store") === null);
});

test("沒有店家的來函（只有沒有店家資料的帳號）沒有建立措施的入口", async () => {
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({
				order: order({ targets: [{ accountId: "202", noSellerData: true }] }),
			}),
		}),
		PARAMS,
	);
	assert.ok(buttonByText("建立措施") === undefined);
	assert.ok(buttonByText("建立風險措施") === undefined);
});

// ---------------------------------------------------------------------------
// 3c-b 修正輪

test("清單：已撤銷登記分頁請求 REVOKED，欄位是撤銷時間（沒有保存到）", async () => {
	const requests: Req[] = [];
	await open(
		ListPage,
		transportWith({
			listAuthorityOrders: (req) => {
				requests.push(req);
				return req.state === AuthorityOrderState.REVOKED
					? {
							orders: [
								order({
									state: AuthorityOrderState.REVOKED,
									revokeTime: ts(T0),
									revokeReason: "登記錯誤",
								}),
							],
						}
					: { orders: [] };
			},
		}),
	);
	await click("已撤銷登記");
	await until(() => requests.length === 2, "換分頁後重讀");
	assert.equal(requests[1].state, AuthorityOrderState.REVOKED);
	assert.deepEqual(headers(), [
		"機關",
		"文號",
		"種類",
		"指名的店家",
		"撤銷時間",
	]);
	await until(() => rows().length === 1 && rows()[0].length === 5, "列出");
	assert.equal(rows()[0][4], "2026/10/18 10:00");
});

test("詳情：撤銷登記按鈕只看 can_revoke", async () => {
	await open(DetailPage, transportWith({}), PARAMS);
	assert.ok(buttonByText("撤銷登記") === undefined);
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({ order: order({ canRevoke: true }) }),
		}),
		PARAMS,
	);
	assert.ok(buttonByText("撤銷登記"));
});

test("撤銷登記：對話框說明與欄位，請求帶 id、去掉前後空白的原因與 etag；伺服器的句子顯示在對話框", async () => {
	const revoked: Req[] = [];
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({ order: order({ canRevoke: true }) }),
			revokeAuthorityOrder: (req) => {
				revoked.push(req);
				throw withMessage(Code.InvalidArgument, "撤銷的原因要 1 到 2000 字。");
			},
		}),
		PARAMS,
	);
	await click("撤銷登記");
	assert.match(
		bodyText(),
		/只在登記錯誤時撤銷（例如對象或文號登記錯了）。撤銷之後這筆來函的保存解除，不再是處理中，也不能再當作措施的來源；紀錄會保留。正確的來函請另外登記。/u,
	);
	assert.match(
		dom.window.document.querySelector("label[for=revoke-authority-reason]")
			?.textContent ?? "",
		/^原因/u,
	);
	assert.ok(footerButton("取消"));
	await fill("revoke-authority-reason", "  對象登記錯了 ");
	await submit("撤銷登記");
	await until(
		() => bodyText().includes("撤銷的原因要 1 到 2000 字。"),
		"伺服器句子",
	);
	assert.deepEqual(plain(revoked[0]), {
		id: ORDER_ID,
		reason: "對象登記錯了",
		etag: "o-1",
	});
	assert.ok(maybeById("revoke-authority-reason"), "對話框還在");
});

test("撤銷登記：成功關閉對話框；先前已撤銷時顯示橫幅；版本衝突重讀並顯示衝突句", async () => {
	let mode: "ok" | "already" | "conflict" = "already";
	let reads = 0;
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => {
				reads++;
				return { order: order({ canRevoke: true }) };
			},
			revokeAuthorityOrder: () => {
				if (mode === "conflict") throw versionConflict();
				return {
					order: order({ state: AuthorityOrderState.REVOKED }),
					alreadyRevoked: mode === "already",
				};
			},
		}),
		PARAMS,
	);
	await click("撤銷登記");
	await fill("revoke-authority-reason", "登記錯誤");
	await submit("撤銷登記");
	await until(
		() => maybeById("revoke-authority-reason") === null,
		"對話框關閉",
	);
	assert.match(bodyText(), /這筆來函先前已撤銷登記。/u);

	mode = "conflict";
	const before = reads;
	await click("撤銷登記");
	await fill("revoke-authority-reason", "登記錯誤");
	await submit("撤銷登記");
	await until(
		() =>
			bodyText().includes(
				"這筆來函剛被更新，頁面已重新載入；請確認最新狀態後再操作。",
			),
		"衝突句",
	);
	assert.ok(reads > before, "重讀了來函");
	assert.equal(
		byId<HTMLTextAreaElement>("revoke-authority-reason").value,
		"登記錯誤",
	);
});

test("已撤銷的來函：狀態「已撤銷登記」，處理區塊「已於 {時間} 撤銷登記：{原因}」，沒有完成、延長、撤銷、記下告知", async () => {
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({
				order: order({
					state: AuthorityOrderState.REVOKED,
					revokeTime: ts(T0),
					revokedByAdminId: "9001",
					revokeReason: "對象登記錯了",
				}),
			}),
		}),
		PARAMS,
	);
	assert.match(bodyText(), /已撤銷登記/u);
	assert.match(bodyText(), /已於 2026\/10\/18 10:00 撤銷登記：對象登記錯了/u);
	for (const text of ["完成", "延長保存", "撤銷登記", "記下告知"]) {
		assert.ok(buttonByText(text) === undefined, text);
	}
});

test("完成來函：由措施告知的限制說明是核准的那一句", async () => {
	await open(
		DetailPage,
		transportWith({
			getAuthorityOrder: () => ({ order: order({ canComplete: true }) }),
		}),
		PARAMS,
	);
	await click("完成");
	await fill(
		"complete-authority-notice-0-kind",
		String(AuthorityTargetNoticeKind.MEASURE_IS_NOTICE),
	);
	assert.match(
		bodyText(),
		/只限暫停帳號或限制商品，而且已經從這份來函建立措施。/u,
	);
});

test("登記頁：新增對象不受客戶端上限限制（超過 20 列仍可新增，由伺服器擋）；每列的移除按鈕指向該列的帳號 ID 欄", async () => {
	await open(NewPage, transportWith({}));
	for (let i = 0; i < 21; i++) await click("新增對象");
	assert.ok(maybeById("authority-target-21-account"), "有第 22 列");
	assert.ok(buttonByText("新增對象"), "新增對象還在");
	const removes = [...dom.window.document.querySelectorAll("button")].filter(
		(b) => b.textContent === "移除",
	);
	assert.equal(removes.length, 22);
	removes.forEach((button, index) => {
		assert.equal(
			button.getAttribute("aria-describedby"),
			`authority-target-${index}-account`,
		);
		assert.ok(maybeById(`authority-target-${index}-account`));
	});
});

test("登記頁：店家下拉把所有頁讀完（兩頁都在選項裡）", async () => {
	const listed: Req[] = [];
	await open(
		NewPage,
		transportWith(
			{},
			{
				listStores: (req) => {
					listed.push(req);
					if (req.status === 3) return { stores: [] };
					return req.pageToken
						? { stores: [{ store: { id: "56", storeName: "第二頁的店" } }] }
						: {
								stores: [{ store: { id: "55", storeName: "第一頁的店" } }],
								nextPageToken: "p2",
							};
				},
			},
		),
	);
	await fill("authority-target-0-account", "101");
	await React.act(async () => {
		byId("authority-target-0-account").dispatchEvent(
			new dom.window.FocusEvent("focusout", { bubbles: true }),
		);
	});
	await until(
		() =>
			byId<HTMLSelectElement>("authority-target-0-store").options.length === 3,
		"兩頁的店都在下拉裡",
	);
	assert.deepEqual(
		[...byId<HTMLSelectElement>("authority-target-0-store").options].map(
			(o) => o.textContent,
		),
		["請選擇", "第一頁的店", "第二頁的店"],
	);
	assert.deepEqual(
		listed.filter((r) => r.status !== 3).map((r) => r.pageToken),
		["", "p2"],
	);
});
