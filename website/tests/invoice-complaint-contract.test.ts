import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");

/**
 * 掃「有沒有寫某個東西」時必須先把註解拿掉：本檔要禁的東西，正好也是那些檔案的
 * 註解裡會逐字解釋「為什麼不這樣做」的東西 ⇒ 不去註解就會抓到自己的說明文字，
 * 而那是假紅，修法會變成「把註解刪掉」。
 */
const stripComments = (source: string) =>
	source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const REPO = "lib/invoices/buyer-repo.ts";
const SECTION = "features/invoices/components/InvoiceComplaintSection.tsx";
const CREATE_DIALOG = "features/invoices/screens/InvoiceComplaintDialog.tsx";
const ORDER_SECTION = "features/invoices/order/OrderInvoiceSection.tsx";

/* ── 樂觀鎖與冪等：漏了不會編譯錯、不會測試紅 ─────────────────────────── */

test("63／64／65 三支都送 expectedVersion", () => {
	const source = stripComments(read(REPO));
	for (const fn of [
		"rejectInvoiceComplaintResponse",
		"withdrawInvoiceComplaint",
		"reopenInvoiceComplaint",
	]) {
		const start = source.indexOf(`export async function ${fn}`);
		assert.ok(start >= 0, `${fn} 不存在`);
		// 🔴 邊界切在**下一個 export**，不是固定字元數：固定長度會把下一支函式的
		// body 一起吃進來，於是把這一支的欄位拿掉照樣全綠（實測過，零鑑別力）。
		const nextExport = source.indexOf("\nexport ", start + 1);
		const body = source.slice(start, nextExport > 0 ? nextExport : undefined);
		// 🔴 錨在 **json body 裡**，不是「函式內任何地方出現這個字」——後者會被
		// 參數宣告 `expectedVersion: string` 滿足，於是把它從 body 拿掉照樣全綠
		// （實測：第一版就是這樣，零鑑別力）。
		assert.match(
			body,
			/json:\s*\{[^}]*expectedVersion/,
			`${fn} 沒有把 expectedVersion 送進 body ⇒ 兩個分頁同時操作會互相覆蓋而無人察覺`,
		);
		assert.match(
			body,
			/idempotent\(/,
			`${fn} 沒有帶 Idempotency-Key ⇒ 重試會被後端當成第二次操作`,
		);
	}
});

test("expectedVersion 的來源只有 complaint.version，不得用推導值頂替", () => {
	const source = stripComments(read(SECTION));
	assert.match(source, /complaint\.version/);
	// `responses.length` 之類的推導值不是樂觀鎖：它在同一個版本內會重複，
	// 兩個分頁同時操作時後端分不出來。
	assert.doesNotMatch(source, /responses\.length,/);
});

/* ── 建案的說明是選填、63 的理由是必填，兩者相反 ───────────────────────── */

test("建案 Dialog 的送出鍵不因說明空白而 disable", () => {
	const source = stripComments(read(CREATE_DIALOG));
	// 照抄既有售後 AppealDialog 的 `text.trim().length === 0` 會讓買家送不出最
	// 常見的那個情況（「就是沒收到，沒別的好說」），而按鈕只是灰的、沒有錯誤訊息。
	assert.doesNotMatch(
		source,
		/disabled=\{[^}]*trim\(\)\.length === 0/,
		"建案的 description 是選填，送出鍵不得因空白而 disable（§B-1.0.9 B18t）",
	);
});

/* ── 買家能做的動作只有四個 ─────────────────────────────────────────── */

test("不得出現「確認已解決」按鈕——Phase 1 沒有那支端點", () => {
	for (const file of [SECTION, ORDER_SECTION]) {
		const source = stripComments(read(file));
		assert.doesNotMatch(
			source,
			/確認已解決/,
			`${file} 加了一顆會打到不存在端點的按鈕`,
		);
	}
});

test("9310 走業務碼分流，不看 HTTP status", () => {
	const source = stripComments(read(ORDER_SECTION));
	assert.match(source, /COMPLAINT_DUPLICATE/);
	// 9310 的 HTTP 是 200，照 status 分流會永遠漏接（§B-1.0.9 B05t）。
	assert.doesNotMatch(source, /\.status === 4\d\d/);
	assert.doesNotMatch(source, /\.status === 200/);
});

/* ── 零輪詢（§0.4 ④） ───────────────────────────────────────────────── */

test("申訴區塊零輪詢、不另開第二條 SSE", () => {
	for (const file of [SECTION, "lib/hooks/useInvoiceComplaint.ts"]) {
		const source = stripComments(read(file));
		for (const banned of [
			"setInterval",
			"refetchInterval",
			"useNotificationStream",
		]) {
			assert.ok(
				!source.includes(banned),
				`${file} 出現 ${banned}：發票端點與該使用者其他請求共用限流額度`,
			);
		}
	}
});

/* ── B04 的動作列：申訴（B07）＋ 修改發票資料（B06）───────────────── */

/**
 * 本條原本斷言「不得渲染修改發票資料」——更正執行面 R02–R04 當時尚未實作
 * （backend#2909）。**2026-09-01 那三支已在 `origin/dev`、票已 CLOSED** ⇒ 反轉
 * 成「必須渲染，且入口判準取後端的旗標」。
 *
 * 反轉而不是刪掉：它守的東西沒消失，只是換了方向。而**第二個斷言才是重點**
 * ——`canRequestCorrection` 涵蓋開票狀態、法定作廢窗口、是否已有進行中的申請
 * 三件事。前端若自己算（例如看有沒有發票號碼），買家會按到一顆必定失敗的鍵，
 * 而那不會編譯錯、不會有其他測試轉紅。
 */
test("B04 的動作列同時接上申訴入口與修改發票資料，且後者用後端旗標把關", () => {
	const source = stripComments(read(ORDER_SECTION));
	assert.match(
		source,
		/entryCta/,
		"B07 的入口按鈕沒有接上——B04 的動作列仍然是空的",
	);
	assert.match(source, /修改發票資料/, "B06 的入口沒有接上");
	// 🔴 判準本身走 `lib/invoices/correctionEntry.ts`（有真正的行為測試）。
	// 這裡只驗「元件確實委派給它、沒有在 JSX 裡另外算一份」——**這一條擋的是
	// 判準被複製出第二份**，那是靜默漂移的來源。
	assert.match(
		source,
		/correctionEntry\(/,
		"B06 的入口沒有委派給 correctionEntry()",
	);
	assert.doesNotMatch(
		source,
		/canRequestCorrection/,
		"元件裡又算了一次 canRequestCorrection：判準只能有一份，在 correctionEntry",
	);
	assert.doesNotMatch(
		source,
		/issueAttemptSeq/,
		"B06 送了實體軸的 issueAttemptSeq——那會在作廢重開後誤判成衝突",
	);
});

/* ── 前端零計算工作天（§0.6 C10） ───────────────────────────────────── */

test("倒數不自己算工作天，也不換算日曆天", () => {
	for (const file of [
		SECTION,
		"features/invoices/components/SlaCountdown.tsx",
		"lib/invoices/slaCountdown.ts",
	]) {
		const source = stripComments(read(file));
		for (const banned of ["86400000", "getDay()", "Date.now() -"]) {
			assert.ok(
				!source.includes(banned),
				`${file} 出現 ${banned}：工作天要扣國定假日，而假日表在後端`,
			);
		}
	}
});
