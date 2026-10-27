import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test, { afterEach } from "node:test";
import { Code, ConnectError } from "@connectrpc/connect";
import { RiskGround } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { JSDOM } from "jsdom";

/**
 * 建立風險措施的對話框（CreateRiskMeasureDialog）實際掛進 DOM 後的行為：
 * 同一份內容失敗後重送，request_id 不變（伺服器才認得是同一個請求）；改過內容換新的；關閉再開啟是新的對話框、新的 request_id。
 * 以 jsdom 掛載真元件並操作輸入與按鈕，不是掃原始碼。
 */

const dom = new JSDOM("<!doctype html><body></body>", {
	url: "http://localhost/",
	pretendToBeVisual: true,
});
const g = globalThis as Record<string, unknown>;
g.window = dom.window;
g.document = dom.window.document;
g.HTMLElement = dom.window.HTMLElement;
g.Node = dom.window.Node;
g.IS_REACT_ACT_ENVIRONMENT = true;

const requireFromTest = createRequire(__filename);
requireFromTest.extensions[".css"] = () => {};

const React = requireFromTest("react") as {
	createElement: (type: unknown, props?: unknown) => unknown;
	act: (fn: () => void | Promise<void>) => Promise<void>;
};
const { createRoot } = requireFromTest("react-dom/client") as {
	createRoot: (el: Element) => {
		render: (node: unknown) => void;
		unmount: () => void;
	};
};
const Dialog = requireFromTest(
	"../src/app/admin/merchants/[id]/CreateRiskMeasureDialog",
).default as unknown;

type Request = {
	sourceCaseId?: string;
	requestId?: string;
	statement?: string;
	storeId?: string;
	measure?: {
		case?: string;
		value?: {
			basis?: { case?: string; value?: { ground?: number } };
			reviewBy?: unknown;
		};
	};
};

function setValue(el: Element, value: string) {
	const proto =
		el instanceof dom.window.HTMLTextAreaElement
			? dom.window.HTMLTextAreaElement.prototype
			: el instanceof dom.window.HTMLSelectElement
				? dom.window.HTMLSelectElement.prototype
				: dom.window.HTMLInputElement.prototype;
	const setter = Object.getOwnPropertyDescriptor(proto, "value")?.set;
	setter?.call(el, value);
	const type = el instanceof dom.window.HTMLSelectElement ? "change" : "input";
	el.dispatchEvent(new dom.window.Event(type, { bubbles: true }));
}

function byId(id: string): Element {
	const el = dom.window.document.getElementById(id);
	assert.ok(el, `找不到 #${id}`);
	return el;
}

async function fill(statement: string) {
	await React.act(async () => {
		setValue(byId("create-measure-ground"), String(RiskGround.FRAUD));
		setValue(byId("create-measure-statement"), statement);
		setValue(byId("create-measure-note"), "客服轉來三筆檢舉");
	});
}

async function confirm() {
	const button = [...dom.window.document.querySelectorAll("button")].find(
		(b) => b.textContent === "確認建立",
	);
	assert.ok(button, "找不到確認建立");
	await React.act(async () => {
		button.click();
	});
}

/** 還沒卸載的 root；斷言失敗時由 afterEach 卸載，失敗才會停在那一條斷言而不是整檔逾時。 */
const openRoots = new Set<() => Promise<void>>();
afterEach(async () => {
	for (const unmount of [...openRoots]) await unmount();
	openRoots.clear();
});

/** 開啟一個新的對話框（等於頁面把它掛載），回傳它收到的請求與卸載函式。 */
function open(
	outcomes: Array<"fail" | "ok">,
	props: Record<string, unknown> = {},
) {
	const requests: Request[] = [];
	const host = dom.window.document.createElement("div");
	dom.window.document.body.appendChild(host);
	const root = createRoot(host);
	let mounted = true;
	const unmount = () =>
		React.act(async () => {
			if (!mounted) return;
			mounted = false;
			root.unmount();
			host.remove();
		});
	openRoots.add(unmount);
	const onConfirm = async (request: Request) => {
		requests.push(request);
		const outcome = outcomes.shift() ?? "ok";
		if (outcome === "fail") {
			throw new ConnectError("unavailable", Code.Unavailable);
		}
	};
	return {
		requests,
		mount: () =>
			React.act(async () => {
				root.render(
					React.createElement(Dialog, {
						storeId: "100",
						onClose: () => {},
						onConfirm,
						...props,
					}),
				);
			}),
		unmount,
	};
}

test("同一份內容失敗後重送：request_id 不變；改過內容再送：換新的", async () => {
	const d = open(["fail", "fail", "ok"]);
	await d.mount();
	await fill("疑似詐欺");
	await confirm();
	await confirm();
	assert.equal(d.requests.length, 2);
	assert.ok(d.requests[0]?.requestId, "第一次就帶 request_id");
	assert.equal(
		d.requests[1]?.requestId,
		d.requests[0]?.requestId,
		"同一份內容的重送必須沿用同一個 request_id",
	);
	assert.match(
		dom.window.document.body.textContent ?? "",
		/建立失敗|系統|不可用|unavailable/u,
		"失敗就地顯示，對話框留在原地",
	);

	await React.act(async () => {
		setValue(byId("create-measure-statement"), "疑似詐欺，改過的理由");
	});
	await confirm();
	assert.equal(d.requests.length, 3);
	assert.notEqual(
		d.requests[2]?.requestId,
		d.requests[0]?.requestId,
		"內容改過必須換新的 request_id",
	);
	assert.equal(d.requests[2]?.statement, "疑似詐欺，改過的理由");
	await d.unmount();
});

test("關閉再開啟：新的對話框產生新的 request_id（即使內容相同）", async () => {
	const first = open(["fail"]);
	await first.mount();
	await fill("疑似詐欺");
	await confirm();
	await first.unmount();

	const second = open(["ok"]);
	await second.mount();
	await fill("疑似詐欺");
	await confirm();
	assert.ok(first.requests[0]?.requestId);
	assert.ok(second.requests[0]?.requestId);
	assert.notEqual(
		second.requests[0]?.requestId,
		first.requests[0]?.requestId,
		"重新開啟必須是新的 request_id",
	);
	await second.unmount();
});

test("驗證沒過不送出，也不消耗 request_id：補完欄位後送出的是第一個請求", async () => {
	const d = open(["ok"]);
	await d.mount();
	await confirm();
	assert.equal(d.requests.length, 0, "沒選風險類型、沒填理由，不得送出");
	assert.match(dom.window.document.body.textContent ?? "", /請選擇風險類型/u);
	await fill("疑似詐欺");
	await confirm();
	assert.equal(d.requests.length, 1);
	assert.equal(d.requests[0]?.storeId, "100");
	assert.equal(
		d.requests[0]?.measure?.case,
		"suspension",
		"風險措施是停權輸入",
	);
	assert.equal(d.requests[0]?.measure?.value?.basis?.case, "risk");
	assert.equal(
		d.requests[0]?.measure?.value?.basis?.value?.ground,
		RiskGround.FRAUD,
	);
	assert.ok(d.requests[0]?.measure?.value?.reviewBy, "review_by 在停權輸入裡");
	await d.unmount();
});

test("從案件開啟：頂端說明措施會記為這個案件導出的措施，請求帶 source_case_id；不是從案件開啟時都沒有", async () => {
	const fromCase = open(["ok"], { sourceCaseId: "case-77" });
	await fromCase.mount();
	assert.match(
		dom.window.document.body.textContent ?? "",
		/這個措施會記為案件 case-77 導出的措施。/u,
	);
	await fill("疑似詐欺");
	await confirm();
	assert.equal(fromCase.requests[0]?.sourceCaseId, "case-77");
	await fromCase.unmount();

	const plain = open(["ok"]);
	await plain.mount();
	assert.doesNotMatch(
		dom.window.document.body.textContent ?? "",
		/導出的措施/u,
	);
	await fill("疑似詐欺");
	await confirm();
	assert.equal("sourceCaseId" in (plain.requests[0] ?? {}), false);
	await plain.unmount();
});
