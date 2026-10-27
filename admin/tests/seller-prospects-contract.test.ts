import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

import { OutreachStatus } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { EDITABLE_OUTREACH_STATUSES } from "../src/lib/seller-prospects";

/**
 * CONVERTED 是轉換動作的衍生事實，PUT 送它後端會回 400。下拉選單的資料來源
 * 就是這個常數，所以它必須在來源就不含 CONVERTED——不是靠畫面上少畫一個選項。
 */
test("EDITABLE_OUTREACH_STATUSES 不含 CONVERTED（不可由 admin 設定）", () => {
	assert.equal(
		EDITABLE_OUTREACH_STATUSES.includes(OutreachStatus.CONVERTED),
		false,
	);
});

/**
 * `GovernanceNoticeBanner` 渲染的 `adm-authorization-notice`／`adm-operation-error`，
 * 以及治理確認框的 `adm-operation-dialog-overlay`……這些 class **只定義在**
 * `src/styles/pages/DelegatedOperations.css`（全 repo 的 `adm-*` class 只住在
 * 這一份與 `Invoice.css`，兩份都不是全域載入，不像 `globals.css`／
 * `AdminVisualRefresh.css` 那樣每頁都有）——沒有 import 它的頁面，這些 class
 * 全部是無背景、無邊框、無圖示顏色的裸文字／裸 div，但 `pnpm lint`／`pnpm test`／
 * `pnpm exec next build` 三個都抓不到：class name 是字串，沒有型別檢查，這個 repo
 * 也沒有 React Testing Library 能做渲染快照。
 *
 * 用 `readdir` 遞迴掃 `src/app/admin/seller-prospects/` 而不是寫死一份
 * 檔名清單：
 * Task 5／7／8 還會在這個目錄底下新增治理對話框（封存還原、歸因綁定、合併確認），
 * 這條線的價值就在於它要能抓到**那些還沒寫出來的檔案**，不需要每加一個新對話框
 * 就手動回來加一行清單。
 *
 * 🔴 觸發條件也不列舉 class 名單，改成從 `DelegatedOperations.css` 本身反推：
 * 第一版只手寫了 `adm-operation-dialog-overlay` 一個 class，結果同一輪
 * `[id]/page.tsx` 繞過 `GovernanceNoticeBanner`、直接手刻
 * `className="adm-authorization-notice"`，這則測試完全沒發現（那個 class
 * 不在清單裡）。清單式寫法永遠會漏掉「下一個沒被列出來的 class」；反過來從
 * CSS 檔本身抽出它定義的**所有** `.adm-*` class 名，就不需要預測 Task 5／7／8
 * 會用到哪個 class 名——CSS 檔加了新 class，這裡自動跟著涵蓋。
 *
 * `<GovernanceNoticeBanner` 仍是獨立於 class 名單的額外觸發條件：這個字串是
 * React 元件名，不是 CSS class，光靠掃 CSS 檔抓不到「哪個檔案渲染了它」——
 * 只比對開始標籤（實際渲染該元件），不是裸字 `/GovernanceNoticeBanner/`，
 * 否則會被本檔案自己這類「為什麼不用它」的說明性註解誤觸發。
 *
 * ⚠️ 誤判方向是刻意選的：只在註解裡提到某個 `adm-*` class 名的檔案，也會被
 * 要求補上這行 import（例如 `[id]/page.tsx` 註解裡就寫了
 * `adm-authorization-notice` 這個字串——該檔已經真的 import 了，所以不會轉紅，
 * 但如果哪天有檔案只在註解提到、沒有真的用到，也會被要求補一行無害的 import）。
 * 這個代價是多一行用不到的 CSS import；相反方向的代價是畫面真的沒樣式，而
 * lint／test／build 三個都抓不到——兩害相權，這裡選擇「寧可多 import」。
 */
const SELLER_PROSPECTS_ROOT = "src/app/admin/seller-prospects";
const DELEGATED_OPERATIONS_CSS_PATH =
	"src/styles/pages/DelegatedOperations.css";
const DELEGATED_OPERATIONS_CSS_IMPORT =
	"@/styles/pages/DelegatedOperations.css";

/**
 * 從 CSS 原始碼抽出它定義（或引用）的所有 `.adm-*` class 名，去重成一個 Set。
 *
 * 同一個 class 可能出現在複合選擇器裡（例如 `.adm-operation-entry
 * .adm-operation-error`、`.adm-authorization-notice > i`），所以不能只比對
 * 「選擇器開頭的第一個 class」——正則不管這個 class 出現在哪個位置，逐一抓出
 * 每一個 `.adm-xxx` token。中文註解裡若用 `.adm-audit-*` 這種萬用字元寫法
 * 說明一整組 class，正則會停在 `*` 之前、留下一個以連字號結尾的殘缺 token
 * （例如 `adm-audit-`），這種不是真正的 class 名，過濾掉。
 */
function extractAdmClassNames(css: string): Set<string> {
	const names = new Set<string>();
	for (const match of css.matchAll(/\.(adm-[a-zA-Z0-9-]+)/gu)) {
		const name = match[1];
		if (!name.endsWith("-")) names.add(name);
	}
	return names;
}

async function collectTsxFiles(dir: string): Promise<string[]> {
	const entries = await readdir(dir, { withFileTypes: true });
	const files: string[] = [];
	for (const entry of entries) {
		const full = path.join(dir, entry.name);
		if (entry.isDirectory()) {
			files.push(...(await collectTsxFiles(full)));
		} else if (entry.isFile() && entry.name.endsWith(".tsx")) {
			files.push(full);
		}
	}
	return files;
}

test("seller-prospects 底下用到治理樣式的檔案都要 import DelegatedOperations.css", async () => {
	const css = await readFile(DELEGATED_OPERATIONS_CSS_PATH, "utf8");
	const admClassNames = [...extractAdmClassNames(css)];
	assert.ok(
		admClassNames.length > 0,
		"沒抽到任何 adm-* class——正則可能壞了，這則測試會變成永遠綠燈的假保護",
	);

	const files = await collectTsxFiles(SELLER_PROSPECTS_ROOT);
	const missing: string[] = [];
	for (const file of files) {
		const source = await readFile(file, "utf8");
		const usesComponent = /<GovernanceNoticeBanner\b/u.test(source);
		const usesAdmClass = admClassNames.some((name) => source.includes(name));
		if (
			(usesComponent || usesAdmClass) &&
			!source.includes(DELEGATED_OPERATIONS_CSS_IMPORT)
		) {
			missing.push(file);
		}
	}
	assert.deepEqual(
		missing,
		[],
		`用到 GovernanceNoticeBanner 或 DelegatedOperations.css 定義的 adm-* class 卻沒 import 它: ${missing.join(", ")}`,
	);
});
