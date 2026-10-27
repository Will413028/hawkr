import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import DelegatedAreaUnauthorized from "../features/delegated-operations/components/DelegatedAreaUnauthorized";
import DelegatedCapabilityGate, {
	DelegatedCapabilityGateView,
} from "../features/delegated-operations/components/DelegatedCapabilityGate";
import { DelegatedSessionContext } from "../features/delegated-operations/state/DelegatedSessionContext";
import { withAppRouter } from "./mocks/app-router";

/**
 * 代行工作區在「權限不足」時必須誠實（56 號 §8.1.1）。
 *
 * dev 驗收（全開／唯讀／部分三組 capability 組合）找到的六個 P1 有同一個形狀：
 * **畫面替目標店家的實際狀態作證，而我們根本沒讀過。** 最貴的一個是動態列表在零
 * `posts.read` 時寫「共 0 則動態」＋「發布第一篇」CTA —— 同一家店在有權限的工作階段
 * 下是 11 則、DB 未刪。讀畫面的是平台管理員，他會據此判斷這家店有沒有在經營。
 *
 * 這支測試守三件事：
 * 1. 那個空狀態只有**一個**主人（三份手抄副本正是分岔的起點）
 * 2. 每一條具名路由在渲染賣家中心元件**之前**先過 capability 閘門
 * 3. 閘門為 deny 時 children 完全不出現（不是 disabled、不是隱藏 CTA）
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

/**
 * 「不在本次代行授權範圍」在 JSX 裡只能出現一次。
 *
 * 用 `>…<` 而不是裸字串當錨：這幾個字在註解與說明文裡到處都是（`nav-policy.ts`、
 * 本檔），只有夾在標籤之間的那一份才是真的被畫出來的副本。抄一份出去就會像
 * 訂單／商店設定／動態三條路徑那樣各自演化。
 */
test("授權空狀態的標題只在 DelegatedAreaUnauthorized 裡渲染一次", () => {
	const marker = ">不在本次代行授權範圍<";
	const offenders: string[] = [];
	for (const dir of ["app", "components", "features", "lib"]) {
		for (const file of walk(join(ROOT, dir), ROOT)) {
			if (
				file ===
				"features/delegated-operations/components/DelegatedAreaUnauthorized.tsx"
			)
				continue;
			if (readFileSync(join(ROOT, file), "utf8").includes(marker)) {
				offenders.push(file);
			}
		}
	}
	assert.deepEqual(
		offenders,
		[],
		"第二份手抄的授權空狀態＝第二套會各自演化的標準（P1-1／P1-2 的根因）",
	);
});

/**
 * 具名路由的閘門。
 *
 * 缺閘門的症狀全都是靜默的，而且**每一條都不一樣**——那正是它們沒被一起發現的原因：
 * - `posts`（缺 `posts.read`）：`共 0 則動態` ＋ 兩顆 CTA ⇒ 謊報這家店沒發過文
 * - `storefront`（缺 `storefront.read`）：`無法載入商店資料` ⇒ 謊報系統壞了
 * - `posts/new`（缺 `posts.write`）：整頁編輯器渲染、只有儲存鍵 disabled ⇒ 一個從頭
 *   到尾不可能成功的工作檯面
 *
 * ⚠️ 下面這張表是**手抄的三面**，不是完整清單 —— 它曾經漏掉商品列表，而漏掉不會讓
 * 任何東西轉紅（那一面因此裸奔到 dev 驗收才被抓到）。「每一條路由都有人擋」由
 * `delegated-route-gate-coverage.test.ts` 掃全部路由目錄雙向對拍；新增路由請去改那一份，
 * 不要往這張表補行。
 */
test("三面具名路由都在渲染賣家中心元件之前過 capability 閘門", () => {
	const faces: [string, string, string][] = [
		["posts", "posts.read", "SellerPostsClient"],
		["posts/new", "posts.write", "PostComposeClient"],
		["storefront", "storefront.read", "SellerProfileClient"],
	];
	for (const [face, capability, client] of faces) {
		const file = source(`app/(delegated)/adm/[storeId]/${face}/page.tsx`);
		assert.match(file, /<DelegatedCapabilityGate\b/u, face);
		assert.match(file, new RegExp(`capability="${capability}"`), face);
		// 閘門要**包住**元件，不是擺在它旁邊。
		assert.ok(
			file.indexOf("<DelegatedCapabilityGate") < file.indexOf(`<${client} `),
			`${face}：賣家中心元件必須在閘門之內`,
		);
	}
});

/** 已經有自己 capability 分支的兩支，改用共用元件之後不得退回手抄。 */
test("訂單頁與上架新商品頁沿用共用的授權空狀態", () => {
	for (const file of [
		"features/delegated-operations/orders/DelegatedOrdersPage.tsx",
		"features/delegated-operations/orders/DelegatedOrderDetailRoute.tsx",
		"features/delegated-operations/products/DelegatedProductCreatePage.tsx",
		"app/(delegated)/adm/[storeId]/[...area]/page.tsx",
	]) {
		assert.match(source(file), /<DelegatedAreaUnauthorized\b/u, file);
	}
});

/**
 * 閘門的行為（不是它的形狀）：deny 時 children **完全不出現**。
 *
 * 掛上去就會發查詢，而此刻每一支都必然被拒 —— 那批 FAILED 稽核列會把「有人試圖跨店
 * 讀取」這個安全訊號淹掉。而且 children 之所以危險，正是因為它們在賣家路徑上永遠有
 * 權限、根本沒有「權限不足」這個分支。
 */
test("閘門 deny 時不渲染 children，grant 時不渲染空狀態", () => {
	const child = React.createElement("p", null, "共 11 則動態");

	const denied = renderToStaticMarkup(
		React.createElement(
			DelegatedCapabilityGateView,
			{ granted: false, reason: "本次工作階段沒有核發動態讀取權限。" },
			child,
		),
	);
	assert.ok(denied.includes("不在本次代行授權範圍"), denied);
	assert.ok(
		!denied.includes("共 11 則動態"),
		"deny 卻仍渲染 children ⇒ 畫面會替目標店家作證",
	);

	const granted = renderToStaticMarkup(
		React.createElement(
			DelegatedCapabilityGateView,
			{ granted: true, reason: "本次工作階段沒有核發動態讀取權限。" },
			child,
		),
	);
	assert.ok(granted.includes("共 11 則動態"), granted);
	assert.ok(
		!granted.includes("不在本次代行授權範圍"),
		"grant 卻仍渲染空狀態 ⇒ 閘門把有權限的人也擋掉了",
	);
});

/**
 * 閘門與工作階段之間的**接線**：`granted={hasCapability(capability)}`。
 *
 * 這一條補的是一個真的存在過的覆蓋缺口：呼叫點只驗字面字串、View 只驗 grant/deny 的
 * 渲染，兩者都綠的情況下把那個運算式改成 `granted={true}`（＝閘門永遠放行、等於沒修）
 * 仍然**全套測試通過**。三個新閘門的全部價值就在那一個運算式上，所以它必須有自己的
 * 一支測試 —— 用 stub 工作階段渲染**整支** `DelegatedCapabilityGate`，不是它的 View。
 */
test("閘門把 capability 真的接到工作階段（不是恆真）", () => {
	const child = React.createElement("p", null, "共 11 則動態");
	const render = (capabilities: string[]) =>
		renderToStaticMarkup(
			withAppRouter(
				React.createElement(
					DelegatedSessionContext.Provider,
					{ value: sessionStub(capabilities) },
					React.createElement(
						DelegatedCapabilityGate,
						{
							capability: "posts.read",
							reason: "本次工作階段沒有核發動態讀取權限。",
							backSegment: "posts",
							backLabel: "返回動態管理",
						},
						child,
					),
				),
			),
		);

	// 核發了：children 出現，空狀態不出現。
	const granted = render(["posts.read", "orders.read"]);
	assert.ok(granted.includes("共 11 則動態"), granted);
	assert.ok(!granted.includes("不在本次代行授權範圍"), granted);

	// 沒核發：children 一個字都不得出現。這一格就是「把 granted 寫死成 true」會紅的地方。
	const denied = render(["orders.read"]);
	assert.ok(
		!denied.includes("共 11 則動態"),
		"閘門沒有真的看 capability ⇒ 等於沒有閘門",
	);
	assert.ok(denied.includes("不在本次代行授權範圍"), denied);
	// 出口的 storeId 取自工作階段（不是路由參數），順帶釘住這一段接線。
	assert.ok(denied.includes("/adm/2685914682/posts"), denied);

	// 反向鑑別：換一個 capability 名，同一份工作階段就該翻面 —— 防「只要清單非空就放行」。
	const otherCapability = renderToStaticMarkup(
		withAppRouter(
			React.createElement(
				DelegatedSessionContext.Provider,
				{ value: sessionStub(["posts.read"]) },
				React.createElement(
					DelegatedCapabilityGate,
					{ capability: "posts.write", reason: "沒有核發動態編輯權限。" },
					child,
				),
			),
		),
	);
	assert.ok(!otherCapability.includes("共 11 則動態"), otherCapability);
});

/** 文案必須講「沒有核發權限」，不得講成「沒有資料」或「載入失敗」。 */
test("授權空狀態畫的是授權語意，不是空資料、也不是技術故障", () => {
	const html = renderToStaticMarkup(
		withAppRouter(
			React.createElement(DelegatedAreaUnauthorized, {
				reason:
					"本次工作階段沒有核發店面讀取權限，因此看不到這家店的商店設定。",
				backHref: "/adm/2685914682/posts",
				backLabel: "返回動態管理",
			}),
		),
	);
	assert.ok(html.includes("不在本次代行授權範圍"), html);
	assert.ok(html.includes("沒有核發"), html);
	assert.ok(html.includes("/adm/2685914682/posts"), html);
	for (const lie of ["無法載入", "載入失敗", "尚未建立", "還沒有"]) {
		assert.ok(!html.includes(lie), `授權文案不得混入故障／空資料語氣：${lie}`);
	}
});

/**
 * 貼文預覽卡的作者名不得退化成 storeId。
 *
 * 缺 `storefront.read` 時 `useSellerSelf()` 整支沒有資料，作者名原本會退到
 * `scope.ownerStoreId` —— 畫面上是一串 19 位數字。它既不是名字，也無法讓操作者確認
 * 代行的是不是對的那一家店，而正確的來源一直都在手上（`GET current` 的
 * `target.storeName`）。
 */
test("貼文預覽的作者名退到工作階段的店名，不退到 storeId", () => {
	const compose = source("features/seller-center/posts/PostComposeClient.tsx");
	assert.match(
		compose,
		/delegatedSession\?\.summary\?\.target\.storeName/u,
		"缺 storefront.read 時要退到工作階段 target 的店名",
	);
	assert.doesNotMatch(
		compose,
		/sellerName \|\| scope\.ownerStoreId/u,
		"作者名退到 storeId ⇒ 畫面上是一串 19 位數字",
	);
});

/**
 * 結束摘要的「操作數」口徑必須與 ADM 一致（P1-6）。
 *
 * `operationCount` 的後端述詞是 `count(*) FILTER (WHERE result='SUCCEEDED')` —— 它
 * **只數成功**。www 曾把它標成「總操作數」：dev 上的工作階段 7494590778405027840 因此
 * 顯示「1 項操作」，而稽核表是 4 筆（3 筆 FAILED，其中 2 筆 high risk）。同一份資料，
 * ADM 標「成功操作」、www 標「總操作數」，兩邊對同一個 session 給出兩種說法。
 *
 * www 這一側**拿不到失敗筆數**（那要 `.../audit-logs?result=FAILED`，掛 requireAdmin，
 * www host 打不到），所以正解不是自己算一個總數，而是：標籤與 ADM 用同一個詞、口徑寫
 * 在旁邊、完整三態指向 ADM 的代行紀錄。
 */
test("結束摘要標「成功操作」並寫明口徑，不得再宣稱「總操作數」", () => {
	const dialog = source(
		"features/delegated-operations/components/OperationEndDialog.tsx",
	);
	// 逐 `label="…"` 取值再比對，不對整份原始碼做字串搜尋：這幾個詞在註解裡也會出現，
	// 而註解不是任何人看得到的畫面（同源教訓：grep 命中註解 ≠ 命中程式碼）。
	const labels = [...dialog.matchAll(/label="([^"]+)"/gu)].map((m) => m[1]);
	assert.ok(labels.includes("成功操作"), labels.join("／"));
	assert.ok(
		!labels.includes("總操作數"),
		"operationCount 只數成功，標成「總操作數」就是對操作者說謊",
	);
	assert.match(dialog, /只含已成功/u, "口徑要寫在畫面上，不是只寫在註解裡");
});

/**
 * 一份夠用的工作階段：閘門只讀 `hasCapability` 與 `summary.target.storeId`，
 * 其餘欄位存在即可（型別完整才擋得住日後 context 加欄位時的靜默失衡）。
 */
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

function walk(dir: string, root: string): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(dir);
	} catch {
		return out;
	}
	for (const entry of entries) {
		if (entry === "node_modules" || entry.startsWith(".")) continue;
		const full = join(dir, entry);
		if (statSync(full).isDirectory()) out.push(...walk(full, root));
		else if (/\.tsx?$/.test(entry)) out.push(relative(root, full));
	}
	return out;
}
