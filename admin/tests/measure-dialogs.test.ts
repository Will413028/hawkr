import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test, { afterEach } from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { createRouterTransport } from "@connectrpc/connect";
import {
	GovernanceNoteSchema,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import {
	MeasureSchema,
	MeasureActionUnavailableReason as Reason,
	ViolationSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { MeasureBasis } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient } from "@tanstack/react-query";
import { JSDOM } from "jsdom";
import { buildUpdateMeasure } from "../src/app/admin/measures/measure-actions";
import {
	MEASURE_CONFLICT_TEXT,
	toLocalInput,
} from "../src/app/admin/merchants/[id]/measure";
import { SERVER_TEXT, why } from "./fixtures/measure-unavailability";

/**
 * 措施詳情頁的對話框與註記區塊實際掛進 DOM 後的行為（router transport 回真的 protobuf 訊息）：
 * 修改措施的欄位限制與原因、送出鍵在沒改任何欄位時停用、版本衝突時保留輸入；裁定的選項、勾選與提示；內部註記的時間軸與新增。
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
const UpdateDialog = requireFromTest(
	"../src/app/admin/measures/[id]/UpdateMeasureDialog",
).default as unknown;
const DecideDialog = requireFromTest(
	"../src/app/admin/measures/[id]/DecideMeasureRequestDialog",
).default as unknown;
const RevokeDialog = requireFromTest(
	"../src/app/admin/measures/[id]/RevokeMeasureDialog",
).default as unknown;
const NotesSection = requireFromTest(
	"../src/app/admin/measures/[id]/MeasureNotesSection",
).default as unknown;

function setValue(el: Element, value: string) {
	const proto =
		el instanceof dom.window.HTMLTextAreaElement
			? dom.window.HTMLTextAreaElement.prototype
			: dom.window.HTMLInputElement.prototype;
	Object.getOwnPropertyDescriptor(proto, "value")?.set?.call(el, value);
	el.dispatchEvent(new dom.window.Event("input", { bubbles: true }));
}

function byId<T extends Element = Element>(id: string): T {
	const el = dom.window.document.getElementById(id);
	assert.ok(el, `找不到 #${id}`);
	return el as unknown as T;
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

const openRoots = new Set<() => Promise<void>>();
afterEach(async () => {
	for (const unmount of [...openRoots]) await unmount();
	openRoots.clear();
});

function mount(element: unknown, transport: unknown = transportWith({})) {
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
	const render = (node: unknown) =>
		React.act(async () => {
			root.render(
				React.createElement(
					TransportProvider,
					{ transport },
					React.createElement(
						QueryClientProvider,
						{ client: queryClient },
						node,
					),
				),
			);
		});
	return {
		queryClient,
		mount: () => render(element),
		/** 同一個根換上新的 props（例如重讀後的措施），元件狀態保留。 */
		rerender: render,
	};
}

async function settle(m: { queryClient: QueryClient }) {
	const done = () =>
		m.queryClient.getQueryCache().getAll().length > 0 &&
		m.queryClient.isFetching() === 0 &&
		m.queryClient
			.getQueryCache()
			.getAll()
			.every((q) => q.state.status !== "pending");
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

const notes: ReturnType<typeof create<typeof GovernanceNoteSchema>>[] = [];
const created: string[] = [];
function transportWith(init: { notes?: typeof notes } = {}) {
	return createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listMeasureNotes: (() => ({ notes: init.notes ?? [] })) as never,
			createMeasureNote: ((req: { note?: { body?: string } }) => {
				created.push(req.note?.body ?? "");
				return { note: create(GovernanceNoteSchema, { id: "n9" }) };
			}) as never,
		});
	});
}

const END = new Date(Date.now() + 48 * 60 * 60 * 1000);
function measure(
	init: Parameters<typeof create<typeof MeasureSchema>>[1] = {},
) {
	return create(MeasureSchema, {
		id: "m1",
		etag: "e1",
		statement: "原本的理由",
		endTime: timestampFromDate(END),
		...init,
	});
}

// ---------------------------------------------------------------------------
// 修改措施

function updateElement(
	m: ReturnType<typeof measure>,
	onConfirm: (
		form: unknown,
		baseline: unknown,
	) => Promise<void> = async () => {},
) {
	return React.createElement(UpdateDialog, {
		measure: m,
		onClose: () => {},
		onConfirm,
	});
}

function openUpdate(
	m: ReturnType<typeof measure>,
	onConfirm: (
		form: unknown,
		baseline: unknown,
	) => Promise<void> = async () => {},
) {
	return mount(updateElement(m, onConfirm));
}

test("修改措施：可以改理由與延長時兩欄都能編輯；沒改任何欄位時送出鍵停用，說明文字在", async () => {
	const d = openUpdate(
		measure({ canUpdateStatement: true, canExtend: true, canShorten: true }),
	);
	await d.mount();
	assert.equal(
		byId<HTMLTextAreaElement>("update-measure-statement").disabled,
		false,
	);
	assert.equal(
		byId<HTMLInputElement>("update-measure-end-time").disabled,
		false,
	);
	assert.match(bodyText(), /賣家會看到新的理由與期間，並產生新的版本/u);
	assert.equal(buttonByText("確認修改")?.disabled, true);
	await React.act(async () => {
		setValue(byId("update-measure-statement"), "新的理由");
	});
	assert.equal(buttonByText("確認修改")?.disabled, false);
	await React.act(async () => {
		setValue(byId("update-measure-statement"), "原本的理由");
	});
	assert.equal(buttonByText("確認修改")?.disabled, true, "改回原值又是沒改");
});

test("修改措施：凍結時理由欄停用並列出伺服器的原因，結束時間上限是目前的結束時間", async () => {
	const d = openUpdate(
		measure({
			canUpdateStatement: false,
			canExtend: false,
			canShorten: true,
			updateStatementUnavailabilities: [
				why(Reason.REQUEST_PENDING_FROZEN, SERVER_TEXT.FROZEN_STATEMENT),
			],
			extendUnavailabilities: [
				why(Reason.REQUEST_PENDING_FROZEN, SERVER_TEXT.FROZEN_EXTEND),
				why(55 as Reason),
			],
		}),
	);
	await d.mount();
	assert.equal(
		byId<HTMLTextAreaElement>("update-measure-statement").disabled,
		true,
	);
	assert.match(bodyText(), /結束時間不能再延長/u, "延長的原因顯示伺服器的文字");
	assert.match(bodyText(), /給賣家的理由不能再改/u);
	assert.match(bodyText(), /目前不能操作/u, "不認得的原因值是通用文字");
	const end = byId<HTMLInputElement>("update-measure-end-time");
	assert.equal(end.disabled, false);
	assert.ok(end.max, "只能縮短時有上限");
});

test("修改措施：結束時間與理由都不能改時（警告）結束時間欄停用並說明原因", async () => {
	const d = openUpdate(
		measure({
			endTime: undefined,
			canUpdateStatement: true,
			extendUnavailabilities: [
				why(Reason.KIND_HAS_NO_END_TIME, SERVER_TEXT.KIND_HAS_NO_END_TIME),
			],
			shortenUnavailabilities: [
				why(Reason.KIND_HAS_NO_END_TIME, SERVER_TEXT.KIND_HAS_NO_END_TIME),
			],
		}),
	);
	await d.mount();
	assert.equal(
		byId<HTMLInputElement>("update-measure-end-time").disabled,
		true,
	);
	assert.match(bodyText(), /警告沒有結束時間/u);
});

test("修改措施：送出失敗（含版本衝突）時對話框留在原地、保留輸入並顯示原因", async () => {
	let attempts = 0;
	const d = openUpdate(
		measure({ canUpdateStatement: true, canExtend: true, canShorten: true }),
		async () => {
			attempts += 1;
			throw new Error(MEASURE_CONFLICT_TEXT);
		},
	);
	await d.mount();
	await React.act(async () => {
		setValue(byId("update-measure-statement"), "新的理由");
	});
	await click("確認修改");
	assert.equal(attempts, 1);
	assert.match(bodyText(), new RegExp(MEASURE_CONFLICT_TEXT, "u"));
	assert.equal(
		byId<HTMLTextAreaElement>("update-measure-statement").value,
		"新的理由",
	);
	assert.equal(buttonByText("確認修改")?.disabled, false, "可以再送一次");
});

test("修改措施：版本衝突後換上新的措施重送，update_mask 只有管理員改過的欄位（別人改的理由不被蓋掉）", async () => {
	const sent: {
		form: { statement: string; endTime: string };
		baseline: unknown;
	}[] = [];
	let fail = true;
	const onConfirm = async (form: unknown, baseline: unknown) => {
		sent.push({ form: form as never, baseline });
		if (fail) throw new Error(MEASURE_CONFLICT_TEXT);
	};
	const opened = measure({
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	const d = mount(updateElement(opened, onConfirm));
	await d.mount();
	const later = toLocalInput(new Date(END.getTime() + 24 * 60 * 60 * 1000));
	await React.act(async () => {
		setValue(byId("update-measure-end-time"), later);
	});
	await click("確認修改");
	assert.equal(sent.length, 1);
	assert.match(bodyText(), new RegExp(MEASURE_CONFLICT_TEXT, "u"));
	// 呼叫端重讀：別的管理員在這之間改了理由，etag 變了。
	const refreshed = measure({
		etag: "e2",
		statement: "別人改過的理由",
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	await d.rerender(updateElement(refreshed, onConfirm));
	assert.equal(
		byId<HTMLTextAreaElement>("update-measure-statement").value,
		"別人改過的理由",
		"沒動過的欄位換成新措施的值",
	);
	assert.equal(byId<HTMLInputElement>("update-measure-end-time").value, later);
	fail = false;
	await click("確認修改");
	assert.equal(sent.length, 2);
	const request = buildUpdateMeasure(
		refreshed,
		sent[1]?.form as never,
		sent[1]?.baseline as never,
	);
	assert.deepEqual(request.updateMask, { paths: ["end_time"] });
	assert.equal(request.measure?.statement, undefined);
	assert.equal(request.measure?.etag, "e2");
});

// ---------------------------------------------------------------------------
// 裁定

function openDecide(
	kind: "appeal" | "remediation",
	m: ReturnType<typeof measure>,
	onConfirm: (form: unknown) => Promise<void> = async () => {},
) {
	return mount(
		React.createElement(DecideDialog, {
			kind,
			measure: m,
			onClose: () => {},
			onConfirm,
		}),
	);
}

const radios = () =>
	[
		...dom.window.document.querySelectorAll('input[type="radio"]'),
	] as HTMLInputElement[];

test("裁定申覆：標題、選項、理由欄；接受前沒有「一併撤銷」勾選，沒選結果與沒填理由不能送出", async () => {
	const d = openDecide(
		"appeal",
		measure({
			basis: MeasureBasis.VIOLATION,
			canRevokeFindings: true,
			findings: [create(ViolationSchema, { id: "f1", originMeasureId: "m1" })],
		}),
	);
	await d.mount();
	assert.match(bodyText(), /裁定申覆/u);
	assert.match(bodyText(), /接受（撤銷這個措施）/u);
	assert.match(bodyText(), /駁回/u);
	assert.match(bodyText(), /裁定理由（賣家看得到）/u);
	assert.doesNotMatch(bodyText(), /一併撤銷這個措施建立的認定/u);
	assert.equal(buttonByText("確認裁定")?.disabled, true);
	await React.act(async () => {
		radios()[0]?.click();
	});
	assert.match(
		bodyText(),
		/一併撤銷這個措施建立的認定（引用它們的其他措施也會撤銷）/u,
	);
	assert.equal(buttonByText("確認裁定")?.disabled, true, "理由還沒填");
	await React.act(async () => {
		setValue(byId("decide-measure-request-reason"), "證據不足");
	});
	assert.equal(buttonByText("確認裁定")?.disabled, false);
	await React.act(async () => {
		radios()[1]?.click();
	});
	assert.doesNotMatch(
		bodyText(),
		/一併撤銷這個措施建立的認定/u,
		"駁回時沒有勾選",
	);
});

test("裁定申覆與撤銷：勾選只看伺服器的 can_revoke_findings（違規依據、有自己建立的認定但伺服器說不行就不出現）", async () => {
	const serverSaysNo = measure({
		basis: MeasureBasis.VIOLATION,
		canRevokeFindings: false,
		findings: [create(ViolationSchema, { id: "f1", originMeasureId: "m1" })],
	});
	const d = openDecide("appeal", serverSaysNo);
	await d.mount();
	await React.act(async () => {
		radios()[0]?.click();
	});
	assert.doesNotMatch(bodyText(), /一併撤銷/u);
	const r = mount(
		React.createElement(RevokeDialog, {
			measure: serverSaysNo,
			onClose: () => {},
			onConfirm: async () => {},
		}),
	);
	await r.mount();
	assert.doesNotMatch(bodyText(), /一併撤銷/u);
	await r.rerender(
		React.createElement(RevokeDialog, {
			measure: measure({ basis: MeasureBasis.RISK, canRevokeFindings: true }),
			onClose: () => {},
			onConfirm: async () => {},
		}),
	);
	assert.match(bodyText(), /一併撤銷這個措施建立的認定/u, "照伺服器");
});

test("裁定：理由超過 2000 字時就地顯示原因並停用送出", async () => {
	const d = openDecide("remediation", measure({}));
	await d.mount();
	await React.act(async () => {
		radios()[1]?.click();
	});
	await React.act(async () => {
		setValue(byId("decide-measure-request-reason"), "字".repeat(2000));
	});
	assert.doesNotMatch(bodyText(), /裁定理由最多 2000 字。/u);
	assert.equal(buttonByText("確認裁定")?.disabled, false);
	await React.act(async () => {
		setValue(byId("decide-measure-request-reason"), "字".repeat(2001));
	});
	assert.match(bodyText(), /裁定理由最多 2000 字。/u);
	assert.equal(buttonByText("確認裁定")?.disabled, true);
	await React.act(async () => {
		setValue(byId("decide-measure-request-reason"), "字".repeat(2000));
	});
	assert.doesNotMatch(bodyText(), /裁定理由最多 2000 字。/u, "改回後訊息消失");
});

test("裁定申覆：風險依據的措施沒有認定可撤銷，接受時也不出現勾選", async () => {
	const d = openDecide("appeal", measure({ basis: MeasureBasis.RISK }));
	await d.mount();
	await React.act(async () => {
		radios()[0]?.click();
	});
	assert.doesNotMatch(bodyText(), /一併撤銷/u);
});

test("裁定解除請求：標題與選項不同；駁回時提示會計入冷卻，接受時沒有", async () => {
	const d = openDecide("remediation", measure({}));
	await d.mount();
	assert.match(bodyText(), /裁定解除請求/u);
	assert.match(bodyText(), /接受（結束這個措施）/u);
	assert.doesNotMatch(bodyText(), /計入冷卻/u);
	await React.act(async () => {
		radios()[0]?.click();
	});
	assert.doesNotMatch(bodyText(), /計入冷卻/u);
	await React.act(async () => {
		radios()[1]?.click();
	});
	assert.match(bodyText(), /駁回會計入冷卻，賣家可能要等一段時間才能再請求/u);
});

test("裁定：送出的內容帶結果、理由與一併撤銷的勾選；失敗時保留輸入", async () => {
	const sent: unknown[] = [];
	let fail = true;
	const d = openDecide(
		"appeal",
		measure({
			basis: MeasureBasis.VIOLATION,
			canRevokeFindings: true,
			findings: [create(ViolationSchema, { id: "f1", originMeasureId: "m1" })],
		}),
		async (form) => {
			sent.push(form);
			if (fail) throw new Error(MEASURE_CONFLICT_TEXT);
		},
	);
	await d.mount();
	await React.act(async () => {
		radios()[0]?.click();
	});
	await React.act(async () => {
		setValue(byId("decide-measure-request-reason"), "接受申覆");
		(
			dom.window.document.querySelector(
				'input[type="checkbox"]',
			) as HTMLInputElement
		).click();
	});
	await click("確認裁定");
	assert.deepEqual(sent, [
		{ accept: true, reason: "接受申覆", revokeFindings: true },
	]);
	assert.match(bodyText(), new RegExp(MEASURE_CONFLICT_TEXT, "u"));
	assert.equal(
		byId<HTMLTextAreaElement>("decide-measure-request-reason").value,
		"接受申覆",
	);
	fail = false;
	await click("確認裁定");
	assert.equal(sent.length, 2);
});

// ---------------------------------------------------------------------------
// 內部註記

test("內部註記：標題、說明、placeholder、按鈕；沒有註記時顯示「還沒有註記」", async () => {
	const m = mount(
		React.createElement(NotesSection, { measureId: "m1" }),
		transportWith({ notes: [] }),
	);
	await m.mount();
	await settle(m);
	assert.match(bodyText(), /內部註記/u);
	assert.match(
		bodyText(),
		/只有後台看得到。送出後不能修改或刪除，要更正請再寫一則。不要寫通報人的聯絡方式。/u,
	);
	assert.match(bodyText(), /還沒有註記/u);
	assert.equal(
		byId<HTMLTextAreaElement>("measure-note-body").placeholder,
		"寫下調查經過或判斷依據（最多 2000 字）",
	);
	assert.ok(buttonByText("新增註記"));
});

test("內部註記：每則顯示「{admin ID}・{時間}」與內容；空白註記不送出，送出的內容去掉前後空白", async () => {
	created.length = 0;
	const m = mount(
		React.createElement(NotesSection, { measureId: "m1" }),
		transportWith({
			notes: [
				create(GovernanceNoteSchema, {
					id: "n1",
					body: "第一則",
					createdByAdminId: "admin-7",
					createTime: timestampFromDate(new Date("2026-10-16T02:30:00Z")),
				}),
			],
		}),
	);
	await m.mount();
	await settle(m);
	assert.match(bodyText(), /admin-7・2026\/10\/16 10:30/u);
	assert.match(bodyText(), /第一則/u);
	assert.doesNotMatch(bodyText(), /還沒有註記/u);

	await click("新增註記");
	assert.match(bodyText(), /請填寫內部註記。/u);
	assert.deepEqual(created, []);

	await React.act(async () => {
		setValue(byId("measure-note-body"), "  新的調查經過  ");
	});
	await click("新增註記");
	await settle(m);
	assert.deepEqual(created, ["新的調查經過"]);
	assert.equal(byId<HTMLTextAreaElement>("measure-note-body").value, "");
});
