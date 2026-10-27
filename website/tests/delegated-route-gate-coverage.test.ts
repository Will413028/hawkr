import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, sep } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { DELEGATED_AREA_CAPABILITY } from "../features/delegated-operations/nav-policy";

/**
 * 代行工作區的**每一條**路由都要有人守著缺 capability 的那個分支（56 號 §8.1.1 層 1）。
 *
 * 這一支存在的理由，是商品列表那個洞為什麼一路沒被發現：既有的檢查全是**手抄清單**
 * ——`delegated-authorization-honesty.test.ts` 逐條列了三面、側欄只維護禁區排除表、
 * `nav-policy` 只答「可不可操作」不答「誰在擋」。三份清單沒有一份宣稱自己是完整的，
 * 於是新開一條路由不會讓任何東西轉紅，而缺閘門的症狀全是靜默的：畫面照常渲染，把
 * 「查不到」畫成「沒有」，替目標店家的經營狀態作證。
 *
 * 所以這裡不列「要檢查哪幾條」，而是**掃出全部路由再逐條要求交代**：
 * 表以外多一條或少一條都轉紅，任何新模組上線的那一刻就必須選一種歸屬。
 *
 * ⚠️ 掃描邊界是 `[storeId]` 之下：那裡的每一頁都在工作階段內、都替某一家店畫東西。
 * `adm/unavailable` 在兌換失敗時才出現、此時根本沒有工作階段，不在本表範圍。
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const WORKSPACE = "app/(delegated)/adm/[storeId]";

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

/**
 * 一條路由怎麼交代缺 capability 的分支。四種歸屬，**沒有第五種**。
 *
 * - `page`：路由層掛 `DelegatedCapabilityGate`。賣家中心的雙寫元件只能用這種 ——
 *   它們在賣家路徑上永遠有權限、根本沒有「權限不足」這個分支。
 * - `component`：代行專屬元件，在送出任何請求**之前**就以 capability 判斷並落空狀態。
 * - `postHoc`：先送一支請求、被拒之後才落空狀態。畫面誠實，但多一支必然被拒的請求。
 *   **這一類是凍結清單，只能減不能加**（下面另有一條測試守它）。
 * - `exempt`：這條路由根本不承載賣家中心的內容。每一項都要附**機械可驗的理由**，
 *   而不是一句「這個沒關係」—— 理由不成立時要跟著轉紅，否則白名單就是永久豁免權。
 */
type Coverage =
	| { kind: "page"; capability: string; client: string }
	| { kind: "component"; capability: string; files: string[] }
	| { kind: "postHoc"; capability: string; file: string; marker: string }
	| { kind: "exempt"; why: string; proof: [string, RegExp][] };

const ORDERS_GATE = [
	"features/delegated-operations/orders/useDelegatedOrders.ts",
];

const COVERAGE: Record<string, Coverage> = {
	// 工作區根（`.`）不在本表：它已無 page.tsx。「沒有獨立落地頁」這件事現在由
	// next.config.ts 的 redirects() 在 HTTP 層表達（307 → /products），而純轉址
	// 不進渲染管線，也就沒有「缺 capability 的分支」要交代。守它的是
	// tests/delegated-workspace-faces.test.ts 的「導向在 next.config」那條。

	// 它**就是**那個空狀態頁本身，沒有東西需要被擋。
	"[...area]": {
		kind: "exempt",
		why: "catch-all 本身就是「本期未開放」的空狀態頁",
		proof: [
			[
				"app/(delegated)/adm/[storeId]/[...area]/page.tsx",
				/<DelegatedAreaUnauthorized\b/u,
			],
		],
	},

	/**
	 * 總覽刻意不由單一 capability 決定（`nav-policy` 的 `null`）：它是各 read
	 * capability 的彙總畫面，逐區塊各自判斷比一道總閘門更細。包一道 gate 反而會把
	 * 「只核發商品權限」這種合法工作階段整頁擋掉。
	 */
	dashboard: {
		kind: "exempt",
		why: "彙總畫面，逐區塊各自判 capability，且三項全缺時有整頁分支",
		proof: [
			[
				"features/delegated-operations/dashboard/DelegatedDashboardPage.tsx",
				/hasCapability\("orders\.read"\)/u,
			],
			[
				"features/delegated-operations/dashboard/DelegatedDashboardPage.tsx",
				/hasCapability\("products\.read"\)/u,
			],
			[
				"features/delegated-operations/dashboard/DelegatedDashboardPage.tsx",
				/hasCapability\("posts\.read"\)/u,
			],
			[
				"features/delegated-operations/dashboard/DelegatedDashboardPage.tsx",
				/本次代行沒有可彙總的資料/u,
			],
		],
	},

	orders: {
		kind: "component",
		capability: "orders.read",
		files: [
			"features/delegated-operations/orders/DelegatedOrdersPage.tsx",
			...ORDERS_GATE,
		],
	},
	"orders/[orderNo]": {
		kind: "component",
		capability: "orders.read",
		files: [
			"features/delegated-operations/orders/DelegatedOrderDetailRoute.tsx",
			...ORDERS_GATE,
		],
	},

	posts: {
		kind: "page",
		capability: "posts.read",
		client: "SellerPostsClient",
	},
	"posts/new": {
		kind: "page",
		capability: "posts.write",
		client: "PostComposeClient",
	},

	products: {
		kind: "page",
		capability: "products.read",
		client: "ProductsPage",
	},
	"products/new": {
		kind: "component",
		capability: "products.write",
		files: [
			"features/delegated-operations/products/DelegatedProductCreatePage.tsx",
		],
	},
	"products/[productId]/preview": {
		kind: "component",
		capability: "products.read",
		files: [
			"features/delegated-operations/products/DelegatedProductPreviewPage.tsx",
		],
	},
	/**
	 * 編輯頁只在 detail 被拒之後才落空狀態（`loadFailure === "denied"`）。畫面文案
	 * 是對的，代價是每次進頁多送一支必然被拒的請求。它撐得住的理由是後端依賴閉包：
	 * `products.write` 必然伴隨 `products.read`，所以「能走到編輯頁卻讀不到」是邊角。
	 */
	"products/[productId]/edit": {
		kind: "postHoc",
		capability: "products.read",
		file: "features/delegated-operations/products/DelegatedProductEditPage.tsx",
		marker: 'loadFailure === "denied"',
	},

	storefront: {
		kind: "page",
		capability: "storefront.read",
		client: "SellerProfileClient",
	},
};

/** 先送請求才擋的路由：凍結成一份名冊，只能減不能加。 */
const POST_HOC_ROUTES = ["products/[productId]/edit"];

/** 掃出 `[storeId]` 之下每一支 `page.tsx`，回傳它相對於工作區根的路由鍵。 */
function discoverRoutes(): string[] {
	const root = join(ROOT, WORKSPACE);
	const out: string[] = [];
	const walk = (dir: string) => {
		for (const entry of readdirSync(dir)) {
			const full = join(dir, entry);
			if (statSync(full).isDirectory()) walk(full);
			else if (entry === "page.tsx") {
				const key = relative(root, dir).split(sep).join("/");
				out.push(key === "" ? "." : key);
			}
		}
	};
	walk(root);
	return out.sort();
}

/**
 * 表與現實雙向對齊 —— 這一條才是本檔的價值所在。
 *
 * 方向一（多）：新開一條代行路由卻沒交代它怎麼擋 ⇒ 轉紅。少了它，下一個模組會用
 * 商品列表的方式再漏一次，而且一樣不會有任何東西報錯。
 * 方向二（少）：表裡留著一條已經不存在的路由 ⇒ 轉紅，免得白名單囤積死條目。
 */
test("代行工作區的每一條路由都在本表交代過缺 capability 的分支", () => {
	const discovered = discoverRoutes();
	// 掃描器自己壞掉時要紅，不能靜靜地掃到 0 個檔然後全綠。
	// 由 12 降為 11：工作區根的轉址別名頁（`[storeId]/page.tsx`）退場，導向改由
	// next.config.ts 的 redirects() 在 HTTP 層完成，那一條不再是 App Router 路由。
	assert.ok(
		discovered.length >= 11,
		`只掃到 ${discovered.length} 條路由 —— 目錄搬家或掃描器壞了，要同步改這支測試而不是刪掉它`,
	);
	assert.deepEqual(
		discovered,
		Object.keys(COVERAGE).sort(),
		"代行路由與本表對不起來：新路由要選一種歸屬，刪掉的路由要從表裡移除",
	);
});

test("路由層閘門：capability 對得上，且真的包住賣家中心元件", () => {
	const faces = Object.entries(COVERAGE).filter(
		(entry): entry is [string, Extract<Coverage, { kind: "page" }>] =>
			entry[1].kind === "page",
	);
	assert.ok(faces.length > 0, "一條 page 層閘門都沒有 ⇒ 篩選條件寫錯了");
	for (const [route, spec] of faces) {
		const file = source(`${WORKSPACE}/${route}/page.tsx`);
		assert.match(file, /<DelegatedCapabilityGate\b/u, route);
		assert.match(file, new RegExp(`capability="${spec.capability}"`), route);
		// 擺在元件旁邊不算：閘門要包住它，deny 時 children 才會整個不出現。
		assert.ok(
			file.indexOf("<DelegatedCapabilityGate") <
				file.indexOf(`<${spec.client} `),
			`${route}：賣家中心元件必須在閘門之內`,
		);
	}
});

test("元件層閘門：路由確實接到那支元件，而元件在發請求前就判 capability", () => {
	const faces = Object.entries(COVERAGE).filter(
		(entry): entry is [string, Extract<Coverage, { kind: "component" }>] =>
			entry[1].kind === "component",
	);
	assert.ok(faces.length > 0, "一條元件層閘門都沒有 ⇒ 篩選條件寫錯了");
	for (const [route, spec] of faces) {
		const page = source(`${WORKSPACE}/${route}/page.tsx`);
		const owner = spec.files[0].replace(/\.tsx?$/, "");
		assert.ok(
			page.includes(owner.replace(/^features\//, "@/features/")),
			`${route}：路由沒有接到 ${owner}，這張表指錯了對象`,
		);
		// 元件與它的 hook 合成一份再比對：`orders` 的 capability 判斷住在 hook 裡。
		const bundle = spec.files.map(source).join("\n");
		assert.match(
			bundle,
			new RegExp(`hasCapability\\("${spec.capability.replace(".", "\\.")}"\\)`),
			`${route}：找不到 ${spec.capability} 的判斷 ⇒ 這一面沒有人在擋`,
		);
		assert.match(
			bundle,
			/<DelegatedAreaUnauthorized\b/u,
			`${route}：判了 capability 卻沒有落到共用的授權空狀態`,
		);
	}
});

/**
 * 「先送請求才擋」是一份凍結名冊。
 *
 * 它是既有現況、不是可以照抄的做法：這條路徑上每一次進頁都會產生一支必然被拒的
 * 請求。名冊長大就轉紅 —— 一個可以自由加項的白名單等於沒有白名單。
 */
test("先送請求才擋的路由是凍結名冊，只能減不能加", () => {
	const actual = Object.entries(COVERAGE)
		.filter(([, spec]) => spec.kind === "postHoc")
		.map(([route]) => route)
		.sort();
	assert.deepEqual(
		actual,
		[...POST_HOC_ROUTES].sort(),
		"又多了一條「先送必然被拒的請求、再把拒絕畫成空狀態」的路由",
	);
	for (const [route, spec] of Object.entries(COVERAGE)) {
		if (spec.kind !== "postHoc") continue;
		const file = source(spec.file);
		assert.ok(
			file.includes(spec.marker),
			`${route}：連事後補救的分支都不見了 ⇒ 這一面現在什麼都沒有在擋`,
		);
		assert.match(file, /<DelegatedAreaUnauthorized\b/u, route);
	}
});

/**
 * 白名單的每一項都綁著它的理由。
 *
 * 白名單最常見的壞法不是有人亂加，而是**當初的理由後來不成立了**（轉址頁開始畫東西、
 * 總覽改成單一 capability），而白名單不會自己知道。所以每一項都附一組機械可驗的證據，
 * 理由垮掉的那一刻就跟著轉紅。
 */
test("免閘門的路由，每一項的理由都仍然成立", () => {
	const exempt = Object.entries(COVERAGE).filter(
		(entry): entry is [string, Extract<Coverage, { kind: "exempt" }>] =>
			entry[1].kind === "exempt",
	);
	assert.ok(exempt.length > 0, "一條白名單都沒有 ⇒ 篩選條件寫錯了");
	for (const [route, spec] of exempt) {
		assert.ok(spec.why.length > 0, `${route}：白名單必須寫理由`);
		assert.ok(spec.proof.length > 0, `${route}：理由必須有機械可驗的證據`);
		for (const [file, pattern] of spec.proof) {
			assert.match(source(file), pattern, `${route}：${spec.why} —— 已不成立`);
		}
	}
	// 總覽的豁免直接綁在 nav-policy 的宣告上：改成單一 capability 就該回去掛閘門。
	assert.equal(
		DELEGATED_AREA_CAPABILITY["/seller-center/dashboard"],
		null,
		"總覽改由單一 capability 決定了 ⇒ 白名單失效，要改掛閘門",
	);
});
