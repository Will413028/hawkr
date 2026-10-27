import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { NuqsTestingAdapter } from "nuqs/adapters/testing";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import { withAppRouter } from "../../tests/mocks/app-router";
import { DelegatedSessionContext } from "./state/DelegatedSessionContext";

/**
 * 讀取權限逐一抽掉之後，那一面**實際渲染出來**的東西。
 *
 * 與 `tests/delegated-route-gate-coverage.test.ts` 是兩件事：那一支比對原始碼，答
 * 「有沒有人在擋」；這一支真的渲染，答「擋下來之後畫面上是什麼、以及被擋的元件有沒有
 * 被實例化」。缺閘門的症狀從來不是報錯，而是畫面替目標店家的狀態作證，所以形狀對了
 * 還不夠 —— 那個運算式曾經寫成恆真而全套測試照樣綠。
 *
 * ⚠️ **這一層證明得到與證明不到的，講清楚**：`renderToStaticMarkup` 不跑 effect，
 * 所以「零請求」這句話在這裡只涵蓋兩件事 ——
 *   (a) 渲染期間沒有任何人送出請求（fetch 監聽器計數為 0）；
 *   (b) 被擋的元件**一次都沒有被實例化**（掛載計數為 0）⇒ 它註冊資料抓取的那些 effect
 *       連被排程的機會都沒有。
 * 它**不**等於「瀏覽器跑完一輪之後零請求」；那要真的掛載才驗得到，而這個 repo 的
 * vitest 環境是 node、沒有 DOM。要驗那一層得走瀏覽器層測試。
 *
 * 抽法刻意是「其餘 read capability 全給、只抽走這一個」：整份清空也會 deny，但那證明
 * 不了閘門看的是**這一個** capability（「清單非空就放行」的實作在空清單下同樣會 deny）。
 */

const READ_CAPABILITIES = [
	"orders.read",
	"products.read",
	"posts.read",
	"storefront.read",
];

const UNAUTHORIZED = "不在本次代行授權範圍";

/** 掛載計數與 sentinel 由替身元件回報：deny 時它們必須整份不出現。 */
const probe = vi.hoisted(() => ({ mounts: {} as Record<string, number> }));

function stubModule(name: string) {
	return async () => {
		const react = await import("react");
		const Component = () => {
			probe.mounts[name] = (probe.mounts[name] ?? 0) + 1;
			return react.createElement("p", null, `PROBE:${name}`);
		};
		return name === "products"
			? { ProductsPage: Component }
			: { default: Component };
	};
}

vi.mock("@/features/seller-center/products", stubModule("products"));
vi.mock(
	"@/features/seller-center/posts/SellerPostsClient",
	stubModule("posts"),
);
vi.mock(
	"@/features/seller-center/profile/SellerProfileClient",
	stubModule("storefront"),
);

import PostsRoute from "../../app/(delegated)/adm/[storeId]/posts/page";
import ProductsRoute from "../../app/(delegated)/adm/[storeId]/products/page";
import StorefrontRoute from "../../app/(delegated)/adm/[storeId]/storefront/page";
import DelegatedOrdersPage from "./orders/DelegatedOrdersPage";

/** 閘門只讀 `hasCapability` 與 `summary.target`，其餘欄位給得出型別即可。 */
function sessionStub(capabilities: string[]) {
	return {
		summary: {
			sessionId: "7494590778405027840",
			displayCode: "OPS-ABCDEFGHIJKLM",
			target: {
				storeId: "2685914682",
				sellerAccountId: "2685914682",
				storeName: "梨山盛花園",
			},
			actor: { adminId: "1", displayName: "alexis" },
			reason: { code: "SUPPORT", label: "協助賣家" },
			caseReference: null,
			capabilities,
			issuedAt: "2026-08-16T04:00:00.000Z",
			expiresAt: "2026-08-16T04:30:00.000Z",
			status: "active" as const,
			revision: 0,
		},
		state: "active" as const,
		loading: false,
		now: Date.parse("2026-08-16T04:10:00.000Z"),
		capabilities,
		hasCapability: (capability: string) => capabilities.includes(capability),
		writesAllowed: true,
		registerWriteController: () => () => undefined,
		ending: false,
		endError: null,
		endSession: () => undefined,
		returnUrl: "https://adm.example/admin/merchants/2685914682",
		auditUrl:
			"https://adm.example/admin/delegated-operations/7494590778405027840",
		admOrigin: "https://adm.example",
	};
}

/**
 * 訂單面用了 nuqs 的 query state，裸渲染缺轉接器會直接 throw。
 *
 * 轉接器把 `children` 宣告成**必填 prop**，而 `createElement` 的第三參數不算數 ——
 * 照它的型別寫要把 children 塞進 props，那又踩 `noChildrenProp`。改宣告成一般元件
 * 型別，兩邊都不用讓步。
 */
const NuqsAdapter = NuqsTestingAdapter as unknown as React.ComponentType<{
	searchParams?: string;
}>;

interface RenderResult {
	html: string;
	mounts: number;
	requests: number;
}

/** 渲染一次，順帶把「渲染期間送出的請求」與「替身被實例化幾次」量出來。 */
function render(
	node: React.ReactElement,
	capabilities: string[],
	probeName: string,
): RenderResult {
	probe.mounts[probeName] = 0;
	const spy = vi.spyOn(globalThis, "fetch");
	try {
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false } },
		});
		const html = renderToStaticMarkup(
			React.createElement(
				QueryClientProvider,
				{ client },
				React.createElement(
					NuqsAdapter,
					{ searchParams: "" },
					withAppRouter(
						React.createElement(
							DelegatedSessionContext.Provider,
							{ value: sessionStub(capabilities) as never },
							node,
						),
					),
				),
			),
		);
		return {
			html,
			mounts: probe.mounts[probeName] ?? 0,
			requests: spy.mock.calls.length,
		};
	} finally {
		spy.mockRestore();
	}
}

const without = (capability: string) =>
	READ_CAPABILITIES.filter((entry) => entry !== capability);

afterEach(() => {
	probe.mounts = {};
});

/**
 * 路由層閘門的三面：賣家中心元件是替身，所以「有沒有被實例化」量得準。
 *
 * grant 那一半不是湊數 —— 沒有它，deny 全綠也可能只是因為那一面根本渲染不出東西
 * （整支測試會變成永遠綠的空殼）。
 */
describe.each([
	{
		face: "商品管理",
		capability: "products.read",
		Route: ProductsRoute,
		probeName: "products",
	},
	{
		face: "動態管理",
		capability: "posts.read",
		Route: PostsRoute,
		probeName: "posts",
	},
	{
		face: "商店設定",
		capability: "storefront.read",
		Route: StorefrontRoute,
		probeName: "storefront",
	},
])("$face（抽掉 $capability）", ({ capability, Route, probeName }) => {
	it("落在授權空狀態，賣家中心元件一次都沒有被實例化", () => {
		const denied = render(
			React.createElement(Route),
			without(capability),
			probeName,
		);
		expect(denied.html).toContain(UNAUTHORIZED);
		expect(denied.html).not.toContain(`PROBE:${probeName}`);
		expect(denied.mounts).toBe(0);
		expect(denied.requests).toBe(0);
	});

	it("核發之後照常渲染（反向鑑別：deny 那一格不是因為這面本來就畫不出來）", () => {
		const granted = render(
			React.createElement(Route),
			READ_CAPABILITIES,
			probeName,
		);
		expect(granted.html).toContain(`PROBE:${probeName}`);
		expect(granted.mounts).toBe(1);
		expect(granted.html).not.toContain(UNAUTHORIZED);
	});
});

/**
 * 訂單面的閘門住在元件裡（`DelegatedOrdersPage` 是代行專屬元件，不是賣家中心的雙寫
 * 元件），所以這裡渲染的是真貨，量的是「工作檯面有沒有被畫出來」。
 *
 * 它的請求由 `useDelegatedOrders` 的 `enabled` 擋 —— 那個旗標把 `orders.read` 折進去，
 * 所以 deny 時查詢註冊了但不會被觸發。這一格量得到的是畫面那一半。
 */
describe("訂單管理（抽掉 orders.read）", () => {
	it("落在授權空狀態，整個訂單工作檯面都不畫", () => {
		const denied = render(
			React.createElement(DelegatedOrdersPage),
			without("orders.read"),
			"orders",
		);
		expect(denied.html).toContain(UNAUTHORIZED);
		expect(denied.html).not.toContain("訂單管理");
		expect(denied.requests).toBe(0);
	});

	it("核發之後照常渲染", () => {
		const granted = render(
			React.createElement(DelegatedOrdersPage),
			READ_CAPABILITIES,
			"orders",
		);
		expect(granted.html).toContain("訂單管理");
		expect(granted.html).not.toContain(UNAUTHORIZED);
	});
});
