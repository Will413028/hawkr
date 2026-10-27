import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import type { ApiEnvelope } from "../lib/api/client";
import type { RepoFetcher } from "../lib/api/delegated";
import { createSellerProductRepo } from "../lib/products/seller-repo";
import { createDelegatedProductRepo } from "../lib/seller-data/delegated-repos";

/**
 * 商品上下架：**同一次載入內的第二次寫入必須帶新版本**（56 號 §3.4 樂觀鎖）。
 *
 * 與商店設定那一支（`delegated-storefront-version-echo.test.ts`）是同一個 root cause
 * 的另一半。商品列表在一次載入內可以對**同一個商品**連按上架、下架，而
 * `/publish`／`/unpublish` 每成功一次就把版本推進一格（Gateway `delegatedMutationData`
 * 的註解：「the NEW version after the write — the front end feeds it straight back as
 * the next expectedVersion」）。原本資料層回 `void`、清單只 patch `status` 不 patch
 * `version`，於是第二次送出帶的仍是頁面剛載入時那個版本 ⇒ **必 9266「資料已被更新」**，
 * 畫面接著彈出版本衝突面板叫人重新載入 —— 而現場只有代行者一個人在改。
 *
 * ⚠️ **不得改用「背景 refetch 回來就把版本抄進本地狀態」的一行修法**：那等於把賣家
 * 這段期間的修改當成自己的版本，送出時 `expectedVersion` 相符、回 `code: 0`，樂觀鎖
 * 從此永遠不會擋 —— 拿「一個會被擋下來的錯誤」換「一次無聲的覆寫」。所以本檔第二節
 * 反向釘住版本的來源只能是**這一次寫入的回應**。
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const STORE_ID = "2685914682";
const META = { operationReason: "協助賣家調整上架狀態", notifySeller: true };

const PRODUCTS_PAGE = "features/seller-center/products/ProductsPage.tsx";

function read(file: string): string {
	return readFileSync(join(ROOT, file), "utf8");
}

/** 依序吐出多筆 mutation envelope 的假 fetcher；記下每一次送出的路徑與 body。 */
function scriptedFetcher(responses: unknown[]) {
	const sent: Array<{ path: string; json?: Record<string, unknown> }> = [];
	let index = 0;
	const fetcher = (async <T>(path: string, opts: { json?: unknown } = {}) => {
		sent.push({ path, json: opts.json as Record<string, unknown> | undefined });
		const data = responses[index] ?? {};
		index += 1;
		return { code: 0, data: data as T } satisfies ApiEnvelope<T>;
	}) as RepoFetcher;
	return { sent, fetcher };
}

/* ============================================================
   一、資料層：上下架要把新版本交還給呼叫端
   ============================================================ */

test("商品：連續兩次上下架時，第二次可以帶著第一次回應的版本送出", async () => {
	const { sent, fetcher } = scriptedFetcher([
		{ auditLogId: "9101", version: "13", resource: { version: "13" } },
		{ auditLogId: "9102", version: "14", resource: { version: "14" } },
	]);
	const repo = createDelegatedProductRepo(fetcher, STORE_ID);

	// 上架 → 立刻改變主意下架，中間沒有重新載入列表（這正是真實動線）。
	const published = await repo.updateProductStatus("905", "4004", {
		...META,
		expectedVersion: "12",
	});
	// 呼叫端手上唯一的新版本來源就是上一行的回應 —— 這正是原本被丟掉的東西。
	assert.equal(published.version, "13");

	await repo.updateProductStatus("905", "4003", {
		...META,
		expectedVersion: published.version,
	});

	assert.deepEqual(
		sent.map((call) => call.path),
		[
			`/products/${STORE_ID}/905/publish`,
			`/products/${STORE_ID}/905/unpublish`,
		],
	);
	assert.equal(sent[0]?.json?.expectedVersion, "12");
	// 送 12 的話後端一定回 9266：上架那一次已經把它推進到 13 了。
	assert.equal(sent[1]?.json?.expectedVersion, "13");
});

test("商品：回應沒有 version 時不憑空生一個出來", async () => {
	const { fetcher } = scriptedFetcher([{ auditLogId: "9103", resource: {} }]);
	const repo = createDelegatedProductRepo(fetcher, STORE_ID);

	const written = await repo.updateProductStatus("905", "4004", META);
	// `undefined` 會讓 `mutationBody` 整個欄位不送；退成 `""` 反而會送出一個後端必拒
	// 的空版本。
	assert.equal(written.version, undefined);
});

test("賣家自助路徑：請求形狀與回傳語意都不因為新簽章而改變", async () => {
	const { sent, fetcher } = scriptedFetcher([{}]);
	const repo = createSellerProductRepo(fetcher);

	const written = await repo.updateProductStatus("2698", "4004");

	// 仍然是「傳 status 值」的單一端點，body 一格未動。
	assert.deepEqual(sent, [
		{ path: "/app/products/2698/status", json: { status: "4004" } },
	]);
	// 賣家端沒有樂觀鎖，`version` 必須留空 —— 生一個出來會讓呼叫端把它寫進清單，
	// 而那個值對 `/app/products` 沒有任何意義。
	assert.equal(written.version, undefined);
});

/* ============================================================
   二、呼叫端接線：版本只准從「自己這一次寫入的回應」更新
   ============================================================ */

/**
 * 這個 repo 沒有 DOM 測試環境（`vitest.config.ts` 是 `environment: "node"`，也沒有
 * jsdom／testing-library），連續兩次上下架無法真的點兩下驗證。所以接線這一段沿用
 * 姊妹檔的原始碼掃描形式 —— 它擋不住邏輯寫錯，但擋得住「整條接線被拿掉」與「有人把
 * 版本改回從背景 refetch 抄」這兩種讓 bug 原樣長回來的改法。
 */

/** 抓出 `patchStatusLocal` 的**程式碼**（註解剝掉，理由同姊妹檔：說明文字會打到述詞）。 */
function patchStatusLocalCode(source: string): string {
	const start = source.indexOf("const patchStatusLocal = React.useCallback(");
	assert.notEqual(
		start,
		-1,
		"找不到 patchStatusLocal —— 它被改寫或移走了，本檔的述詞要跟著更新",
	);
	const end = source.indexOf("\t);", start);
	assert.notEqual(end, -1, "patchStatusLocal 的結尾變了，述詞要跟著更新");
	return source
		.slice(start, end)
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.replace(/\/\/.*$/gm, "");
}

test("商品列表：patchStatusLocal 必須連 version 一起 patch", () => {
	const code = patchStatusLocalCode(read(PRODUCTS_PAGE));

	// 反向自檢：確認擷取到的真的是那一段（沒抓到東西時 match 會直接失敗）。
	assert.match(code, /setItems\(/);
	assert.match(code, /setSummaryItems\(/);

	assert.match(
		code,
		/version\?: string/,
		"patchStatusLocal 不收 version ⇒ 清單留著寫入前的舊版本，第二次上下架必 9266",
	);
	assert.match(
		code,
		/\.\.\.\(version \? \{ version \} : \{\}\)/,
		"version 沒有被寫進清單項目 ⇒ 同上；且賣家路徑（undefined）必須維持不加這一欄",
	);
});

test("商品列表：版本取自本次寫入的回應，不是背景 refetch", () => {
	const source = read(PRODUCTS_PAGE);

	// ① 回傳值要接住，不能只 await。
	assert.match(
		source,
		/const written = await productRepo\.updateProductStatus\(/,
		"updateProductStatus 的回傳值被丟掉 ⇒ 第二次上下架必 9266",
	);
	// ② 接住之後要真的交給清單。
	assert.match(source, /patchStatusLocal\(id, target, written\.version\)/);

	// ③ 反向：帶版本的 patch 呼叫點只有這一處，而且來源是 `written`。
	//    多出任何一處，就要回頭確認那一處的版本是不是從 refetch／query cache 抄來的。
	const versioned =
		source.match(/patchStatusLocal\([^)]*,[^)]*,[^)]*\)/g) ?? [];
	assert.deepEqual(versioned, [
		"patchStatusLocal(id, target, written.version)",
	]);

	// ④ 這一頁的清單是自持狀態（`items`），不訂閱 seller-products query —— 只用它的
	//    invalidate。訂閱進來就等於開了「背景 refetch 回填版本」那條路。
	//    用 assert.ok 而不是 doesNotMatch：後者失敗時會把整份原始碼（近 40KB）印進
	//    測試輸出，真正的訊息反而被埋掉。
	assert.ok(
		!/\buseSellerProducts\s*\(/.test(source),
		"訂閱 seller-products query ⇒ 背景 refetch 會把賣家這段期間的修改靜默納為己方版本，樂觀鎖從此不會擋",
	);
});
