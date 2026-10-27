import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

/**
 * 確認抽屜的提示卡必須**指名店家**（56 號 §6.5）。
 *
 * `confirm-content.ts` 的 `targetStoreLabel` 早就寫下這條規則（「拿不到店名時不得
 * 退回『本店』」），也有兩支測試守著它 —— 但那兩支只掃**商品編輯頁**一支檔案，而
 * 其餘六處 `confirmMutation` 各自組樣板字串、根本沒呼叫它：三處寫死「本店」、三處
 * `|| "本店"`。規則存在、機制存在、守它的測試也存在，違反的地方卻一個都沒被抓到。
 *
 * 所以這一支把述詞錨在**每一則 `intent:`** 上，而不是某幾支檔案：
 * ① 任何 intent 字面值不得出現「本店」——「替別人操作」的畫面上，它既沒指出是哪
 *    一家，讀起來還像是管理員自己的店，而這是按錯店的最後一道關卡
 * ② 每一則 intent 都必須用 `targetStoreLabel` 產生的稱呼指名店家（直接呼叫，或
 *    元件頂部由它算出來的 `targetStore`）
 *
 * ⚠️ 述詞刻意錨在 `intent:` 之後的樣板字串，而不是「檔案裡出現『本店』」：
 * `PostComposeClient.tsx` 的**預覽卡署名**退不到店名時寫「本店」是刻意的（那裡畫
 * 一串 19 位數字沒有意義，`tests/delegated-authorization-honesty.test.ts` 守著它），
 * 而優惠券規則、載入失敗文案等處的「本店」也都合法。兩條規則作用域不同。
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));

/** 抽屜請求的組裝點。有它就代表這支檔案在寫確認文案。 */
const CONFIRM_CALL = /confirmMutation\(/;

/**
 * 行首的 `intent:` ＋ 緊接著的字串字面值。
 *
 * 排除 `intent: string;`（`SellerMutationRequest` 的型別宣告）與函式參數
 * （`(intent: "draft" | "publish") =>`，那個不在行首）。
 */
const INTENT_LITERAL = /^[ \t]*intent:\s*(["'`])/gm;
/** 同樣是行首的 `intent:`，但不管後面是什麼 —— 用來確認沒有漏讀的形態。 */
const INTENT_DECLARED = /^[ \t]*intent:(?!\s*string\b)/gm;

/** 指名店家的唯一合法來源：`targetStoreLabel(...)` 或它算出來的 `targetStore`。 */
const NAMES_TARGET_STORE = /\$\{[^}]*targetStore/;

interface IntentHit {
	file: string;
	line: number;
	text: string;
}

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
		else if (/\.tsx?$/.test(rel)) out.push(rel);
	}
	return out;
}

/** 組確認文案的檔案（測試檔本身不算 —— 它們會把違規字串當斷言值寫進來）。 */
function confirmFiles(): string[] {
	return ["app", "components", "features", "lib"]
		.flatMap(walk)
		.filter((path) => !/\.(test|spec|unit\.test)\.tsx?$/.test(path))
		.filter((path) =>
			CONFIRM_CALL.test(readFileSync(join(ROOT, path), "utf8")),
		);
}

/** 一支檔案裡的每一則 intent 字面值（跨行的樣板字串整段取出）。 */
function intentLiterals(file: string, src: string): IntentHit[] {
	const hits: IntentHit[] = [];
	INTENT_LITERAL.lastIndex = 0;
	let match = INTENT_LITERAL.exec(src);
	while (match) {
		const quote = match[1];
		const start = match.index + match[0].length;
		let cursor = start;
		while (cursor < src.length) {
			if (src[cursor] === "\\") {
				cursor += 2;
				continue;
			}
			if (src[cursor] === quote) break;
			cursor += 1;
		}
		hits.push({
			file,
			line: src.slice(0, match.index).split("\n").length,
			text: src.slice(start, cursor),
		});
		INTENT_LITERAL.lastIndex = cursor + 1;
		match = INTENT_LITERAL.exec(src);
	}
	return hits;
}

function allIntents(): IntentHit[] {
	return confirmFiles().flatMap((file) =>
		intentLiterals(file, readFileSync(join(ROOT, file), "utf8")),
	);
}

test("確認抽屜的 intent 不得寫「本店」", () => {
	const offenders = allIntents()
		.filter((hit) => hit.text.includes("本店"))
		.map((hit) => `${hit.file}:${hit.line}`);

	assert.deepEqual(
		offenders,
		[],
		"代行是替別人操作：「本店」既沒指出是哪一家、讀起來還像管理員自己的店，而抽屜是按錯店的最後一道關卡。改用 features/delegated-operations/confirm-content.ts 的 targetStoreLabel（拿不到店名退『店家 {id}』）。",
	);
});

test("確認抽屜的 intent 必須用 targetStoreLabel 指名店家", () => {
	const offenders = allIntents()
		.filter((hit) => !NAMES_TARGET_STORE.test(hit.text))
		.map((hit) => `${hit.file}:${hit.line}`);

	assert.deepEqual(
		offenders,
		[],
		"每一則 intent 都要指名目標店家，且稱呼一律由 targetStoreLabel 產生（直接呼叫或元件頂部算出來的 targetStore），不要各自組字串。",
	);
});

test("intent 一律寫成行內字面值，掃描器才讀得到", () => {
	for (const file of confirmFiles()) {
		const src = readFileSync(join(ROOT, file), "utf8");
		const declared = src.match(INTENT_DECLARED)?.length ?? 0;
		const literals = intentLiterals(file, src).length;
		assert.equal(
			literals,
			declared,
			`${file}：有 ${declared} 個 intent，但只讀得到 ${literals} 個字面值。intent 若改由函式或變數組出來，上面兩條規則就對它失明 —— 要這麼寫請同時擴充本檔的述詞。`,
		);
	}
});

/**
 * 掃描器本身要掃得到東西 —— 否則「零違規」是因為根本沒有檔案被讀。
 *
 * ⚠️ 這一條在 #2419 的教訓上：`grep` 命中零筆與 pattern 打錯，在輸出上長得一模一樣。
 */
test("intent 的掃描集合涵蓋六支組確認文案的檔案（鑑別力自檢）", () => {
	const files = confirmFiles();
	const intents = allIntents();

	assert.ok(
		intents.length >= 8,
		`只掃到 ${intents.length} 則 intent，述詞有誤`,
	);
	for (const expected of [
		"features/delegated-operations/products/DelegatedProductCreatePage.tsx",
		"features/delegated-operations/products/DelegatedProductEditPage.tsx",
		// 以下四支住在 seller-center／seller 之下，卻同樣被代行工作區渲染 ——
		// 違規當初能長出來，正是因為守規則的測試只掃 delegated-operations。
		"features/seller-center/products/ProductsPage.tsx",
		"features/seller-center/posts/PostComposeClient.tsx",
		"features/seller-center/profile/SellerProfileClient.tsx",
		"features/seller/components/SellerProfileBranding.tsx",
	]) {
		assert.ok(files.includes(expected), `掃描集合缺少 ${expected}`);
	}

	// 述詞要抓得到違規形狀，也不得把合法形狀誤判成違規。
	const bad = intentLiterals(
		"fake.tsx",
		`\t\t\tintent: \`以 ADM 身分替本店下架 \${n} 項商品\`,\n`,
	);
	assert.equal(bad.length, 1);
	assert.ok(bad[0].text.includes("本店"));
	assert.ok(!NAMES_TARGET_STORE.test(bad[0].text));

	const good = intentLiterals(
		"fake.tsx",
		`\t\t\tintent: \`以 ADM 身分替「\${targetStore}」下架 \${n} 項商品\`,\n`,
	);
	assert.equal(good.length, 1);
	assert.ok(!good[0].text.includes("本店"));
	assert.ok(NAMES_TARGET_STORE.test(good[0].text));
});
