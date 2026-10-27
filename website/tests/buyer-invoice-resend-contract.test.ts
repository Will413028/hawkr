import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { InvoiceDocumentStatus } from "@/lib/codemeta/enums";
import { canResendInvoiceNotification } from "@/lib/invoices/documentStatus";

/**
 * B05｜「重寄通知」（端點 71）的接線契約。
 *
 * 為什麼需要這一支：這顆鍵有兩種失效方式，**兩種都不會編譯錯、不會有其他測試轉紅**：
 *
 *   ① 元件定義著、卻沒有人渲染它（未使用的 local 元件不是錯誤）
 *      ⇒ 買家看不到入口，而檔案裡到處都是「重寄」兩個字，逐字 grep 照樣通過。
 *   ② 閘門從 `canResendInvoiceNotification` 退化成自己推導（例如「有沒有發票號碼」）
 *      ⇒ 對還沒開好的文件也渲染，按下去拿到 `9301`，而 `9301` 同時也是
 *      「文件不存在／不是你的」（Tier A）——買家看到的是一句無法對應到任何動作的話。
 *
 * 🔴 底下的可重寄集合**逐字對應 invoice-service 的 `resendableStatus`**
 * （`services/invoice-service/internal/usecase/notification_resend.go`）。改動這裡
 * 之前先去看那支函式，不要反過來讓程式碼遷就測試。
 */

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");
const stripComments = (source: string) =>
	source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const B05 = "features/invoices/screens/InvoiceDocumentDetailClient.tsx";
const BUYER_REPO = "lib/invoices/buyer-repo.ts";

test("B05 渲染重寄通知入口，且閘門是可重寄狀態表", () => {
	const source = stripComments(read(B05));

	// 錨在 **JSX 使用點**，不是那些字串出現過——見檔頭失效方式 ①。
	assert.ok(
		/<ResendNotificationCard\b/.test(source),
		"B05 定義了重寄卡卻沒有渲染它 ⇒ 買家沒有重寄入口，只能去開一張申訴",
	);
	assert.ok(
		/canResendInvoiceNotification\(summary\.status\)\s*\?\s*\(?\s*<ResendNotificationCard/.test(
			source,
		),
		"B05 的重寄入口沒有以 canResendInvoiceNotification 當閘門（見檔頭失效方式 ②）",
	);
	assert.ok(
		source.includes("useResendInvoiceNotification"),
		"B05 沒有接上端點 71 的 mutation hook",
	);
});

test("端點 71 的呼叫在 repo 層，且帶得出冪等鍵", () => {
	const repo = stripComments(read(BUYER_REPO));
	assert.ok(
		repo.includes("export async function resendBuyerInvoiceNotification"),
		"buyer-repo 沒有匯出 resendBuyerInvoiceNotification",
	);
	// `Idempotency-Key` 是 gateway 標成必填的 header；漏了會 400，而那個 400
	// 在畫面上與「後端壞了」長得一模一樣。
	assert.ok(
		/resendBuyerInvoiceNotification\([\s\S]{0,400}?idempotent\(idempotencyKey\)/.test(
			repo,
		),
		"端點 71 沒有帶 Idempotency-Key",
	);
	assert.ok(repo.includes("notification_resends"), "端點 71 的路徑不見了");
});

test("可重寄的狀態集合與 invoice-service 的 resendableStatus 一致", () => {
	const allowed = new Set([
		InvoiceDocumentStatus.ISSUED,
		InvoiceDocumentStatus.RESOLVED_EXTERNALLY,
		InvoiceDocumentStatus.VOIDED,
	]);
	// 對 CodeMeta 的**全值域**逐一比對，不是只測那三個：新增狀態時預設要是
	// fail-closed（不渲染），而只測三個正例的話新值落哪一邊都全綠。
	for (const status of Object.values(InvoiceDocumentStatus)) {
		assert.equal(
			canResendInvoiceNotification(status),
			allowed.has(status),
			`狀態 ${status} 的可重寄判定與後端 resendableStatus 不一致`,
		);
	}
});
