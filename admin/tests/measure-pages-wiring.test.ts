import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

/**
 * 措施收件匣與詳情頁的接線（讀原始碼的結構性不變式；行為由 measure-inbox／measure-actions／measure-dialogs 測試覆蓋）。
 */

const LIST = "src/app/admin/measures/page.tsx";
const DETAIL = "src/app/admin/measures/[id]/page.tsx";
const exists = (path: string) =>
	access(path).then(
		() => true,
		() => false,
	);

test("側欄：「措施與申覆」在「賣家 / 商品」分組；「商家通報」不在內容治理", async () => {
	const sidebar = await readFile("src/components/Layout/Sidebar.tsx", "utf8");
	const measures = sidebar.indexOf('href: "/admin/measures"');
	const sellerSection = sidebar.indexOf('section: "賣家 / 商品"');
	assert.ok(
		sellerSection >= 0 && measures > sellerSection,
		"在賣家 / 商品分組內",
	);
	assert.match(sidebar, /label: "措施與申覆"/u);
	assert.match(sidebar, /label: "商家通報"/u);
	assert.doesNotMatch(sidebar, /商家違規 \/ 調查/u);
	assert.match(sidebar, /label: "內容審核佇列"/u);
});

test("收件匣：用 ListMeasures 的伺服器篩選與排序，不在 client 重排；列連到詳情頁", async () => {
	const page = await readFile(LIST, "utf8");
	assert.match(page, /SellerGovernanceAdminService\.method\.listMeasures/u);
	assert.match(page, /measureListRequest\(tab, currentToken\(pageTokens\)/u);
	assert.doesNotMatch(page, /\.sort\(|\.toSorted\(/u, "排序由伺服器決定");
	assert.match(page, /href=\{`\/admin\/measures\/\$\{m\.id\}`\}/u);
	assert.doesNotMatch(
		page,
		/逾期|超過 7 天/u,
		"逾期徽章隨 decide_due_time 在 3d",
	);
});

test("詳情頁：操作列只看 actionBar（伺服器的 can_* 與原因），不自己推導；寫入帶 etag 並在衝突時重讀", async () => {
	const page = await readFile(DETAIL, "utf8");
	assert.match(page, /const bar = actionBar\(measure\)/u);
	for (const key of [
		"bar.decideAppeal.visible",
		"bar.decideRemediation.visible",
		"bar.edit.enabled",
		"bar.end.visible",
		"bar.revoke.enabled",
		"bar.scheduleReview.visible",
	]) {
		assert.ok(page.includes(key), `操作列缺 ${key}`);
	}
	assert.doesNotMatch(
		page,
		/measure\.(canEnd|canRevoke|canExtend|canShorten|canUpdateStatement|canDecide\w+|canScheduleReview)/u,
		"頁面不直接讀 can_*，一律經 actionBar",
	);
	assert.match(
		page,
		/runAction\(\s*call,\s*\{\s*reload:\s*\(\) => detail\.refetch\(\),\s*fallback,\s*conflictText:\s*MEASURE_CONFLICT_TEXT,?\s*\}\s*,?\s*\)/u,
	);
	assert.match(page, /CreateTerminationDialog/u, "關聯終止在詳情頁建立");
	assert.doesNotMatch(page, /\balert\(|\bconfirm\(/u);
});

test("舊的措施詳情對話框、違規詳情頁、違規建立頁與舊的通報詳情頁已刪除（建立改為對話框）；兄弟頁還在", async () => {
	assert.equal(
		await exists("src/app/admin/merchants/[id]/MeasureDetailDialog.tsx"),
		false,
	);
	assert.equal(await exists("src/app/admin/violations/[id]/page.tsx"), false);
	assert.equal(await exists("src/app/admin/violations/new/page.tsx"), false);
	assert.equal(
		await exists("src/app/admin/violation-reports/[source]/[id]/page.tsx"),
		false,
	);
	assert.equal(
		await exists("src/app/admin/merchants/[id]/CreateMeasureDialog.tsx"),
		true,
	);
	for (const sibling of [
		"src/app/admin/violation-reports/page.tsx",
		"src/app/admin/violation-reports/[id]/page.tsx",
		"src/app/admin/violation-reports/cases/[id]/page.tsx",
		"src/app/admin/reports/page.tsx",
	]) {
		assert.equal(await exists(sibling), true, `${sibling} 不該被刪`);
	}
});

test("商家詳情的措施卡連到 /admin/measures/[id]，歷史顯示全部請求種類（不只申覆）", async () => {
	const history = await readFile(
		"src/app/admin/merchants/[id]/MeasureHistoryCard.tsx",
		"utf8",
	);
	assert.match(history, /href=\{`\/admin\/measures\/\$\{m\.id\}`\}/u);
	assert.match(history, /m\.requests\.map\(requestStateText\)/u);
	assert.doesNotMatch(history, /MeasureRequestKind\.APPEAL/u);
});

test("詳情頁：停用的修改／撤銷按鈕用 aria-describedby 指向各自的原因清單", async () => {
	const page = await readFile(DETAIL, "utf8");
	for (const [bar, id] of [
		["edit", "measure-edit-reasons"],
		["revoke", "measure-revoke-reasons"],
	] as const) {
		assert.match(
			page,
			new RegExp(
				`aria-describedby=\\{\\s*bar\\.${bar}\\.reasons\\.length > 0\\s*\\? "${id}"`,
				"u",
			),
			`${bar} 按鈕缺 aria-describedby`,
		);
		assert.ok(
			page.includes(`bar.${bar}.reasons, "${id}")`),
			`${id} 的清單要帶同一個 id`,
		);
	}
	const button = await readFile("src/components/UI/Button.tsx", "utf8");
	assert.match(button, /aria-describedby=\{ariaDescribedBy\}/u);
});

test("措施詳情的依據認定表顯示類別與嚴重度（經標籤對照，不顯示原始列舉）", async () => {
	const sections = await readFile(
		"src/app/admin/measures/[id]/MeasureSections.tsx",
		"utf8",
	);
	assert.match(sections, /policyCategoryText\(finding\.category\)/u);
	assert.match(sections, /violationSeverityText\(finding\.severity\)/u);
});
