import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

/**
 * 商店設定：**同一次載入內的第二次寫入必須帶新版本**（56 號 §3.4 樂觀鎖）。
 *
 * 這一頁在一次載入內可以連續寫三種東西 —— 儲存商店資訊、換頭像、換封面 —— 而每一次
 * 成功都會把店面版本推進一格。原本三支資料層方法都回 `void`，把後端已經送回來的
 * `version` 當場丟掉，於是第二次送出帶的仍是頁面剛載入時那個版本 ⇒ **必 9266「資料
 * 已被更新」**。它的難堪之處在於現場只有代行者一個人在改：訊息指向「有人搶先改了」，
 * 實際上搶先的是他自己的上一次儲存，而且每撞一次就在稽核表留下一列 FAILED。
 *
 * ⚠️ **不得改用「背景 refetch 回來就把版本抄進表單」的一行修法。** `useSellerSelf`
 * 有 `staleTime` ＋ `refetchOnWindowFocus`：那等於在代行者切回分頁的那一刻，把賣家
 * 這段期間的修改當成自己的版本 —— 送出時 `expectedVersion` 相符、回 `code: 0`，
 * 樂觀鎖從此永遠不會擋。那是拿「一個會被擋下來的錯誤」換「一次無聲的覆寫」。
 * 所以本檔第二節反向釘住：etag 只在第一次載入、或版本衝突之後才從 refetch 採用。
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));

const PROFILE_CLIENT = "features/seller-center/profile/SellerProfileClient.tsx";
const BRANDING = "features/seller/components/SellerProfileBranding.tsx";

function read(file: string): string {
	return readFileSync(join(ROOT, file), "utf8");
}

/* ============================================================
   一、呼叫端接線：版本只准從「自己這一次寫入的回應」更新
   ============================================================ */

/**
 * 這個 repo 沒有 DOM 測試環境（`vitest.config.ts` 是 `environment: "node"`，也沒有
 * jsdom／testing-library），連續兩次儲存無法真的點兩下驗證。所以接線這一段改用
 * 這個 repo 既有的原始碼掃描形式釘住 —— 它擋不住邏輯寫錯，但擋得住「整條接線被拿掉」
 * 與「有人把版本改回從 refetch 抄」這兩種讓 bug 原樣長回來的改法。
 */

/**
 * 抓出「採用伺服器 etag」那支 effect 的**程式碼**（註解剝掉）。
 *
 * 剝註解不是為了整潔：那一段旁邊的說明文字本身就在講「etag 為什麼不能每次都抄」，
 * 不剝掉的話述詞會被自己的說明打到，變成一支恆紅、於是遲早被人放寬的測試。
 */
function etagEffectCode(source: string): string {
	const start = source.indexOf(
		"React.useEffect(() => {\n\t\tif (serverEtag && etag === null)",
	);
	assert.notEqual(
		start,
		-1,
		"找不到採用 etag 的 effect —— 它被改寫或移走了，本檔的述詞要跟著更新",
	);
	const end = source.indexOf("}, [serverEtag, etag]);", start);
	assert.notEqual(end, -1, "effect 的 deps 陣列變了，述詞要跟著更新");
	return source
		.slice(start, end)
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.replace(/\/\/.*$/gm, "");
}

test("商店設定：背景 refetch 的 etag 只在第一次載入與版本衝突之後才被採用", () => {
	const code = etagEffectCode(read(PROFILE_CLIENT));

	// 反向自檢：確認擷取到的真的是那一段。
	assert.match(code, /setEtag\(serverEtag\)/);
	assert.match(
		code,
		/serverEtag && etag === null/,
		"每次 refetch 都採用 etag ＝ 背景 refetch 會把賣家這段期間的修改靜默納為己方版本，樂觀鎖從此不會擋",
	);
	const source = read(PROFILE_CLIENT);
	assert.match(
		source,
		/adoptLatestStore = useVersionConflict\(\{[\s\S]*?await scope\.repos\.storefront\.fetchMyStore\(\)[\s\S]*?adopt: \(latest\) => \{\s*if \(latest\.store\?\.etag\) setEtag\(latest\.store\.etag\)/,
		"版本衝突後要直接讀最新版本取 etag：快取已是最新版本時 query 的 etag 不會再變",
	);
});

test("商店設定：兩條寫入路徑都用自己那一次的回應更新版本", () => {
	const source = read(PROFILE_CLIENT);

	// ① 儲存商店資訊：回傳值要接住，不能只 await。
	assert.match(
		source,
		/const written = await scope\.repos\.storefront\.updateMyStore\(/,
		"updateMyStore 的回傳值被丟掉 ⇒ 第二次儲存必版本衝突",
	);

	// ② 換頭像／換封面：版本經由 onSaved 的參數回到這一頁。
	assert.match(
		source,
		/onSaved=\{async \(written\) =>/,
		"onSaved 不收參數 ⇒ finalize 推進的那一格版本回不來",
	);

	// 兩條路徑各更新一次，一次都不能少。
	assert.equal(
		source.match(/if \(written\.etag\) setEtag\(written\.etag\)/g)?.length,
		2,
		"更新版本的呼叫點應為兩處（儲存商店資訊 ＋ 換圖 finalize）",
	);
});

test("商店預覽：finalize 的版本要往上傳給 onSaved，不是原地丟掉", () => {
	const source = read(BRANDING);

	assert.match(
		source,
		/const written = logo\s*\?\s*await scope\.repos\.storefront\.updateStoreLogo\(/,
		"finalize 的回傳值被丟掉 ⇒ 換完圖再儲存必版本衝突",
	);
	assert.match(source, /await onSaved\(written\)/);
});
