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
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	ContactKind,
	CreateProspectRequestSchema,
	OutreachStatus,
	ProspectMatchReason,
	ProspectMatchTier,
	ProspectSchema,
	ProspectStatus,
	RestoreProspectRequestSchema,
	SellerProspectAdminService,
	UpdateProspectRequestSchema,
	VendorType,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import {
	bodyText,
	buttonByText,
	byId,
	click,
	clickElement,
	dom,
	mount,
	navigation,
	React,
	requireFromTest,
	setSearch,
	setValue,
	until,
} from "./fixtures/dom-harness";
import { versionConflict } from "./fixtures/report-fixtures";

/**
 * 站外業者名單的清單、建立、詳情頁實際掛進 DOM 後的行為（router transport 回真的 protobuf 訊息）：
 * 撞到相符業者的確認流程、巢狀對話框不重複送出、修改的版本衝突、還原、清單的翻頁與篩選。
 */

const h = React.createElement;
const doc = () => dom.window.document;

const { outreachStatusText } = requireFromTest(
	"../src/lib/seller-prospects",
) as { outreachStatusText: (value: OutreachStatus) => string };
const Create = requireFromTest(
	"../src/app/admin/seller-prospects/CreateProspectDialog",
).default as unknown;
const ListPage = requireFromTest("../src/app/admin/seller-prospects/page")
	.default as unknown;
const DetailPage = requireFromTest(
	"../src/app/admin/seller-prospects/[id]/page",
).default as unknown;
const { ToastProvider } = requireFromTest("../src/components/UI/Toast") as {
	ToastProvider: unknown;
};

type Init = Parameters<typeof create<typeof ProspectSchema>>[1];
function prospect(init: Init = {}) {
	return create(ProspectSchema, {
		id: "7",
		etag: "e1",
		displayName: "阿里山農場",
		vendorType: VendorType.BUSINESS,
		status: ProspectStatus.ACTIVE,
		outreachStatus: OutreachStatus.NOT_CONTACTED,
		region: "嘉義",
		contacts: [{ kind: ContactKind.EMAIL, value: "a@x.test", primary: true }],
		...init,
	});
}

function matchFound(ids: string): ConnectError {
	return new ConnectError("match", Code.FailedPrecondition, undefined, [
		{
			desc: ErrorInfoSchema,
			value: {
				reason: "PROSPECT_MATCH_FOUND",
				domain: "hawkr",
				metadata: { prospectIds: ids },
			},
		},
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message: "找到相符的業者" },
		},
	]);
}

type Handlers = Partial<
	Record<
		keyof typeof SellerProspectAdminService.method,
		(req: never) => unknown
	>
>;
function transportWith(handlers: Handlers): Transport {
	return createRouterTransport(({ service }) => {
		service(SellerProspectAdminService, handlers as never);
	});
}

function open(element: unknown, transport: Transport) {
	return mount(h(ToastProvider, null, element), transport);
}

const contactInput = () =>
	doc().querySelector('input[aria-label="電子郵件"]') as HTMLInputElement;

async function type(el: Element, value: string) {
	await React.act(async () => {
		setValue(el, value);
	});
}

async function fillCreateForm() {
	await type(byId("prospect-display-name"), "阿里山農場");
	await click("新增聯絡方式");
	await type(contactInput(), "a@x.test");
}

const checkboxes = () =>
	[...doc().querySelectorAll('input[type="checkbox"]')] as HTMLInputElement[];
const rowNames = () =>
	[...doc().querySelectorAll("li a")].map((a) => a.textContent);

function previewReplying(
	counter: { n: number },
	matches: { id: string; name: string; reasons: ProspectMatchReason[] }[],
) {
	return () => {
		counter.n += 1;
		return {
			matches: matches.map((m) => ({
				prospect: prospect({ id: m.id, displayName: m.name, contacts: [] }),
				tier: ProspectMatchTier.T2_CONTACT,
				matchedOn: m.reasons,
			})),
		};
	};
}

test("建立撞到相符業者：開對話框、Preview 一次、列出命中原因、Preview 沒回的以 GetProspect 補上且不顯示原因、兩筆都勾才能送、重送帶兩筆", async () => {
	const previews = { n: 0 };
	const gets: string[] = [];
	const creates: { ids: string[]; contactCount: number }[] = [];
	const transport = transportWith({
		previewProspect: previewReplying(previews, [
			{
				id: "11",
				name: "甲農場",
				reasons: [ProspectMatchReason.EMAIL, ProspectMatchReason.NAME],
			},
		]),
		getProspect: (req: { id: string }) => {
			gets.push(req.id);
			return {
				prospect: prospect({
					id: req.id,
					displayName: req.id === "11" ? "甲農場" : "乙農場",
					contacts: [
						{
							kind: ContactKind.EMAIL,
							value: `${req.id}@x.test`,
							primary: true,
						},
					],
				}),
			};
		},
		createProspect: (req: unknown) => {
			const request = create(CreateProspectRequestSchema, req as never);
			creates.push({
				ids: [...request.confirmedDistinctProspectIds],
				contactCount: request.prospect?.contacts.length ?? 0,
			});
			if (creates.length === 1) throw matchFound("11,12");
			return { prospect: prospect({ id: "99" }) };
		},
	});
	const m = open(h(Create, { onClose: () => {} }), transport);
	await m.render({ settle: false });
	await fillCreateForm();
	await until(() => previews.n === 1, "即時查重送出一次");
	await click("建立");
	await until(() => bodyText().includes("可能是同一家業者"), "相符業者對話框");
	await until(() => rowNames().length === 2, "兩筆相符業者");

	assert.equal(previews.n, 2, "對話框只多打一次 Preview");
	assert.deepEqual(
		[...gets].sort(),
		["11", "12"],
		"相符業者每一筆都用 GetProspect 取資料",
	);
	assert.deepEqual(rowNames(), ["甲農場", "乙農場"]);
	const items = [...doc().querySelectorAll("li")];
	assert.match(items[0].textContent ?? "", /電子郵件相同、名稱相同/u);
	assert.match(
		items[0].textContent ?? "",
		/電子郵件 · 11@x\.test/u,
		"Preview 有回的列也顯示 GetProspect 的主要聯絡方式",
	);
	assert.match(items[1].textContent ?? "", /電子郵件 · 12@x\.test/u);
	assert.match(items[0].textContent ?? "", /進行中/u);
	assert.doesNotMatch(items[1].textContent ?? "", /相同/u);
	assert.match(bodyText(), /下列業者的聯絡方式與這次填寫的相同/u);

	const confirm = () => buttonByText("確認不是同一家並建立");
	assert.ok(confirm()?.disabled, "沒勾選時不能送出");
	await clickElement(checkboxes()[0]);
	assert.ok(confirm()?.disabled, "只勾一筆還不能送出");
	await clickElement(checkboxes()[1]);
	assert.equal(confirm()?.disabled, false);
	await click("確認不是同一家並建立");
	await until(() => creates.length === 2, "重送");

	assert.deepEqual(creates[0].ids, []);
	assert.deepEqual(creates[1].ids, ["11", "12"], "帶全部相符業者的 ID");
	assert.equal(creates[1].contactCount, 1);
	await until(() => navigation.pushed.length === 1, "導到新業者");
	assert.deepEqual(navigation.pushed, ["/admin/seller-prospects/99"]);
});

test("巢狀送出：在對話框按確認，外層建立表單的 onSubmit 只因重送被呼叫一次，不因冒泡多一次", async () => {
	let creates = 0;
	const transport = transportWith({
		previewProspect: () => ({
			matches: [
				{
					prospect: prospect({ id: "11", displayName: "甲農場" }),
					tier: ProspectMatchTier.T2_CONTACT,
					matchedOn: [ProspectMatchReason.EMAIL],
				},
			],
		}),
		getProspect: (req: { id: string }) => ({
			prospect: prospect({ id: req.id, displayName: "甲農場" }),
		}),
		createProspect: () => {
			creates += 1;
			if (creates === 1) throw matchFound("11");
			return { prospect: prospect({ id: "99" }) };
		},
	});
	const m = open(h(Create, { onClose: () => {} }), transport);
	await m.render({ settle: false });
	await fillCreateForm();
	await click("建立");
	await until(() => rowNames().length === 1, "相符業者對話框");
	await clickElement(checkboxes()[0]);
	await click("確認不是同一家並建立");
	await until(() => navigation.pushed.length === 1, "重送成功");
	await React.act(async () => {
		await new Promise((resolve) => setTimeout(resolve, 20));
	});
	assert.equal(creates, 2, "第一次送出加一次重送，沒有第三次");
});

test("重送又撞到第三筆：對話框以三筆重開，前兩筆保持勾選", async () => {
	const ids: string[][] = [];
	const transport = transportWith({
		previewProspect: () => ({ matches: [] }),
		getProspect: (req: { id: string }) => ({
			prospect: prospect({ id: req.id, displayName: `業者${req.id}` }),
		}),
		createProspect: (req: unknown) => {
			const request = create(CreateProspectRequestSchema, req as never);
			ids.push([...request.confirmedDistinctProspectIds]);
			if (ids.length === 1) throw matchFound("11,12");
			if (ids.length === 2) throw matchFound("11,12,13");
			return { prospect: prospect({ id: "99" }) };
		},
	});
	const m = open(h(Create, { onClose: () => {} }), transport);
	await m.render({ settle: false });
	await fillCreateForm();
	await click("建立");
	await until(() => rowNames().length === 2, "兩筆");
	await clickElement(checkboxes()[0]);
	await clickElement(checkboxes()[1]);
	await click("確認不是同一家並建立");
	await until(() => rowNames().length === 3, "以三筆重開");

	assert.deepEqual(rowNames(), ["業者11", "業者12", "業者13"]);
	assert.deepEqual(
		checkboxes().map((c) => c.checked),
		[true, true, false],
		"已勾過而且仍在的保留勾選",
	);
	assert.ok(buttonByText("確認不是同一家並建立")?.disabled);
	await clickElement(checkboxes()[2]);
	await click("確認不是同一家並建立");
	await until(() => ids.length === 3, "第三次送出");
	assert.deepEqual(ids[2], ["11", "12", "13"]);
});

function resendFailing(resend: (n: number) => unknown): Transport {
	let creates = 0;
	return transportWith({
		previewProspect: () => ({ matches: [] }),
		getProspect: (req: { id: string }) => ({
			prospect: prospect({ id: req.id, displayName: `業者${req.id}` }),
		}),
		createProspect: () => {
			creates += 1;
			if (creates === 1) throw matchFound("11");
			return resend(creates);
		},
	});
}

async function resendFromDialog(transport: Transport) {
	const m = open(h(Create, { onClose: () => {} }), transport);
	await m.render({ settle: false });
	await fillCreateForm();
	await click("建立");
	await until(() => rowNames().length === 1, "相符業者對話框");
	await clickElement(checkboxes()[0]);
	await click("確認不是同一家並建立");
}

test("確認框重送失敗且帶 reason（非 PROSPECT_MATCH_FOUND）：關閉對話框並交給 onFailure", async () => {
	await resendFromDialog(
		resendFailing(() => {
			throw new ConnectError("held", Code.FailedPrecondition, undefined, [
				{
					desc: ErrorInfoSchema,
					value: {
						reason: "PROSPECT_TAX_ID_HELD",
						domain: "hawkr",
						metadata: { prospectIds: "5" },
					},
				},
				{
					desc: LocalizedMessageSchema,
					value: { locale: "zh-TW", message: "統一編號已由業者 5 使用" },
				},
			]);
		}),
	);
	await until(
		() => buttonByText("確認不是同一家並建立") === undefined,
		"對話框關閉",
	);
	await until(
		() => bodyText().includes("統一編號已由業者 5 使用"),
		"onFailure",
	);
});

test("確認框重送失敗但沒有 reason：留在框內顯示伺服器文字", async () => {
	await resendFromDialog(
		resendFailing(() => {
			throw new ConnectError("busy", Code.Internal, undefined, [
				{
					desc: LocalizedMessageSchema,
					value: { locale: "zh-TW", message: "伺服器忙碌，請稍後再試" },
				},
			]);
		}),
	);
	await until(() => bodyText().includes("伺服器忙碌，請稍後再試"), "框內文字");
	assert.ok(buttonByText("確認不是同一家並建立"), "對話框仍開著");
	assert.equal(rowNames().length, 1);
});

test("確認框重送沒收到回應：留在框內", async () => {
	let creates = 0;
	const inner = resendFailing(() => ({ prospect: prospect({ id: "99" }) }));
	const transport: Transport = {
		...inner,
		unary: ((...args: Parameters<Transport["unary"]>) => {
			if (args[0].name === "CreateProspect") {
				creates += 1;
				if (creates === 2)
					return Promise.reject(
						ConnectError.from(new TypeError("Failed to fetch")),
					);
			}
			return inner.unary(...args);
		}) as Transport["unary"],
		stream: inner.stream.bind(inner),
	};
	await resendFromDialog(transport);
	await until(() => creates === 2, "重送已送出");
	await until(
		() => buttonByText("確認不是同一家並建立")?.disabled === false,
		"送出結束",
	);
	assert.equal(rowNames().length, 1, "對話框仍開著");
	assert.deepEqual(navigation.pushed, []);
});

test("相符業者讀取失敗：顯示讀取失敗與重試，不能送出", async () => {
	let previews = 0;
	const transport = transportWith({
		previewProspect: () => {
			previews += 1;
			if (previews === 1) throw new ConnectError("boom", Code.Internal);
			return { matches: [] };
		},
		getProspect: (req: { id: string }) => ({
			prospect: prospect({ id: req.id, displayName: "丙農場" }),
		}),
		createProspect: () => {
			throw matchFound("11");
		},
	});
	const m = open(h(Create, { onClose: () => {} }), transport);
	await m.render({ settle: false });
	await type(byId("prospect-display-name"), "阿里山農場");
	await click("建立");
	await until(() => bodyText().includes("無法讀取相符的業者。"), "讀取失敗");
	assert.ok(buttonByText("確認不是同一家並建立")?.disabled);
	await click("重試");
	await until(() => rowNames().length === 1, "重試後列出");
});

async function openDetail(transport: Transport) {
	navigation.params = { id: "7" };
	const m = open(h(DetailPage), transport);
	await m.render();
	return m;
}

test("修改衝突：aborted 之後重讀，草稿還在、沒碰的欄位換成新版本、顯示衝突句；再送出只帶自己改的欄位與新 etag", async () => {
	let version = 1;
	const updates: { etag: string; paths: string[]; name: string }[] = [];
	const transport = transportWith({
		getProspect: () => ({
			prospect:
				version === 1 ? prospect() : prospect({ etag: "e2", region: "花蓮" }),
		}),
		updateProspect: (req: unknown) => {
			const request = create(UpdateProspectRequestSchema, req as never);
			updates.push({
				etag: request.prospect?.etag ?? "",
				paths: [...(request.updateMask?.paths ?? [])],
				name: request.prospect?.displayName ?? "",
			});
			if (updates.length === 1) {
				version = 2;
				throw versionConflict();
			}
			return {
				prospect: prospect({
					etag: "e3",
					region: "花蓮",
					displayName: "新名稱",
				}),
			};
		},
	});
	await openDetail(transport);
	assert.equal(
		(byId("prospect-edit-region") as HTMLInputElement).value,
		"嘉義",
	);
	await type(byId("prospect-edit-display-name"), "新名稱");
	await click("儲存變更");
	await until(() => bodyText().includes("你輸入的內容還在"), "衝突句");

	assert.match(
		bodyText(),
		/資料已被其他操作更新。畫面已重新讀取，你輸入的內容還在，請確認後再送出。/u,
	);
	assert.equal(
		(byId("prospect-edit-display-name") as HTMLInputElement).value,
		"新名稱",
	);
	assert.equal(
		(byId("prospect-edit-region") as HTMLInputElement).value,
		"花蓮",
		"沒碰的欄位換成新版本",
	);
	await click("儲存變更");
	await until(() => updates.length === 2, "再送出");
	assert.deepEqual(updates[0].paths, ["display_name"]);
	assert.deepEqual(updates[1], {
		etag: "e2",
		paths: ["display_name"],
		name: "新名稱",
	});
});

test("儲存只送改過的欄位並帶 etag；成功後表單以回應為基準，不誤報衝突，儲存鍵回到停用", async () => {
	const updates: { etag: string; paths: string[]; id: string }[] = [];
	const transport = transportWith({
		getProspect: () => ({ prospect: prospect() }),
		updateProspect: (req: unknown) => {
			const request = create(UpdateProspectRequestSchema, req as never);
			updates.push({
				etag: request.prospect?.etag ?? "",
				paths: [...(request.updateMask?.paths ?? [])],
				id: request.prospect?.id ?? "",
			});
			return { prospect: prospect({ etag: "e2", displayName: "新名稱" }) };
		},
	});
	await openDetail(transport);
	assert.ok(buttonByText("儲存變更")?.disabled, "沒有改動時停用");
	await type(byId("prospect-edit-display-name"), "新名稱");
	assert.equal(buttonByText("儲存變更")?.disabled, false);
	await click("儲存變更");
	await until(() => updates.length === 1, "送出");
	await until(() => bodyText().includes("已儲存"), "成功提示");
	assert.deepEqual(updates, [{ etag: "e1", paths: ["display_name"], id: "7" }]);
	await React.act(async () => {
		await new Promise((resolve) => setTimeout(resolve, 20));
	});
	assert.doesNotMatch(bodyText(), /你輸入的內容還在/u);
	assert.ok(buttonByText("儲存變更")?.disabled);
});

test("還原撞到相符業者：還原模式的說明、不帶 etag，確認後帶全部 ID 重送", async () => {
	const restores: { keys: string[]; ids: string[] }[] = [];
	const transport = transportWith({
		getProspect: () => ({
			prospect: prospect({ status: ProspectStatus.ARCHIVED }),
		}),
		previewProspect: () => ({ matches: [] }),
		restoreProspect: (req: unknown) => {
			const request = create(RestoreProspectRequestSchema, req as never);
			restores.push({
				keys: Object.keys(req as object).sort(),
				ids: [...request.confirmedDistinctProspectIds],
			});
			if (restores.length === 1) throw matchFound("21");
			return { prospect: prospect() };
		},
	});
	const m = await openDetail(transport);
	await click("還原");
	assert.match(bodyText(), /確定要還原這筆站外業者嗎？/u);
	await click("確認還原");
	await until(
		() => bodyText().includes("可能是同一家業者"),
		"還原模式的對話框",
	);
	await until(() => rowNames().length === 1, "相符業者");
	assert.match(
		bodyText(),
		/下列業者的聯絡方式與這筆相同。是同一家的話不需要還原/u,
	);
	assert.ok(buttonByText("確認不是同一家並還原"));
	assert.equal(restores[0].keys.includes("etag"), false, "還原不帶 etag");
	assert.deepEqual(restores[0].ids, [], "第一次送出沒有預先帶 ID");
	await clickElement(checkboxes()[0]);
	await click("確認不是同一家並還原");
	await until(() => restores.length === 2, "重送");
	assert.deepEqual(restores[1].ids, ["21"]);
	await m.unmount();
});

test("封存的業者頁：欄位唯讀顯示值，沒有儲存鍵與轉成店家", async () => {
	const transport = transportWith({
		getProspect: () => ({
			prospect: prospect({ status: ProspectStatus.ARCHIVED }),
		}),
	});
	const m = await openDetail(transport);
	const name = byId("prospect-edit-display-name") as HTMLInputElement;
	assert.equal(name.value, "阿里山農場");
	assert.equal(name.disabled, true);
	assert.equal(
		(byId("prospect-edit-region") as HTMLInputElement).value,
		"嘉義",
	);
	assert.equal(buttonByText("儲存變更"), undefined, "沒有儲存鍵");
	assert.equal(
		(byId("prospect-media-source") as HTMLInputElement).disabled,
		true,
		"圖片區塊仍在，來源網址停用",
	);
	await m.unmount();
});

test("已入駐的業者頁：顯示目前的 logo 與來源網址，選圖、來源網址與儲存停用", async () => {
	const transport = transportWith({
		getProspect: () => ({
			prospect: prospect({
				status: ProspectStatus.CONVERTED,
				outreachStatus: OutreachStatus.CONVERTED,
				convertedStoreId: "88",
				logoImage: { uriTemplate: "https://img.test/logo/{preset}.webp" },
				logoSourceUrl: "https://brand.test/about",
			}),
		}),
	});
	const m = await openDetail(transport);
	const logo = doc().querySelector(
		'img[alt="阿里山農場 Logo"]',
	) as HTMLImageElement | null;
	assert.ok(logo, "顯示目前的 logo");
	assert.match(logo.src, /^https:\/\/img\.test\/logo\//u);
	assert.ok(
		doc().querySelector('a[href="https://brand.test/about"]'),
		"顯示來源網址",
	);
	assert.equal(
		(byId("prospect-media-file") as HTMLInputElement).disabled,
		true,
	);
	assert.equal(
		(byId("prospect-media-source") as HTMLInputElement).disabled,
		true,
	);
	assert.ok(buttonByText("儲存Logo／大頭貼")?.disabled, "儲存鍵停用");
	const outreach = byId("prospect-edit-outreach-status") as HTMLSelectElement;
	assert.equal(
		outreach.selectedOptions[0]?.textContent,
		outreachStatusText(OutreachStatus.CONVERTED),
		"招商狀態顯示目前值而不是第一個選項",
	);
	assert.equal(outreach.selectedOptions[0]?.disabled, true);
	await m.unmount();
});

test("封存撞到版本衝突、重讀回已封存：對話框只留取消並顯示衝突句，沒有還原鍵", async () => {
	let archived = false;
	const transport = transportWith({
		getProspect: () => ({
			prospect: archived
				? prospect({ status: ProspectStatus.ARCHIVED, etag: "e2" })
				: prospect(),
		}),
		archiveProspect: () => {
			archived = true;
			throw versionConflict();
		},
	});
	const m = await openDetail(transport);
	await click("封存");
	await click("確認封存");
	await until(
		() =>
			bodyText().includes("資料已被其他操作更新") &&
			buttonByText("確認封存") === undefined,
		"衝突句與重讀後的畫面",
	);
	assert.match(bodyText(), /確定要封存這筆站外業者嗎？/u);
	assert.equal(buttonByText("確認還原") === undefined, true, "不出現還原鍵");
	assert.equal(buttonByText("確認封存") === undefined, true, "也不能再封存");
	assert.ok(buttonByText("取消"));
	await m.unmount();
});

test("業務優先星等只接受數字；清空會列進 update_mask 且值為 0", async () => {
	const updates: { paths: string[]; stars: number }[] = [];
	const transport = transportWith({
		getProspect: () => ({ prospect: prospect({ priorityStars: 3 }) }),
		updateProspect: (req: unknown) => {
			const request = create(UpdateProspectRequestSchema, req as never);
			updates.push({
				paths: [...(request.updateMask?.paths ?? [])],
				stars: request.prospect?.priorityStars ?? -1,
			});
			return { prospect: prospect({ etag: "e2" }) };
		},
	});
	const m = await openDetail(transport);
	const stars = () => byId("prospect-edit-priority-stars") as HTMLInputElement;
	await type(stars(), "3a-");
	assert.equal(stars().value, "3", "去掉非數字字元");
	assert.ok(buttonByText("儲存變更")?.disabled, "去掉後與原值相同，沒有改動");
	await type(stars(), "");
	await click("儲存變更");
	await until(() => updates.length === 1, "送出");
	assert.deepEqual(updates[0], { paths: ["priority_stars"], stars: 0 });
	await m.unmount();
});

test("清單：翻頁帶 page_token、換篩選回第一頁、狀態預設進行中；統計卡有下一頁時顯示「+」", async () => {
	const lists: {
		pageToken: string;
		status: ProspectStatus;
		outreach: number;
	}[] = [];
	const transport = transportWith({
		listProspects: (req: {
			pageToken: string;
			status: ProspectStatus;
			outreachStatus: number;
		}) => {
			lists.push({
				pageToken: req.pageToken,
				status: req.status,
				outreach: req.outreachStatus,
			});
			return {
				prospects: [
					prospect({
						id: `p-${req.pageToken || "1"}`,
						displayName: `業者-${req.pageToken || "1"}`,
					}),
				],
				nextPageToken:
					req.pageToken === "" && req.status === ProspectStatus.ACTIVE
						? "t2"
						: "",
			};
		},
		listProspectDuplicates: () => ({
			duplicates: Array.from({ length: 50 }, () => ({})),
			nextPageToken: "more",
		}),
		listProspectTaxIDMatches: () => ({
			matches: [{}, {}, {}],
			nextPageToken: "",
		}),
	});
	const m = open(h(ListPage), transport);
	await m.render();

	assert.deepEqual(lists[0], {
		pageToken: "",
		status: ProspectStatus.ACTIVE,
		outreach: OutreachStatus.UNSPECIFIED,
	});
	assert.deepEqual(
		[...doc().querySelectorAll("button[aria-label]")].map((b) =>
			b.getAttribute("aria-label"),
		),
		[
			"疑似重複：50+ 組候選，點選查看清單",
			"統編對帳：3 筆待覆核，點選查看清單",
		],
	);
	assert.match(bodyText(), /預設只列進行中的業者/u);
	assert.match(bodyText(), /業者-1/u);

	await click("下一頁");
	await until(() => bodyText().includes("第 2 頁"), "第 2 頁");
	assert.equal(lists.at(-1)?.pageToken, "t2");
	await until(() => bodyText().includes("業者-t2"), "第 2 頁的資料");

	const status = doc().querySelector(
		'select[aria-label="狀態"]',
	) as HTMLSelectElement;
	assert.deepEqual(
		[...status.options].map((o) => o.textContent),
		["進行中", "已封存", "已入駐"],
		"狀態篩選沒有「全部」",
	);
	await type(status, "archived");
	await until(
		() => lists.at(-1)?.status === ProspectStatus.ARCHIVED,
		"以新篩選重讀",
	);
	assert.equal(lists.at(-1)?.pageToken, "", "換篩選回第一頁");
	assert.doesNotMatch(bodyText(), /第 2 頁/u);
	assert.deepEqual(navigation.replaced, [
		"/admin/seller-prospects?status=archived",
	]);

	await React.act(async () => {
		setSearch("status=garbage&vendorType=business");
	});
	await until(
		() => lists.at(-1)?.status === ProspectStatus.ACTIVE,
		"不認得的狀態回到預設",
	);
});
