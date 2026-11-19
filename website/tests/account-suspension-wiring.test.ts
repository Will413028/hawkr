import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

/** 去掉所有空白：JSX 會把長句斷在任意位置。 */
function compact(relativePath: string) {
	return readFileSync(
		new URL(`../${relativePath}`, import.meta.url),
		"utf8",
	).replace(/\s+/g, "");
}

test("停權橫幅掛在根文件的導覽列之後、main 之前（所有頁面都看得到）", () => {
	assert.match(
		compact("components/shared/RootDocument.tsx"),
		/<RoleAwareNavbar\/><\/Suspense><AccountSuspensionBanner\/><mainclassName="page-content">/,
	);
});

test("帳號設定頁頂端有受限區塊，刪除帳號沿用既有區塊", () => {
	const page = compact("features/account/screens/settings/SettingsPage.tsx");
	assert.match(
		page,
		/<divclassName="flexflex-colgap-4"><AccountRestrictedSection\/>/,
	);
	assert.equal(page.split("<DeleteAccountSection/>").length - 1, 1);
});

test("預告橫幅與停權橫幅共用 WarningNotice", () => {
	for (const file of [
		"features/seller/measures/TerminationNoticeBanner.tsx",
		"components/shared/AccountSuspensionBanner.tsx",
	]) {
		assert.match(compact(file), /<WarningNotice/, file);
	}
});

test("兩個 transport 都掛 accountSuspendedInterceptor", () => {
	const src = readFileSync(
		new URL("../lib/connect/transport.ts", import.meta.url),
		"utf8",
	);
	assert.equal(src.split("accountSuspendedInterceptor(),").length - 1, 2);
});
