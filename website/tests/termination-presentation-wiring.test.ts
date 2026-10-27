import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { SELLER_TERMS } from "../features/legal/documents/sellerTerms.gen";

/** 去掉所有空白：JSX 會把長句斷在任意位置。 */
function compact(relativePath: string) {
	return readFileSync(
		new URL(`../${relativePath}`, import.meta.url),
		"utf8",
	).replace(/\s+/g, "");
}

test("隱私政策的生效日與生成的賣家契約同一次修訂", () => {
	const privacy = compact("features/legal/pages/PrivacyPage.tsx");
	// 版本日期取自生成的條款（docs/legal 的唯一來源），隱私政策與它同一次修訂。
	assert.ok(
		privacy.includes(`lastUpdated="${SELLER_TERMS.version}"`),
		SELLER_TERMS.version,
	);
});

test("預告橫幅掛在賣家中心外框、措施與申覆入口掛在帳號總覽", () => {
	assert.match(
		compact("features/seller-center/components/SellerCenterShell.tsx"),
		/<mainclassName="seller-main"><TerminationNoticeBanner\/>\{children\}<\/main>/,
	);
	assert.match(
		compact("features/account/screens/overview/AccountOverviewClient.tsx"),
		/<MeasuresAccountEntry\/>/,
	);
});
