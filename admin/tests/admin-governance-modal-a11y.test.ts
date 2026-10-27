import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

/**
 * 商家詳情頁治理確認框的內容不變式（寫入的端點、帳號 ID、理由、狀態值）。
 *
 * 對話框的 a11y（focus trap、Escape、背景 inert、還焦、aria）由共用的 `<Dialog>` 一處保證，
 * 測試在 dialog-shared-component.test.ts；「沒有人自己畫骨架」由 dialog-skeleton-guard.test.ts 守。
 * 這支檔案只留與具體治理動作有關的斷言。
 */
const MERCHANT_DETAIL = "src/app/admin/merchants/[id]/page.tsx";
/**
 * 舊的一般管理頁（不是治理確認框）：既有存量，頁面還用 alert()，不在這條不變式的範圍；要清是另一件事。
 */
const LEGACY_PAGES = [
	"ai-styles",
	"app-versions",
	"membership-plans",
	"notification-templates",
	"holiday-calendar",
	"posts",
	"reports",
	"disputes",
];

/** 使用共用 Dialog 的治理檔案：治理確認框都在裡面。 */
async function dialogSources() {
	const entries = await readdir("src", {
		withFileTypes: true,
		recursive: true,
	});
	const files = entries
		.filter((e) => e.isFile() && e.name.endsWith(".tsx"))
		.map((e) => `${e.parentPath}/${e.name}`.replaceAll("\\", "/"));
	const out: { path: string; source: string }[] = [];
	for (const path of files) {
		const source = await readFile(path, "utf8");
		if (LEGACY_PAGES.some((page) => path.startsWith(`src/app/admin/${page}/`)))
			continue;
		if (/<Dialog\b/u.test(source)) out.push({ path, source });
	}
	assert.ok(
		out.length > 20,
		"掃到的對話框檔案太少，掃描範圍改了就要同步改這支測試",
	);
	return out;
}

/**
 * 治理動作不得再回到瀏覽器原生對話框。
 *
 * `confirm()` 放不下目標卡片、必填理由與後果說明，`alert()` 則是把後端訊息原樣
 * 蓋在畫面上、關掉就什麼都不剩——治理操作的成功與失敗都要留在頁面上讀得到。
 */
test("治理頁不得殘留原生 alert() / confirm()", async () => {
	const sources = await dialogSources();

	for (const { path, source } of sources) {
		assert.doesNotMatch(
			source,
			/\balert\(/u,
			`${path} 還有原生 alert()，成功／失敗要走站內提示（role="status" / role="alert"）`,
		);
		assert.doesNotMatch(
			source,
			/\bconfirm\(/u,
			`${path} 還有原生 confirm()，治理確認要走站內對話框`,
		);
	}
});

/**
 * 刪除賣家帳號的理由必須真的送到後端，而且帳號必須是店家的 account_id。
 *
 * IdentityAdminService.DeleteAccount 的 reason 寫進刪除申請紀錄；前端不送＝治理紀錄永遠缺這一欄。
 * 帳號 ID 取自店家（store.accountId），絕不能是 URL 上的 store_id（兩者不同，送錯會刪錯帳號）。
 * 送出前理由必填由對話框擋下（validateReason），與伺服器的上限一致。
 */
test("刪除賣家帳號走 DeleteAccount，帳號 ID 取店家的 account_id，理由一併送出", async () => {
	const page = await readFile(MERCHANT_DETAIL, "utf8");
	const dialog = await readFile(
		"src/app/admin/merchants/[id]/SellerAccountDeleteDialog.tsx",
		"utf8",
	);

	assert.match(
		page,
		/useConnectMutation\(\s*IdentityAdminService\.method\.deleteAccount/u,
		"刪除帳號要走 IdentityAdminService.DeleteAccount（連線），不得再打 REST",
	);
	assert.doesNotMatch(
		page,
		/\/admin\/accounts/u,
		"詳情頁不得再出現已刪除的 DELETE /admin/accounts 端點",
	);
	assert.match(
		page,
		/mutateAsync\(\{\s*accountId: store\.accountId,\s*reason,\s*\}\)/u,
		"DeleteAccount 的帳號 ID 必須是店家的 account_id，理由必須一併送出",
	);
	assert.match(
		dialog,
		/validateReason\(\s*reason,/u,
		"理由必填與上限要在送出前擋下（與伺服器的 1000 字上限一致）",
	);
});

/**
 * 「強制下架」送的是 4003，而不是一個不存在的狀態。
 *
 * `ProductStatus`（common-core）只有 4001/4003/4004，product-service 收到別的值
 * 一律丟 `IllegalArgumentException`。這顆按鈕過去送 `"deleted"`，等於 100% 失敗
 * 而畫面照樣宣告「商品已刪除」——沒有任何 gate 擋得住這種一路綠燈的死碼。
 */
test("強制下架送 4003，且不得再出現 deleted 這個狀態值", async () => {
	const page = await readFile(MERCHANT_DETAIL, "utf8");

	const statusPayload = page.match(/\/status`,\s*\{\s*status:\s*"([^"]*)"/u);
	assert.ok(statusPayload, "找不到商品狀態變更的請求 payload");
	assert.equal(
		statusPayload[1],
		"4003",
		`商品狀態變更送出的是 "${statusPayload[1]}"，合法值只有 4001/4003/4004`,
	);

	assert.doesNotMatch(
		page,
		/"deleted"/u,
		'"deleted" 不是合法的 ProductStatus，平台端也沒有刪除商品的端點',
	);
});

/**
 * 商家詳情頁自己不得再放對話框：確認框都在獨立元件（用共用 `<Dialog>`）。
 */
test("商家詳情頁本身沒有對話框骨架；對話框都在獨立元件", async () => {
	const page = await readFile(MERCHANT_DETAIL, "utf8");
	assert.doesNotMatch(
		page,
		/modal-overlay|adm-operation-dialog-overlay|role="dialog"/u,
	);
	assert.doesNotMatch(page, /useFocusTrap\(|useModalIsolation\(/u);
});
