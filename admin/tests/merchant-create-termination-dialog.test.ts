import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test, { afterEach, mock } from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import {
	LinkedAccountSchema,
	LinkedIdentity,
	MeasureRequestSchema,
	MeasureSchema,
	TerminationOptionSchema,
	TerminationViolationRequirement,
	ViolationSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	MeasureKind,
	MeasurePeriodSchema,
	MeasureRequestKind,
	MeasureRequestState,
	MeasureState,
	TerminationGround,
	ViolationStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient } from "@tanstack/react-query";
import { JSDOM } from "jsdom";

/**
 * 終止合作對話框（CreateTerminationDialog）與措施詳情頁的區塊（LinkedAccountsSection、MeasureContentSection）實際掛進 DOM 後的行為，
 * 用 router transport 回真的 protobuf 訊息（不是掃原始碼）：request_id 重送沿用／改內容換新／重開新的、
 * 重大違約的認定複選與送出的請求、關聯終止（事由與來源不可改）、關聯帳號區塊只在有命中時出現。
 */

const dom = new JSDOM("<!doctype html><body></body>", {
	url: "http://localhost/",
	pretendToBeVisual: true,
});
const g = globalThis as Record<string, unknown>;
g.window = dom.window;
// next/link 載入時讀 `self`。
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
const TerminationDialog = requireFromTest(
	"../src/app/admin/merchants/[id]/CreateTerminationDialog",
).default as unknown;
const ActiveCards = requireFromTest(
	"../src/app/admin/merchants/[id]/ActiveMeasuresCard",
) as {
	ScheduledTerminationNotice: unknown;
	ActiveMeasureCards: unknown;
};
const HistoryCard = requireFromTest(
	"../src/app/admin/merchants/[id]/MeasureHistoryCard",
).default as unknown;
const LinkedSection = requireFromTest(
	"../src/app/admin/measures/[id]/LinkedAccountsSection",
).default as unknown;
const { MeasureContentSection } = requireFromTest(
	"../src/app/admin/measures/[id]/MeasureSections",
) as { MeasureContentSection: unknown };

type Request = {
	sourceCaseId?: string;
	requestId?: string;
	storeId?: string;
	statement?: string;
	measure?: {
		case?: string;
		value?: {
			ground?: number;
			sourceMeasureId?: string;
			violation?: { citedFindingIds?: string[] };
		};
	};
};

/**
 * 伺服器給的生效時機：句子刻意用和現行規則不同的天數與日期（45 天、2099 年），證明畫面顯示的是伺服器的句子，
 * 不是 client 自己推算或寫死的天數。
 */
const SERVER_NOTICE_SENTENCE =
	"建立後 45 天生效（2099/01/02 03:04）。期間店家照常營業，賣家可提出申覆。";
const SERVER_IMMEDIATE_SENTENCE = "立即生效：店家暫停營業。";

/** 伺服器給的依據需求：預告與關聯帳號不可帶、重大違約必填、其他選填（model.TerminationGround.ViolationRequirement）。 */
function requirementFor(ground: number) {
	if (ground === TerminationGround.MATERIAL_BREACH) {
		return TerminationViolationRequirement.REQUIRED;
	}
	return ground === TerminationGround.NOTICE ||
		ground === TerminationGround.LINKED_ACCOUNT
		? TerminationViolationRequirement.FORBIDDEN
		: TerminationViolationRequirement.OPTIONAL;
}

function effectFor(ground: number) {
	const notice = ground === TerminationGround.NOTICE;
	return create(TerminationOptionSchema, {
		ground,
		violationRequirement: requirementFor(ground),
		effectiveTime: notice
			? timestampFromDate(new Date("2099-01-01T19:04:00Z"))
			: undefined,
		explanation: {
			locale: "zh-TW",
			message: notice ? SERVER_NOTICE_SENTENCE : SERVER_IMMEDIATE_SENTENCE,
		},
	});
}

/** 伺服器給的可選事由（enum 去掉 UNSPECIFIED 與 LINKED_ACCOUNT，依數字順序）。 */
const SELECTABLE_GROUNDS = Object.values(TerminationGround).filter(
	(v): v is TerminationGround =>
		typeof v === "number" &&
		v !== TerminationGround.UNSPECIFIED &&
		v !== TerminationGround.LINKED_ACCOUNT,
);
const SERVER_OPTIONS = SELECTABLE_GROUNDS.map(effectFor);

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

/**
 * 等到 React Query 裡至少有一個查詢、而且所有查詢都不在 pending／fetching（成功或失敗都算）：
 * 以條件等待，不靠固定次數的計時器。每一輪都包在 act 裡，讓狀態更新與渲染完成。
 */
async function settle(m: { queryClient: QueryClient }) {
	const done = () => {
		const queries = m.queryClient.getQueryCache().getAll();
		return (
			queries.length > 0 &&
			m.queryClient.isFetching() === 0 &&
			queries.every((q) => q.state.status !== "pending")
		);
	};
	const deadline = Date.now() + 5000;
	while (!done()) {
		assert.ok(Date.now() < deadline, "查詢在 5 秒內沒有完成");
		await React.act(async () => {
			await new Promise((resolve) => setTimeout(resolve, 1));
		});
	}
	// React Query 把訂閱者的通知排到下一個 macrotask，所以查詢完成後再讓出一輪，渲染才會反映結果。
	await React.act(async () => {
		await new Promise((resolve) => setTimeout(resolve, 0));
	});
}

function transportWith(handlers: {
	listViolations?: () => { violations: unknown[]; nextPageToken?: string };
	getMeasure?: (request: { id: string }) => { measure: unknown };
	listMeasures?: () => { measures: unknown[]; nextPageToken?: string };
}) {
	return createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listViolations: (() =>
				handlers.listViolations?.() ?? { violations: [] }) as never,
			getMeasure: ((req: { id: string }) =>
				handlers.getMeasure?.(req)) as never,
			listMeasures: (() =>
				handlers.listMeasures?.() ?? { measures: [] }) as never,
		});
	});
}

/** 還沒卸載的 root；斷言失敗時由 afterEach 卸載，失敗才會停在那一條斷言而不是整檔逾時。 */
const openRoots = new Set<() => Promise<void>>();
afterEach(async () => {
	for (const unmount of [...openRoots]) await unmount();
	openRoots.clear();
});

function mount(element: (t: unknown) => unknown, transport: unknown) {
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
		queryClient,
		mount: () =>
			React.act(async () => {
				root.render(
					React.createElement(
						TransportProvider,
						{ transport },
						React.createElement(
							QueryClientProvider,
							{ client: queryClient },
							element(transport),
						),
					),
				);
			}),
		unmount,
	};
}

function openTermination(
	outcomes: Array<"fail" | "ok">,
	props: Record<string, unknown> = {},
	handlers: Parameters<typeof transportWith>[0] = {},
) {
	const requests: Request[] = [];
	const onConfirm = async (request: Request) => {
		requests.push(request);
		if ((outcomes.shift() ?? "ok") === "fail") {
			throw new ConnectError("unavailable", Code.Unavailable);
		}
	};
	const m = mount(
		() =>
			React.createElement(TerminationDialog, {
				storeId: "100",
				accountId: "acc-1",
				options: SERVER_OPTIONS,
				onClose: () => {},
				onConfirm,
				...props,
			}),
		transportWith(handlers),
	);
	return { requests, ...m };
}

async function fillCommon(ground: number | null) {
	await React.act(async () => {
		if (ground !== null) {
			setValue(byId("create-termination-ground"), String(ground));
		}
		setValue(byId("create-termination-statement"), "店家已停業");
		setValue(byId("create-termination-note"), "電話確認");
	});
}

test("同一份內容失敗後重送：request_id 不變；改過內容換新的；重新開啟是新的", async () => {
	const d = openTermination(["fail", "fail", "ok"]);
	await d.mount();
	await fillCommon(TerminationGround.CEASED_BUSINESS);
	await click("確認建立");
	await click("確認建立");
	assert.equal(d.requests.length, 2);
	assert.ok(d.requests[0]?.requestId);
	assert.equal(
		d.requests[1]?.requestId,
		d.requests[0]?.requestId,
		"同一份內容的重送必須沿用同一個 request_id",
	);
	assert.match(bodyText(), /建立失敗|unavailable|不可用/u);

	await React.act(async () => {
		setValue(byId("create-termination-statement"), "店家已停業，改過");
	});
	await click("確認建立");
	assert.notEqual(
		d.requests[2]?.requestId,
		d.requests[0]?.requestId,
		"內容改過必須換新的 request_id",
	);
	await d.unmount();

	const again = openTermination(["ok"]);
	await again.mount();
	await fillCommon(TerminationGround.CEASED_BUSINESS);
	await click("確認建立");
	assert.notEqual(
		again.requests[0]?.requestId,
		d.requests[0]?.requestId,
		"關閉再開啟是新的 request_id",
	);
	await again.unmount();
});

test("終止事由下拉只列伺服器給的可選事由（已關閉的店沒有預告）；沒給的事由不在下拉", async () => {
	const d = openTermination(["ok"], {
		options: [
			effectFor(TerminationGround.CEASED_BUSINESS),
			effectFor(TerminationGround.FRAUD),
		],
	});
	await d.mount();
	const select = byId("create-termination-ground") as HTMLSelectElement;
	assert.deepEqual(
		[...select.options].map((o) => Number(o.value)),
		[
			TerminationGround.UNSPECIFIED,
			TerminationGround.CEASED_BUSINESS,
			TerminationGround.FRAUD,
		],
	);
	await d.unmount();
});

test("預告的生效時機是伺服器的句子：畫面重繪不改它，也沒有 client 寫死的預告天數", async () => {
	mock.timers.enable({ apis: ["Date"], now: new Date("2026-10-15T02:00:00Z") });
	try {
		const d = openTermination(["ok"]);
		await d.mount();
		await fillCommon(TerminationGround.NOTICE);
		assert.match(bodyText(), /建立後 45 天生效（2099\/01\/02 03:04）/u);
		mock.timers.setTime(new Date("2026-10-18T02:00:00Z").getTime());
		await React.act(async () => {
			setValue(byId("create-termination-statement"), "店家已停業，再改");
		});
		assert.match(bodyText(), /建立後 45 天生效（2099\/01\/02 03:04）/u);
		assert.doesNotMatch(bodyText(), /30 天|2026\/11/u);
		await d.unmount();
	} finally {
		mock.timers.reset();
	}
});

test("驗證沒過不送出：沒選事由、沒填理由；補完後送出的請求是 TERMINATION 與該事由", async () => {
	const d = openTermination(["ok"]);
	await d.mount();
	await click("確認建立");
	assert.equal(d.requests.length, 0);
	assert.match(bodyText(), /請選擇終止事由/u);
	await fillCommon(TerminationGround.FRAUD);
	await click("確認建立");
	assert.equal(d.requests.length, 1);
	assert.equal(d.requests[0]?.storeId, "100");
	assert.equal(d.requests[0]?.measure?.case, "termination");
	assert.equal(d.requests[0]?.measure?.value?.ground, TerminationGround.FRAUD);
	await d.unmount();
});

test("說明文字依事由：第一句是伺服器給的生效時機句，共同說明在；沒有選事由或伺服器沒給該事由的效果時沒有第一句", async () => {
	const d = openTermination(["ok"]);
	await d.mount();
	assert.match(bodyText(), /請先選擇終止事由/u);
	await fillCommon(TerminationGround.NOTICE);
	assert.match(
		bodyText(),
		new RegExp(SERVER_NOTICE_SENTENCE.replace(/[()/]/gu, "\\$&"), "u"),
	);
	assert.match(
		bodyText(),
		/未結訂單完成後店家會關閉。終止生效後 6 個月內撤銷或申覆成立/u,
	);
	await fillCommon(TerminationGround.DISSOLUTION);
	assert.match(bodyText(), /立即生效：店家暫停營業。/u);
	assert.doesNotMatch(bodyText(), /建立後 45 天生效/u);
	await d.unmount();

	const bare = openTermination(["ok"], {
		options: SELECTABLE_GROUNDS.map((ground) =>
			create(TerminationOptionSchema, { ground }),
		),
	});
	await bare.mount();
	await fillCommon(TerminationGround.NOTICE);
	assert.doesNotMatch(bodyText(), /天生效|立即生效/u);
	assert.match(bodyText(), /未結訂單完成後店家會關閉/u);
	await bare.unmount();
});

test("重大違約：出現「依據的認定」複選（只列生效中的認定），沒選不送出，選了送出所選的 id", async () => {
	let statusSeen: number | undefined;
	const transport = createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listViolations: (req) => {
				statusSeen = req.status;
				return {
					violations: [
						create(ViolationSchema, {
							id: "f1",
							createTime: timestampFromDate(new Date("2026-09-01T00:00:00Z")),
						}),
						create(ViolationSchema, { id: "f2" }),
					],
				};
			},
		});
	});
	const requests: Request[] = [];
	const m = mount(
		() =>
			React.createElement(TerminationDialog, {
				storeId: "100",
				accountId: "acc-1",
				options: SERVER_OPTIONS,
				onClose: () => {},
				onConfirm: async (r: Request) => {
					requests.push(r);
				},
			}),
		transport,
	);
	await m.mount();
	assert.equal(
		dom.window.document.querySelectorAll('input[type="checkbox"]').length,
		0,
		"其他事由不顯示認定複選",
	);
	await fillCommon(TerminationGround.MATERIAL_BREACH);
	await settle(m);
	const boxes = [
		...dom.window.document.querySelectorAll('input[type="checkbox"]'),
	] as HTMLInputElement[];
	assert.equal(boxes.length, 2);
	assert.equal(statusSeen, ViolationStatus.ENFORCED);
	await click("確認建立");
	assert.equal(requests.length, 0);
	assert.match(bodyText(), /請選擇依據的認定/u);
	// 先勾 f2 再勾 f1：引用的認定是集合，送出的 id 排序（不取決於勾選順序）。
	await React.act(async () => {
		boxes[1]?.click();
	});
	await React.act(async () => {
		boxes[0]?.click();
	});
	await click("確認建立");
	assert.deepEqual(requests[0]?.measure?.value?.violation?.citedFindingIds, [
		"f1",
		"f2",
	]);
	await m.unmount();
});

test("依據的認定欄位依選項的需求：選填的事由出現欄位、不選也能送出；選了送出所選的 id；不可帶的事由沒有欄位；文字不寫 7 日規則", async () => {
	const transport = createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listViolations: () => ({
				violations: [create(ViolationSchema, { id: "f1" })],
			}),
		});
	});
	const open = () => {
		const requests: Request[] = [];
		const m = mount(
			() =>
				React.createElement(TerminationDialog, {
					storeId: "100",
					accountId: "acc-1",
					options: SERVER_OPTIONS,
					onClose: () => {},
					onConfirm: async (r: Request) => {
						requests.push(r);
					},
				}),
			transport,
		);
		return { requests, ...m };
	};
	const boxes = () =>
		dom.window.document.querySelectorAll('input[type="checkbox"]');

	// 選填而沒選：欄位出現、說明是「選填。」、不選也能送出、請求沒有 violation。
	const unpicked = open();
	await unpicked.mount();
	await fillCommon(TerminationGround.NOTICE);
	assert.equal(boxes().length, 0, "不可帶依據的事由沒有欄位");
	await fillCommon(TerminationGround.FRAUD);
	await settle(unpicked);
	assert.equal(boxes().length, 1, "選填的事由出現欄位");
	assert.match(bodyText(), /選填。/u);
	assert.doesNotMatch(bodyText(), /已存在 7 日以上/u);
	await click("確認建立");
	assert.equal(unpicked.requests.length, 1, "選填：不選也能送出");
	assert.equal(unpicked.requests[0]?.measure?.value?.violation, undefined);
	await unpicked.unmount();

	// 選填而有選：送出所選的 id。
	const picked = open();
	await picked.mount();
	await fillCommon(TerminationGround.FRAUD);
	await settle(picked);
	await React.act(async () => {
		(boxes()[0] as HTMLInputElement).click();
	});
	await click("確認建立");
	assert.deepEqual(
		picked.requests[0]?.measure?.value?.violation?.citedFindingIds,
		["f1"],
	);
	await picked.unmount();

	// 必填：說明只有「至少選一筆。」，7 日規則由伺服器的說明句呈現。
	const required = open();
	await required.mount();
	await fillCommon(TerminationGround.MATERIAL_BREACH);
	await settle(required);
	assert.match(bodyText(), /至少選一筆。/u);
	assert.doesNotMatch(bodyText(), /已存在 7 日以上/u);
	await required.unmount();
});

test("帳號沒有生效中的認定：必填的事由說「無法以這個事由終止」、選填的事由說「沒有生效中的認定」，文字由選項的需求決定", async () => {
	const transport = createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listViolations: () => ({ violations: [] }),
		});
	});
	const open = () =>
		mount(
			() =>
				React.createElement(TerminationDialog, {
					storeId: "100",
					accountId: "acc-1",
					options: SERVER_OPTIONS,
					onClose: () => {},
					onConfirm: async () => {},
				}),
			transport,
		);

	const required = open();
	await required.mount();
	await fillCommon(TerminationGround.MATERIAL_BREACH);
	await settle(required);
	assert.match(bodyText(), /這個帳號沒有生效中的認定，無法以這個事由終止。/u);
	assert.doesNotMatch(bodyText(), /無法以重大違約終止/u);
	await required.unmount();

	const optional = open();
	await optional.mount();
	await fillCommon(TerminationGround.FRAUD);
	await settle(optional);
	assert.match(bodyText(), /這個帳號沒有生效中的認定。/u);
	assert.doesNotMatch(bodyText(), /無法以這個事由終止/u);
	await optional.unmount();
});

test("關聯終止：事由固定且不能改、顯示來源措施、請求帶來源與該列帳號的店家", async () => {
	const d = openTermination(["ok"], {
		storeId: "200",
		accountId: "acc-2",
		sourceMeasureId: "src-9",
		options: [effectFor(TerminationGround.LINKED_ACCOUNT)],
	});
	await d.mount();
	const select = byId("create-termination-ground") as HTMLSelectElement;
	assert.equal(select.disabled, true);
	assert.equal(select.value, String(TerminationGround.LINKED_ACCOUNT));
	assert.equal(select.options.length, 1);
	assert.match(bodyText(), /來源措施：src-9/u);
	assert.match(bodyText(), /立即生效：店家暫停營業。/u);
	assert.doesNotMatch(
		bodyText(),
		/來源措施撤銷時/u,
		"關聯帳號的條件句由伺服器的說明句提供，client 不寫",
	);
	await React.act(async () => {
		setValue(byId("create-termination-statement"), "同一身分已被終止");
		setValue(byId("create-termination-note"), "關聯");
	});
	await click("確認建立");
	assert.equal(d.requests[0]?.storeId, "200");
	assert.equal(
		d.requests[0]?.measure?.value?.ground,
		TerminationGround.LINKED_ACCOUNT,
	);
	assert.equal(d.requests[0]?.measure?.value?.sourceMeasureId, "src-9");
	await d.unmount();
});

// ---------------------------------------------------------------------------
// 措施詳情

function detail(measure: unknown) {
	const linked: Array<[string, string]> = [];
	const m = mount(
		() =>
			React.createElement(
				"div",
				null,
				React.createElement(LinkedSection, {
					measure,
					onCreateLinked: (source: string, account: { accountId: string }) =>
						linked.push([source, account.accountId]),
				}),
				React.createElement(MeasureContentSection, { measure }),
			),
		transportWith({}),
	);
	return { linked, ...m };
}

/** 第一列的 can_create_linked_termination 由呼叫端決定（伺服器推導，client 只呈現）；第二列已建立關聯終止。 */
function linkedRows(canCreateFirst: boolean) {
	return [
		create(LinkedAccountSchema, {
			accountId: "acc-2",
			storeId: "200",
			storeCode: "S200",
			matchedIdentity: LinkedIdentity.UBN,
			canCreateLinkedTermination: canCreateFirst,
		}),
		create(LinkedAccountSchema, {
			accountId: "acc-3",
			storeId: "300",
			storeCode: "S300",
			matchedIdentity: LinkedIdentity.NATIONAL_ID,
			linkedMeasureId: "m77",
		}),
	];
}
const LINKED_ROWS = linkedRows(true);

test("措施詳情頁：有命中時列出相同身分的其他帳號（店號連結、身分類別、已建立或按鈕），按鈕帶來源措施與該列帳號", async () => {
	const d = detail(
		create(MeasureSchema, {
			id: "m1",
			kind: MeasureKind.TERMINATION,
			state: MeasureState.ACTIVE,
			terminationGround: TerminationGround.FRAUD,
			linkedAccounts: LINKED_ROWS,
			appealDeadlineTime: timestampFromDate(new Date("2027-05-18T02:00:00Z")),
		}),
	);
	await d.mount();
	assert.match(bodyText(), /相同身分的其他帳號/u);
	assert.match(bodyText(), /統編/u);
	assert.match(bodyText(), /身分證號/u);
	assert.match(bodyText(), /已建立關聯終止（措施 m77）/u);
	assert.match(bodyText(), /申覆期限2027\/05\/18 10:00/u);
	const links = [...dom.window.document.querySelectorAll("a")].map((a) =>
		a.getAttribute("href"),
	);
	assert.deepEqual(links, [
		"/admin/merchants/200",
		"/admin/merchants/300",
		"/admin/measures/m77",
	]);
	assert.equal(
		[...dom.window.document.querySelectorAll("button")].filter(
			(b) => b.textContent === "建立關聯終止",
		).length,
		1,
		"已建立關聯終止的那一列不再有按鈕",
	);
	await click("建立關聯終止");
	assert.deepEqual(d.linked, [["m1", "acc-2"]]);
	await d.unmount();
});

test("措施詳情頁：沒有命中不顯示區塊；來源措施不能建立關聯終止時不給按鈕；關聯終止顯示來源措施並可切過去", async () => {
	const none = detail(
		create(MeasureSchema, {
			id: "m1",
			kind: MeasureKind.TERMINATION,
			state: MeasureState.ACTIVE,
			terminationGround: TerminationGround.FRAUD,
		}),
	);
	await none.mount();
	assert.doesNotMatch(bodyText(), /相同身分的其他帳號/u);
	await none.unmount();

	const suspension = detail(
		create(MeasureSchema, {
			id: "m1",
			kind: MeasureKind.SUSPENSION,
			state: MeasureState.ACTIVE,
			linkedAccounts: linkedRows(false),
		}),
	);
	await suspension.mount();
	assert.match(bodyText(), /相同身分的其他帳號/u);
	assert.match(bodyText(), /命中的身分類別/u);
	assert.equal(
		buttonByText("建立關聯終止")?.textContent,
		undefined,
		"不該有按鈕",
	);
	await suspension.unmount();

	// 按鈕只看伺服器給的 can_create_linked_termination：措施是生效中的詐欺終止，但伺服器說不行（例如該帳號的店家不允許）就沒有按鈕。
	const refused = detail(
		create(MeasureSchema, {
			id: "m1",
			kind: MeasureKind.TERMINATION,
			state: MeasureState.ACTIVE,
			terminationGround: TerminationGround.FRAUD,
			linkedAccounts: linkedRows(false),
		}),
	);
	await refused.mount();
	assert.equal(
		buttonByText("建立關聯終止")?.textContent,
		undefined,
		"不該有按鈕",
	);
	await refused.unmount();

	const child = detail(
		create(MeasureSchema, {
			id: "m1",
			kind: MeasureKind.TERMINATION,
			state: MeasureState.ACTIVE,
			terminationGround: TerminationGround.LINKED_ACCOUNT,
			sourceMeasureId: "m0",
		}),
	);
	await child.mount();
	assert.match(bodyText(), /來源措施/u);
	const source = [...dom.window.document.querySelectorAll("a")].find(
		(a) => a.textContent === "m0",
	);
	assert.equal(source?.getAttribute("href"), "/admin/measures/m0");
	await child.unmount();
});

// ---------------------------------------------------------------------------
// 預定終止與生效中的終止卡

test("預定終止：以帳號與 state=SCHEDULED 讀 ListMeasures，顯示最早的生效時間；沒有預定終止時不渲染", async () => {
	let seen: { accountId?: string; state?: number } = {};
	let items: unknown[] = [
		create(MeasureSchema, {
			id: "m1",
			kind: MeasureKind.TERMINATION,
			state: MeasureState.SCHEDULED,
			period: create(MeasurePeriodSchema, {
				startTime: timestampFromDate(new Date("2026-11-14T02:00:00Z")),
			}),
		}),
	];
	const transport = createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listMeasures: (req) => {
				seen = { accountId: req.accountId, state: req.state };
				return { measures: items as never };
			},
		});
	});
	const m = mount(
		() =>
			React.createElement(ActiveCards.ScheduledTerminationNotice, {
				accountId: "acc-1",
			}),
		transport,
	);
	await m.mount();
	await settle(m);
	assert.deepEqual(seen, { accountId: "acc-1", state: MeasureState.SCHEDULED });
	assert.match(bodyText(), /預定終止：2026\/11\/14 10:00/u);
	await m.unmount();

	items = [];
	const empty = mount(
		() =>
			React.createElement(ActiveCards.ScheduledTerminationNotice, {
				accountId: "acc-1",
			}),
		transport,
	);
	await empty.mount();
	await settle(empty);
	assert.doesNotMatch(bodyText(), /預定終止/u);
	await empty.unmount();
});

test("預定終止讀取失敗：說出來並可重試，不當成沒有預定終止", async () => {
	const transport = createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listMeasures: () => {
				throw new ConnectError("boom", Code.Unavailable);
			},
		});
	});
	const m = mount(
		() =>
			React.createElement(ActiveCards.ScheduledTerminationNotice, {
				accountId: "acc-1",
			}),
		transport,
	);
	await m.mount();
	await settle(m);
	assert.ok(buttonByText("重試"));
	await m.unmount();
});

test("生效中的終止卡：標題「合作已終止」、顯示事由與申覆期限，沒有結束入口與複審期限", async () => {
	const measure = create(MeasureSchema, {
		id: "m1",
		kind: MeasureKind.TERMINATION,
		basis: 3,
		state: MeasureState.ACTIVE,
		terminationGround: TerminationGround.FRAUD,
		statement: "詐欺事件",
		canEnd: false,
		appealDeadlineTime: timestampFromDate(new Date("2027-05-18T02:00:00Z")),
	});
	const m = mount(
		() =>
			React.createElement(ActiveCards.ActiveMeasureCards, {
				active: [measure],
				endPending: false,
				onEnd: () => {},
			}),
		transportWith({}),
	);
	await m.mount();
	assert.match(bodyText(), /合作已終止/u);
	assert.doesNotMatch(bodyText(), /賣家停權中/u);
	assert.match(bodyText(), /終止合作・契約條款：詐欺事件/u);
	assert.match(bodyText(), /申覆期限：2027\/05\/18 10:00/u);
	assert.doesNotMatch(bodyText(), /複審期限/u);
	assert.equal(buttonByText("結束措施")?.textContent, undefined, "不該有按鈕");
	await m.unmount();
});

test("措施歷史每一列的「查看」連到該筆措施的詳情頁；請求欄列出申覆與解除請求的全部種類", async () => {
	const rows = [
		create(MeasureSchema, {
			id: "m-a",
			kind: MeasureKind.SUSPENSION,
			state: MeasureState.ENDED,
			requests: [
				create(MeasureRequestSchema, {
					id: "r1",
					kind: MeasureRequestKind.REMEDIATION,
					state: MeasureRequestState.PENDING,
				}),
				create(MeasureRequestSchema, {
					id: "r2",
					kind: MeasureRequestKind.APPEAL,
					state: MeasureRequestState.REJECTED,
				}),
			],
		}),
		create(MeasureSchema, {
			id: "m-b",
			kind: MeasureKind.TERMINATION,
			state: MeasureState.ACTIVE,
			terminationGround: TerminationGround.FRAUD,
		}),
	];
	const m = mount(
		() => React.createElement(HistoryCard, { accountId: "acc-1" }),
		transportWith({ listMeasures: () => ({ measures: rows }) }),
	);
	await m.mount();
	await settle(m);
	const links = [...dom.window.document.querySelectorAll("a")].filter(
		(a) => a.textContent === "查看",
	);
	assert.deepEqual(
		links.map((a) => a.getAttribute("href")),
		["/admin/measures/m-a", "/admin/measures/m-b"],
	);
	assert.match(bodyText(), /解除請求待裁定、申覆駁回/u, "不只顯示申覆");
	assert.equal(dom.window.document.querySelector('[role="dialog"]'), null);
	await m.unmount();
});

test("從案件開啟：頂端說明措施會記為這個案件導出的措施，請求帶 source_case_id；不是從案件開啟時都沒有", async () => {
	const fromCase = openTermination(["ok"], { sourceCaseId: "case-77" });
	await fromCase.mount();
	assert.match(bodyText(), /這個措施會記為案件 case-77 導出的措施。/u);
	await fillCommon(TerminationGround.CEASED_BUSINESS);
	await click("確認建立");
	assert.equal(fromCase.requests[0]?.sourceCaseId, "case-77");
	await fromCase.unmount();

	const plain = openTermination(["ok"]);
	await plain.mount();
	assert.doesNotMatch(bodyText(), /導出的措施/u);
	await fillCommon(TerminationGround.CEASED_BUSINESS);
	await click("確認建立");
	assert.equal("sourceCaseId" in (plain.requests[0] ?? {}), false);
	await plain.unmount();
});
