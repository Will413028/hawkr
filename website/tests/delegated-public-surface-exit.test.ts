import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

/**
 * 代行工作區不得有通往**買家公開面**的出口（56 號 §2.2 out-of-scope 第 13 列）。
 *
 * 設計書承諾過一條 gate：「`app/(delegated)/**` 之下禁止 import `apiClient.public`，
 * 違反即 CI 紅」。它從來沒被寫出來，**而且那個述詞抓不到現存的破口** —— 破口不是
 * import，是商品列表那顆「預覽」鈕的 `<a href="/products/{id}">`；那個檔案叫
 * `features/seller-center/products/ProductsPage.tsx`，不在 `app/(delegated)/` 之下。
 *
 * 所以這一支換了兩個地方：
 *
 * **① 錨改成「出口」而不是「import」。** 一條通往公開面的路有兩種形態：
 *   - **transport**：`apiClient.public`（打匿名端點）——草稿在那裡查不到，失敗形狀是
 *     畫面靜默空白而不是報錯
 *   - **navigation**：公開路徑的字面值導航（`href` / `router.push` / `redirect` /
 *     `window.open` / `location.href`）——草稿連過去是 404，已上架的則把代行工作階段
 *     的上下文丟掉
 *
 * **② 作用域改成「代行 scope」而不是一個目錄。** 賣家中心與代行工作區**共用同一批
 * 元件**（`useSellerDataScope` 注入傳輸層／capability），所以代行畫面的一半根本不住在
 * `app/(delegated)/`。作用域因此是：兩個代行專屬目錄 ＋ **任何提到 scope／session
 * hook 的檔案**（＝雙寫檔案，它們同時服務兩條路徑）。
 *
 * ⚠️ 兩條規則的作用域**刻意不同**，理由不對稱：
 *   - navigation 兩者都掃 —— 公開路徑導航在雙寫檔案裡也是錯的（賣家路徑該去
 *     `/seller-center/*`，那不在禁區清單裡，所以賣家分支不會被誤判）
 *   - transport **只掃代行專屬目錄** —— 雙寫檔案的賣家分支合法使用 `apiClient.public`
 *     （例：貼文編輯器的「目前上架商品」挑選器刻意讀公開端點），而「這一行屬於哪個
 *     分支」不是靜態判得出來的。在雙寫檔案上硬套只會逼出一份 allowlist，那份清單
 *     終將變成「大家都在裡面」
 *
 * ⚠️ 禁區清單**由 `app/` 的路由群組推導**，不是手寫的：新增一條買家路由會自動加入
 * 禁區，不必有人記得回來改這裡。`(seller)`（賣家中心，雙寫檔案的合法去處）、
 * `(delegated)`（工作區自己）、`(canary)`（內部）刻意不列入。
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));

/** 代行專屬目錄：這裡的每一支檔案只為工作區存在。 */
const DELEGATED_ONLY = [
	"app/(delegated)",
	"features/delegated-operations",
] as const;

/** 雙寫檔案的識別字：用了它們就代表這支檔案會被代行工作區渲染／呼叫。 */
const DELEGATED_SCOPE_TOKENS =
	/useSellerDataScope|useDelegatedSession|useOptionalDelegatedSession/;

/**
 * 買家公開面所在的路由群組。
 *
 * `(auth)`（登入／註冊）也列入：代行者的身分是 ADM 憑證，把他導去 www 的登入頁是把
 * 兩套身分混在一起 —— 而那正是 `delegated-runtime.ts` 整個存在的理由。
 */
const PUBLIC_ROUTE_GROUPS = [
	"(public)",
	"(commerce)",
	"(content)",
	"(account)",
	"(auth)",
] as const;

/** 導航 sink。取到 `(` / `=` 為止，後面接著的第一個字串字面值就是目標。 */
const NAV_SINK =
	/(?:href\s*=|router\.(?:push|replace)\(|\bredirect\(|window\.open\(|location\.href\s*=)/g;

function walk(dir: string): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(join(ROOT, dir));
	} catch {
		return out;
	}
	for (const entry of entries) {
		if (entry === "node_modules" || entry === ".next" || entry === ".git") {
			continue;
		}
		const rel = relative(ROOT, join(ROOT, dir, entry));
		if (statSync(join(ROOT, rel)).isDirectory()) out.push(...walk(rel));
		else if (/\.(ts|tsx)$/.test(rel)) out.push(rel);
	}
	return out;
}

function isDelegatedOnly(path: string): boolean {
	return DELEGATED_ONLY.some((dir) => path.startsWith(`${dir}/`));
}

/** 禁區的第一段路徑（`/products`、`/cart`…），由 `app/` 的目錄結構推導。 */
function publicRouteRoots(): Set<string> {
	const roots = new Set<string>();
	for (const group of PUBLIC_ROUTE_GROUPS) {
		for (const entry of readdirSync(join(ROOT, "app", "[locale]", group), {
			withFileTypes: true,
		})) {
			// `@modal`＝parallel route、`(x)`＝巢狀群組、`_x`＝私有目錄：都不是 URL 段。
			if (!entry.isDirectory()) continue;
			if (/^[@(_[]/.test(entry.name)) continue;
			roots.add(`/${entry.name}`);
		}
	}
	return roots;
}

/** 代行 scope 內的檔案（測試檔本身不算 —— 它們常常要引用禁區路徑當斷言值）。 */
function delegatedScopeFiles(): string[] {
	return ["app", "features", "lib", "components"]
		.flatMap(walk)
		.filter((path) => !/\.(test|spec|unit\.test)\.tsx?$/.test(path))
		.filter(
			(path) =>
				isDelegatedOnly(path) ||
				DELEGATED_SCOPE_TOKENS.test(readFileSync(join(ROOT, path), "utf8")),
		);
}

interface NavHit {
	file: string;
	line: number;
	target: string;
}

/**
 * 一支檔案裡「導航到禁區路徑」的每一處。
 *
 * 只認**字面值開頭**（`"/products/…"`、`` `/products/${id}` ``）：以變數組出來的路徑
 * 靜態判不了，硬猜只會產生誤報。這是這支 gate 已知的天花板 —— 而現實中的破口就是
 * 字面值，因為 `/adm/…` 的那幾條反而都是變數（`delegatedProductRoot`）。
 */
function publicNavHits(file: string, roots: ReadonlySet<string>): NavHit[] {
	const text = readFileSync(join(ROOT, file), "utf8");
	const hits: NavHit[] = [];
	for (const match of text.matchAll(NAV_SINK)) {
		const index = match.index ?? 0;
		// sink 之後最近的一個字串字面值。跨行的屬性值不會出現在這裡（JSX 的
		// href 值一定與 `href=` 同行或緊接其後）。
		const literal = text.slice(index, index + 200).match(/["'`]([^"'`\n]*)/);
		if (!literal) continue;
		const target = literal[1];
		if (!target.startsWith("/")) continue;
		const segment = `/${target.slice(1).split(/[/?#]/)[0]}`;
		if (!roots.has(segment)) continue;
		hits.push({
			file,
			line: text.slice(0, index).split("\n").length,
			target,
		});
	}
	return hits;
}

test("代行 scope 之下沒有通往買家公開面的字面值導航", () => {
	const roots = publicRouteRoots();
	// 反向自檢：禁區清單推導不出東西時，下面的斷言會空過。
	assert.ok(
		roots.has("/products") && roots.has("/cart") && roots.size >= 10,
		`公開路由禁區推導失敗（${roots.size} 條），這條斷言已失去鑑別力`,
	);

	const offenders = delegatedScopeFiles().flatMap((file) =>
		publicNavHits(file, roots),
	);
	assert.deepEqual(
		offenders.map((hit) => `${hit.file}:${hit.line} → ${hit.target}`),
		[],
		"代行工作區導向買家公開面：草稿在那裡是 404，已上架的則把代行工作階段的上下文丟掉。要看買家視角請用工作區內的自繪預覽（/adm/{storeId}/products/{productId}/preview）。",
	);
});

test("代行專屬檔案不得直接呼叫公開（匿名）transport", () => {
	const offenders = delegatedScopeFiles()
		.filter(isDelegatedOnly)
		.filter((file) =>
			/apiClient\.public/.test(readFileSync(join(ROOT, file), "utf8")),
		);
	assert.deepEqual(
		offenders,
		[],
		"代行工作區的資料一律走 /adm/api/* BFF：公開端點只看得到 publish_status=4402，草稿在那裡查不到，而失敗形狀是靜默空白不是報錯。",
	);
});

/**
 * 掃描器本身要掃得到東西 —— 否則上面兩條「零命中」是因為根本沒有檔案被讀。
 *
 * ⚠️ 這一條在 #2419 的教訓上：`grep` 命中零筆與 pattern 打錯，在輸出上長得一模一樣。
 */
test("代行 scope 的檔案集合非空，且涵蓋 scope.mode 雙寫的那支檔案", () => {
	const files = delegatedScopeFiles();
	assert.ok(files.length > 20, `代行 scope 只掃到 ${files.length} 支檔案`);
	assert.ok(
		files.includes("features/seller-center/products/ProductsPage.tsx"),
		"商品列表是賣家中心與代行工作區雙寫的同一支元件，唯一的破口就出在它身上 —— 掃不到它，這支 gate 對真實破口失明",
	);
	assert.ok(
		files.some((f) => f.startsWith("app/(delegated)/")),
		"代行專屬目錄一支都沒掃到",
	);
});
