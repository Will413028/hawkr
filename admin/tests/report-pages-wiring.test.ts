import assert from "node:assert/strict";
import { access, readdir, readFile } from "node:fs/promises";
import test from "node:test";

/**
 * 商家通報（/admin/violation-reports）頁面的接線（讀原始碼的結構性不變式；行為由 report-*、case-detail-page 等測試覆蓋）：
 * 路由、側欄、舊頁與舊測試已刪、對話框都登記在 a11y 清單、三個建立措施的入口都帶來源案件、頁面不走舊的 REST。
 */

const ROOT = "src/app/admin/violation-reports";
const exists = (path: string) =>
	access(path).then(
		() => true,
		() => false,
	);

async function sourceFiles(dir: string): Promise<string[]> {
	const out: string[] = [];
	for (const entry of await readdir(dir, { withFileTypes: true })) {
		const path = `${dir}/${entry.name}`;
		if (entry.isDirectory()) out.push(...(await sourceFiles(path)));
		else if (/\.(ts|tsx)$/u.test(entry.name)) out.push(path);
	}
	return out.sort();
}

test("路由：清單、通報詳情、案件詳情都在；案件詳情在靜態段 cases 底下，不會被 [id] 吃掉", async () => {
	for (const page of [
		`${ROOT}/page.tsx`,
		`${ROOT}/[id]/page.tsx`,
		`${ROOT}/cases/[id]/page.tsx`,
	]) {
		assert.ok(await exists(page), `${page} 不存在`);
	}
});

test("舊的通報詳情頁（[source]/[id]）、舊的 REST 契約測試已刪除", async () => {
	assert.equal(await exists(`${ROOT}/[source]`), false);
	assert.equal(await exists("tests/governance-wire-contract.test.ts"), false);
});

test("側欄：「商家通報」在「賣家 / 商品」分組、賣家管理 / 審核之後；內容治理分組裡沒有它；舊名稱沒了", async () => {
	const sidebar = await readFile("src/components/Layout/Sidebar.tsx", "utf8");
	const seller = sidebar.indexOf('section: "賣家 / 商品"');
	const content = sidebar.indexOf('section: "內容治理"');
	const merchants = sidebar.indexOf('href: "/admin/merchants"');
	const reports = sidebar.indexOf('href: "/admin/violation-reports"');
	assert.ok(seller > content && content >= 0, "分組順序");
	assert.ok(reports > seller, "在賣家 / 商品分組內");
	assert.ok(reports > merchants, "在賣家管理 / 審核之後");
	assert.ok(
		reports < sidebar.indexOf('href: "/admin/measures"'),
		"賣家管理 / 審核與措施與申覆之間",
	);
	assert.match(sidebar, /label: "商家通報"/u);
	assert.doesNotMatch(sidebar, /商家違規 \/ 調查/u);
});

test("每個對話框都用共用的 Dialog（a11y 由 Dialog 一處保證）", async () => {
	const dialogs = (await sourceFiles(ROOT)).filter((f) =>
		f.endsWith("Dialog.tsx"),
	);
	assert.equal(
		dialogs.length,
		8,
		"分案、設定結果、移到其他案件、結案、改嚴重度；詢問賣家、記下賣家的回覆、撤回詢問（3c-b）",
	);
	for (const dialog of dialogs) {
		assert.match(
			await readFile(dialog, "utf8"),
			/<Dialog\b/u,
			`${dialog} 沒有用共用的 Dialog`,
		);
	}
});

test("通報與案件的頁面本身沒有 overlay（對話框都在獨立元件）", async () => {
	for (const file of [
		`${ROOT}/[id]/page.tsx`,
		`${ROOT}/cases/[id]/page.tsx`,
		`${ROOT}/page.tsx`,
	]) {
		const source = await readFile(file, "utf8");
		assert.doesNotMatch(
			source,
			/modal-overlay|useFocusTrap\(|useModalIsolation\(/u,
			file,
		);
	}
});

test("案件詳情的三個建立措施入口都帶來源案件；建立措施（違規依據）另帶案件的類別與預先勾選的來源通報", async () => {
	const page = await readFile(`${ROOT}/cases/[id]/page.tsx`, "utf8");
	for (const dialog of [
		"CreateMeasureDialog",
		"CreateRiskMeasureDialog",
		"CreateTerminationDialog",
	]) {
		const element = page.match(
			new RegExp(`<${dialog}\\b[\\s\\S]*?/>`, "u"),
		)?.[0];
		assert.ok(element, `找不到 <${dialog}`);
		assert.match(
			element,
			/sourceCaseId=\{kase\.id\}/u,
			`${dialog} 沒有帶 sourceCaseId`,
		);
		assert.match(
			element,
			/storeId=\{kase\.storeId\}/u,
			`${dialog} 的店家不是案件的店家`,
		);
	}
	assert.match(page, /category: kase\.category/u);
	assert.match(page, /sourceReportIds: reports\.sourceReportIds/u);
});

test("按鈕只看伺服器的 can_*：案件與通報頁沒有自己推導可用性（不讀狀態來決定按鈕）", async () => {
	const caseSource = await readFile(`${ROOT}/cases/[id]/page.tsx`, "utf8");
	for (const flag of [
		"canDecide",
		"canCreateMeasure",
		"canLinkMeasure",
		"canChangeSeverity",
	]) {
		assert.ok(
			caseSource.includes(`kase.${flag}`),
			`案件頁沒有依 ${flag} 顯示按鈕`,
		);
	}
	const reportSource = await readFile(`${ROOT}/[id]/page.tsx`, "utf8");
	for (const flag of ["canTriage", "canResolve", "canMove"]) {
		assert.ok(
			reportSource.includes(`report.${flag}`),
			`通報頁沒有依 ${flag} 顯示按鈕`,
		);
	}
	for (const [name, source] of [
		["案件頁", caseSource],
		["通報頁", reportSource],
	]) {
		assert.doesNotMatch(
			source,
			/\.state\s*(===|!==)\s*(Case|Report)State\./u,
			`${name}不得用狀態推導按鈕`,
		);
	}
});

test("內部註記：措施與案件共用 governance/NotesSection，各自只給自己的讀取與新增", async () => {
	const measure = await readFile(
		"src/app/admin/measures/[id]/MeasureNotesSection.tsx",
		"utf8",
	);
	const sections = await readFile(
		`${ROOT}/cases/[id]/CaseSections.tsx`,
		"utf8",
	);
	assert.match(measure, /from "\.\.\/\.\.\/governance\/NotesSection"/u);
	assert.match(sections, /from "@\/app\/admin\/governance\/NotesSection"/u);
	const shared = await readFile(
		"src/app/admin/governance/NotesSection.tsx",
		"utf8",
	);
	assert.match(
		shared,
		/severityChangeText\(note\.severityChange\)/u,
		"改嚴重度的註記標頭",
	);
	assert.doesNotMatch(
		shared,
		/listMeasureNotes|listCaseNotes|createMeasureNote|createCaseNote/u,
	);
});

test("頁面不走舊的 gateway REST，也不用原生 alert／confirm", async () => {
	for (const file of await sourceFiles(ROOT)) {
		const source = await readFile(file, "utf8");
		assert.doesNotMatch(
			source,
			/services\/api"|\/api\/v1\//u,
			`${file} 還在用舊的 REST`,
		);
		assert.doesNotMatch(
			source,
			/\balert\(|\bconfirm\(/u,
			`${file} 有原生對話框`,
		);
	}
});

test("刪掉的舊頁約 60 處字串沒有留在新頁面：沒有「危急（食安）」「調查中」「已成立」「確認 ack」", async () => {
	for (const file of await sourceFiles(ROOT)) {
		const source = await readFile(file, "utf8");
		assert.doesNotMatch(
			source,
			/危急（食安）|調查中|已成立|已駁回|確認 ack|unacked/u,
			file,
		);
	}
});

test("主管機關來函：側欄「主管機關來函」在「賣家 / 商品」分組、「商家通報」之後；三個頁面與四個對話框都在，對話框都用共用的 Dialog", async () => {
	const sidebar = await readFile("src/components/Layout/Sidebar.tsx", "utf8");
	const reports = sidebar.indexOf('href: "/admin/violation-reports"');
	const orders = sidebar.indexOf('href: "/admin/authority-orders"');
	assert.ok(orders > reports, "在商家通報之後");
	assert.ok(
		orders < sidebar.indexOf('href: "/admin/measures"'),
		"商家通報與措施與申覆之間",
	);
	assert.match(sidebar, /label: "主管機關來函"/u);
	const root = "src/app/admin/authority-orders";
	const files = await sourceFiles(root);
	for (const page of ["page.tsx", "new/page.tsx", "[id]/page.tsx"]) {
		assert.ok(files.includes(`${root}/${page}`), page);
	}
	const dialogs = files.filter((f) => f.endsWith("Dialog.tsx"));
	assert.equal(dialogs.length, 4, "完成來函、延長保存、記下告知、撤銷登記");
	for (const dialog of dialogs) {
		assert.match(await readFile(dialog, "utf8"), /<Dialog\b/u, dialog);
	}
});

test("從來函開的三個建立措施入口都帶來源來函（違規依據另帶新認定的來源來函）", async () => {
	const page = await readFile(
		"src/app/admin/authority-orders/[id]/page.tsx",
		"utf8",
	);
	for (const dialog of [
		"CreateMeasureDialog",
		"CreateRiskMeasureDialog",
		"CreateTerminationDialog",
	]) {
		const element = page.match(
			new RegExp(`<${dialog}\\b[\\s\\S]*?/>`, "u"),
		)?.[0];
		assert.ok(element, `找不到 <${dialog}`);
		assert.match(element, /sourceAuthorityOrder=\{sourceOrder\}/u, dialog);
	}
	assert.match(
		page,
		/initial=\{\{ sourceAuthorityOrderIds: \[order\.id\] \}\}/u,
		"違規依據的新認定帶來源來函",
	);
});
