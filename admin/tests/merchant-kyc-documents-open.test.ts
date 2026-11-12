import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test, { afterEach } from "node:test";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { KYCApplicationSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { KYCField } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { JSDOM } from "jsdom";
import { errorMessage } from "../src/lib/error-text";

/**
 * 公司補充文件實際掛進 DOM 後的行為：載入時不對任何 view_uri 發請求；點某一份才讀那一份（一次點擊一次請求），
 * 讀到後交給放大檢視；被拒或失敗時該列顯示目錄的錯誤文字，不交出影像。
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
	createElement: (t: unknown, p?: unknown, ...c: unknown[]) => unknown;
	act: (fn: () => void | Promise<void>) => Promise<void>;
};
const { createRoot } = requireFromTest("react-dom/client") as {
	createRoot: (el: Element) => {
		render: (node: unknown) => void;
		unmount: () => void;
	};
};
const cards = requireFromTest(
	"../src/app/admin/merchants/[id]/KycApplicationCards",
) as { KycDocumentsCard: unknown };

const URIS = [
	"/api/files/admin/seller/kyc_applications/app1/documents/0",
	"/api/files/admin/seller/kyc_applications/app1/documents/1",
];
const application = create(KYCApplicationSchema, {
	id: "app1",
	documents: URIS.map((viewUri, position) => ({
		field: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
		position,
		assetId: `asset-${position}`,
		viewUri,
	})),
});

const requests: string[] = [];
let status = 200;
g.fetch = async (url: string) => {
	requests.push(url);
	return {
		ok: status >= 200 && status < 300,
		status,
		blob: async () => new Blob(["x"]),
	};
};
(URL as unknown as { createObjectURL: () => string }).createObjectURL = () =>
	"blob:doc";
(URL as unknown as { revokeObjectURL: () => void }).revokeObjectURL = () => {};

const container = dom.window.document.body;
let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
	await React.act(async () => root?.unmount());
	root = undefined;
	requests.length = 0;
	status = 200;
});

async function mount(onPreview: (src: string, alt: string) => void) {
	root = createRoot(container);
	await React.act(async () =>
		root?.render(
			React.createElement(cards.KycDocumentsCard, { application, onPreview }),
		),
	);
}
const buttons = () =>
	Array.from(container.querySelectorAll<HTMLButtonElement>("button"));

test("載入時不對任何文件發請求，也沒有 <img>", async () => {
	await mount(() => {});
	assert.deepEqual(requests, []);
	assert.equal(container.querySelectorAll("img").length, 0);
	assert.equal(buttons().length, 2);
});

test("點一份才讀那一份：一次點擊一次請求，讀到的影像交給放大檢視；重新 render 不重複讀", async () => {
	const opened: string[] = [];
	await mount((src) => opened.push(src));
	await React.act(async () => buttons()[1]?.click());
	assert.deepEqual(requests, [`/api/bff${URIS[1]}`]);
	assert.deepEqual(opened, ["blob:doc"]);
	await React.act(async () =>
		root?.render(
			React.createElement(cards.KycDocumentsCard, {
				application,
				onPreview: () => {},
			}),
		),
	);
	assert.equal(requests.length, 1);
});

test("讀取被拒（403）：該列顯示目錄的權限文字，不交出影像", async () => {
	status = 403;
	const opened: string[] = [];
	await mount((src) => opened.push(src));
	await React.act(async () => buttons()[0]?.click());
	assert.equal(requests.length, 1);
	assert.deepEqual(opened, []);
	const expected = errorMessage(new ConnectError("", Code.PermissionDenied));
	assert.ok(expected.length > 0);
	assert.equal(
		container.querySelector('[role="alert"]')?.textContent,
		expected,
	);
	assert.equal(container.querySelectorAll("img").length, 0);
});
