import assert from "node:assert/strict";
import test, { beforeEach } from "node:test";
import { create } from "@bufbuild/protobuf";
import { createRouterTransport } from "@connectrpc/connect";
import {
	AuthorityOrderKind,
	AuthorityOrderSchema,
	AuthorityOrderState,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import {
	CaseInquiryKind,
	CaseInquirySchema,
	CaseInquiryState,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import {
	KYCSummarySchema,
	SellerAdminService,
	StoreSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	click,
	dom,
	maybeById,
	mount,
	navigation,
	React,
	requireFromTest,
	until,
} from "./fixtures/dom-harness";
import { CASE_ID, kase, STORE_ID, T0, ts } from "./fixtures/report-fixtures";

/**
 * 待辦點進詳情頁的錨點（設計 step10e-3d-design §4.1）：目標元素在資料載入後才渲染，
 * 所以 `useScrollToHash` 在頁面資料就緒時才捲。這裡掛載真實頁面，斷言預設狀態下三種錨點的 id 都存在，
 * 且就緒後確實捲到該元素；hook 本身另測「hash 不存在不動」「只捲一次」。
 */

const { useScrollToHash } = requireFromTest(
	"../src/lib/use-scroll-to-hash",
) as typeof import("../src/lib/use-scroll-to-hash");
const CasePage = requireFromTest(
	"../src/app/admin/violation-reports/cases/[id]/page",
).default as unknown;
const OrderPage = requireFromTest("../src/app/admin/authority-orders/[id]/page")
	.default as unknown;
const MerchantPage = requireFromTest("../src/app/admin/merchants/[id]/page")
	.default as unknown;
const { useState } = requireFromTest("react") as typeof import("react");
const h = React.createElement;

/** 記下哪個元素被捲到（JSDOM 沒有 scrollIntoView）。 */
const scrolled: string[] = [];
(
	dom.window.Element.prototype as unknown as { scrollIntoView: () => void }
).scrollIntoView = function (this: Element) {
	scrolled.push(this.id);
};

beforeEach(() => {
	scrolled.length = 0;
	dom.window.location.hash = "";
});

const INQUIRY_ID = "0190b000-0000-7000-8000-0000000000a1";
const ORDER_ID = "0190b000-0000-7000-8000-0000000000d1";

test("案件詳情：詢問卡片有 inquiry-{id}，資料就緒後捲到它", async () => {
	dom.window.location.hash = `#inquiry-${INQUIRY_ID}`;
	navigation.params = { id: CASE_ID };
	const m = mount(
		h(CasePage),
		createRouterTransport(({ service }) => {
			service(SellerGovernanceAdminService, {
				getCase: () => ({ case: kase() }),
				listReports: () => ({ reports: [] }),
				listMeasures: () => ({ measures: [] }),
				listCaseNotes: () => ({ notes: [] }),
				listCaseInquiries: () => ({
					inquiries: [
						create(CaseInquirySchema, {
							id: INQUIRY_ID,
							caseId: CASE_ID,
							storeId: STORE_ID,
							kind: CaseInquiryKind.RECORDS,
							message: "請提供進貨紀錄",
							createTime: ts(T0),
							state: CaseInquiryState.OPEN,
						}),
					],
				}),
			} as never);
			service(SellerAdminService, {
				getStore: () => ({ store: { id: STORE_ID, accountId: "acc-1" } }),
			} as never);
		}),
	);
	await m.render({ settle: false });
	await until(() => !!maybeById(`inquiry-${INQUIRY_ID}`), "詢問卡片");
	await until(() => scrolled.length > 0, "捲到詢問");
	assert.deepEqual(scrolled, [`inquiry-${INQUIRY_ID}`]);
});

test("來函詳情：每個對象有 target-{position}，資料就緒後捲到它", async () => {
	dom.window.location.hash = "#target-1";
	navigation.params = { id: ORDER_ID };
	const m = mount(
		h(OrderPage),
		createRouterTransport(({ service }) => {
			service(SellerGovernanceAdminService, {
				getAuthorityOrder: () => ({
					order: create(AuthorityOrderSchema, {
						id: ORDER_ID,
						agency: "新北市衛生局",
						documentNo: "北衛食字第 1150001 號",
						receiveTime: ts(T0),
						kind: AuthorityOrderKind.SUSPEND_ACCOUNT,
						state: AuthorityOrderState.OPEN,
						etag: "o-1",
						targets: [
							{ accountId: "101", storeId: "55", storeName: "阿里山農場" },
							{ accountId: "102", storeId: "56", storeName: "玉山農場" },
						],
					}),
				}),
				listMeasures: () => ({ measures: [] }),
				listReports: () => ({ reports: [] }),
			} as never);
			service(SellerAdminService, {
				getStore: () => ({ store: { id: "55", accountId: "101" } }),
				listStores: () => ({ stores: [] }),
			} as never);
		}),
	);
	await m.render({ settle: false });
	await until(
		() => !!maybeById("target-0") && !!maybeById("target-1"),
		"兩個對象",
	);
	await until(() => scrolled.length > 0, "捲到對象");
	assert.deepEqual(scrolled, ["target-1"]);
});

test("商家詳情：預設狀態下有 #kyc，資料就緒後捲到它", async () => {
	dom.window.location.hash = "#kyc";
	navigation.params = { id: "9" };
	const m = mount(
		h(MerchantPage),
		createRouterTransport(({ service }) => {
			service(SellerAdminService, {
				getStore: () => ({
					store: create(StoreSchema, { id: "9", storeName: "阿里山農場" }),
					kyc: create(KYCSummarySchema, { submitTime: ts(T0) }),
				}),
			} as never);
		}),
	);
	await m.render({ settle: false });
	await until(() => !!maybeById("kyc"), "#kyc 區塊");
	await until(() => scrolled.length > 0, "捲到 kyc");
	assert.deepEqual(scrolled, ["kyc"]);
});

function Probe() {
	const [ready, setReady] = useState(false);
	const [, setTick] = useState(0);
	useScrollToHash(ready);
	return h(
		"div",
		null,
		h("button", { onClick: () => setReady(true) }, "就緒"),
		h("button", { onClick: () => setReady(false) }, "重讀中"),
		h("button", { onClick: () => setTick((n: number) => n + 1) }, "重繪"),
		ready ? h("div", { id: "anchor" }) : null,
	);
}

test("hook：就緒前不捲、就緒後捲一次、之後重繪或重新就緒都不再捲", async () => {
	dom.window.location.hash = "#anchor";
	const m = mount(
		h(Probe),
		createRouterTransport(() => {}),
	);
	await m.render({ settle: false });
	assert.deepEqual(scrolled, [], "資料還沒載入，不捲");
	await click("就緒");
	assert.deepEqual(scrolled, ["anchor"]);
	await click("重繪");
	await click("重讀中");
	await click("就緒");
	assert.deepEqual(scrolled, ["anchor"], "只捲一次");
});

test("hook：hash 沒有對應的元素或沒有 hash，不動", async () => {
	for (const hash of ["#missing", ""]) {
		scrolled.length = 0;
		dom.window.location.hash = hash;
		const m = mount(
			h(Probe),
			createRouterTransport(() => {}),
		);
		await m.render({ settle: false });
		await click("就緒");
		assert.deepEqual(scrolled, [], `hash=${hash || "(無)"}`);
		await m.unmount();
	}
});
