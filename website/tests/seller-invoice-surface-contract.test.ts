import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
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

const APPEALS =
	"features/seller-center/invoices/appeals/SellerInvoiceAppealsPage.tsx";
const APPEAL_DETAIL =
	"features/seller-center/invoices/appeals/SellerInvoiceAppealDetailPage.tsx";
const JOB = "features/seller-center/invoices/jobs/SellerInvoiceJobPage.tsx";
const ORDER_CARD = "features/seller-center/invoices/SellerOrderInvoiceCard.tsx";
const RESPOND_DIALOG =
	"features/seller-center/invoices/appeals/InvoiceComplaintRespondDialog.tsx";
const REPO = "lib/invoices/seller-repo.ts";
const BLOCK_HOOKS = "lib/hooks/useSellerInvoices.ts";
const REMEDY_HOOKS = "lib/hooks/useSellerInvoiceRemedy.ts";
const SELLER_INVOICE_DIR = "features/seller-center/invoices";

function walk(dir: string): string[] {
	const out: string[] = [];
	for (const entry of readdirSync(path.join(root, dir))) {
		const rel = `${dir}/${entry}`;
		if (statSync(path.join(root, rel)).isDirectory()) out.push(...walk(rel));
		else if (/\.tsx?$/.test(rel)) out.push(rel);
	}
	return out;
}



/**
 * 端點 36 `RespondSellerInvoiceComplaint` 的 RPC 已於 `internal/grpc/seller_write.go`
 * 實作（2026-09-01 從 gateway handler 的呼叫點反推查證）⇒ 本條**由「禁止渲染」
 * 反轉成「必須渲染且三個 action 齊全」**。
 *
 * 為什麼反轉而不是刪掉：這一格守的東西沒有消失，只是換了方向。賣家逾期未回覆會
 * 累積違規（90 天 3 次即停權）⇒ 「回覆入口不見了」與先前「按下去是 503」的後果
 * 一樣嚴重，而兩者都不會編譯錯、不會有其他測試轉紅。
 *
 * ⚠️ **三個 action 逐一斷言**：少一個不會壞畫面，但賣家就少了一條路
 * ——尤其 `NEED_PLATFORM_HELP` 是「我自己處理不了」的唯一出口，漏掉它，
 * 處理不了的案子只能放到逾期。
 */
test("S10 渲染回覆申訴 CTA，且賣家可選的三個 action 齊全", () => {
	const source = stripComments(read(APPEAL_DETAIL));
	assert.ok(
		source.includes("回覆申訴") && source.includes("補充說明"),
		"S10 缺少回覆入口：逾期未回覆會累積違規，沒有入口等於逼賣家逾期",
	);
	assert.ok(
		source.includes("InvoiceComplaintRespondDialog"),
		"S10 沒有掛上回覆 dialog",
	);

	const dialog = stripComments(read(RESPOND_DIALOG));
	for (const action of ["RESENT", "REISSUED", "NEED_PLATFORM_HELP"]) {
		assert.ok(
			new RegExp(`value:\\s*"${action}"`).test(dialog),
			`回覆 dialog 的選項清單缺少 action「${action}」`,
		);
	}
	for (const admOnly of ["REMINDED", "DECIDED_RESOLVED", "DECIDED_VIOLATION"]) {
		assert.ok(
			!dialog.includes(`"${admOnly}"`),
			`回覆 dialog 出現只屬於平台的 action「${admOnly}」`,
		);
	}
	assert.ok(
		stripComments(read(REPO)).includes("export async function respondSeller"),
		"seller-repo 沒有匯出 respondSeller*",
	);
});

test("S08 不渲染重試 CTA（沒有 retries 端點），但**必須**有回報 CTA", () => {
	const source = stripComments(read(JOB));
	for (const forbidden of ["重新嘗試", "重試開立", "轉人工"]) {
		assert.ok(
			!source.includes(forbidden),
			`S08 出現「${forbidden}」——本頁刻意沒有這顆鍵`,
		);
	}
	assert.ok(source.includes("重試"), "S08 應保留讀取失敗時的重試鍵");

	assert.ok(
		source.includes("回報發票號碼"),
		"S08 缺少回報入口：賣家自行開立後無法告知平台，平台會重複開立",
	);
	assert.ok(
		source.includes("ReportExternalIssuanceDialog"),
		"S08 沒有掛上回報 dialog",
	);
	assert.ok(
		/<ReportExternalCard\b/.test(source),
		"S08 定義了回報卡但沒有把它渲染出來 ⇒ 賣家看不到入口",
	);
	assert.ok(
		/<ReportExternalCard[\s\S]{0,80}?\/>/.test(source) &&
			/job\.canReportExternal \? <ReportExternalCard/.test(source),
		"S08 的回報 CTA 沒有以後端的 canReportExternal 當閘門",
	);
});

/**
 * S07 是**停權賣家唯一**還走得通的回報路徑（S08 對停權 BLOCK）⇒ 缺了它，
 * 因違規停權的賣家連「我已經自己開好了」都說不出口，而那是解除停權的路徑之一。
 *
 * 🔴 同時斷言它**不掛** `isSuspended`：掛上去的症狀是「未停權時一切正常」，
 * 沒有任何測試會叫（§B-2.0 T05）。
 */
test("S07 渲染回報 CTA，且閘門是 canReportExternal 而不是停權旗標", () => {
	const source = stripComments(read(ORDER_CARD));
	assert.ok(
		source.includes("ReportExternalIssuanceDialog"),
		"S07 沒有掛上回報 dialog ⇒ 停權賣家沒有任何回報路徑",
	);
	// TypeScript 與 biome 都不會叫的失效方式（未使用的 local 元件不算錯誤）。
	assert.ok(
		/<ReportExternalIssuance\s/.test(source),
		"S07 定義了回報區塊但沒有把它渲染出來 ⇒ 停權賣家看不到入口",
	);
	assert.ok(
		/canReportExternal/.test(source),
		"S07 的回報 CTA 沒有以後端的 canReportExternal 當閘門",
	);
	assert.ok(
		!/canReportExternal[\s\S]{0,120}isSuspended/.test(source),
		"S07 的回報閘門混進了停權判斷 ⇒ 停權賣家沒有回報路徑",
	);
	assert.ok(
		stripComments(read(REPO)).includes(
			"export async function reportExternalIssuance",
		),
		"seller-repo 沒有匯出 reportExternalIssuance",
	);
});


test("停權放行類的 hook 一個 !isSuspended 都不准有", () => {
	const source = stripComments(read(REMEDY_HOOKS));
	assert.ok(
		!source.includes("isSuspended"),
		"useSellerInvoiceRemedy 出現 isSuspended ⇒ 停權賣家的救濟管道被前端關掉了",
	);
});

test("BLOCK 類的每一支 hook 都在查詢發出之前 gate", () => {
	const source = stripComments(read(BLOCK_HOOKS));
	const hooks = source.match(/export function use[A-Za-z]+\(/g) ?? [];
	const gates = source.match(/!blocked/g) ?? [];
	assert.ok(hooks.length >= 5, `BLOCK hook 數量異常：${hooks.length}`);
	assert.equal(
		gates.length,
		hooks.length,
		"BLOCK 類 hook 數與 `!blocked` 數不相等 ⇒ 有一支在停權時仍會發出查詢，" +
			"觸發 9240 的全域停權廣播（58A §B-2.3 (3) 查詢層）",
	);
});

test("S09／S10 走放行類 hook，不走 BLOCK 類", () => {
	for (const file of [APPEALS, APPEAL_DETAIL]) {
		const source = stripComments(read(file));
		assert.ok(
			source.includes("useSellerInvoiceRemedy"),
			`${file} 沒有從 useSellerInvoiceRemedy 取 hook`,
		);
		assert.ok(
			!source.includes("hooks/useSellerInvoices"),
			`${file} 從 BLOCK 類取 hook ⇒ 停權賣家看不到申訴，違規會繼續累積`,
		);
	}
});


test("賣家面不共用買家的 documentStatus 折疊表", () => {
	for (const file of walk(SELLER_INVOICE_DIR)) {
		const source = stripComments(read(file));
		assert.ok(
			!/invoices\/documentStatus/.test(source),
			`${file} import 了買家的 documentStatus ⇒ 賣家永遠看不到開立失敗`,
		);
	}
});


test("S09／S10 的期限一律走後端給的工作天，前端不自己算", () => {
	for (const file of [APPEALS, APPEAL_DETAIL]) {
		const source = stripComments(read(file));
		assert.ok(source.includes("SlaCountdown"), `${file} 沒有用 SlaCountdown`);
		for (const forbidden of [
			"taipeiDaysUntil",
			"DeadlineCountdown",
			"86400000",
		]) {
			assert.ok(
				!source.includes(forbidden),
				`${file} 出現 ${forbidden} ⇒ 前端在自己算工作天`,
			);
		}
	}
});

test("S09 送 due_at_asc，且不提供排序 UI", () => {
	assert.ok(
		stripComments(read(REPO)).includes('"due_at_asc"'),
		"seller-repo 沒有寫死 due_at_asc",
	);
	const source = stripComments(read(APPEALS));
	assert.ok(
		!/sort/i.test(source),
		"S09 出現排序相關的 UI 狀態 ⇒ 排序變成賣家可關掉的開關",
	);
});
