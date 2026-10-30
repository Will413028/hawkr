import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { KYC_PRIVACY_NOTICE } from "../features/legal/documents/sellerKycNotice.gen";
import { SELLER_TERMS } from "../features/legal/documents/sellerTerms.gen";

/** 去掉所有空白：JSX 會把長句斷在任意位置。 */
function compact(relativePath: string) {
	return readFileSync(
		new URL(`../${relativePath}`, import.meta.url),
		"utf8",
	).replace(/\s+/g, "");
}

test("隱私政策的生效日是它引用的生成文件中最新的一次修訂", () => {
	const privacy = compact("features/legal/pages/PrivacyPage.tsx");
	// 版本日期取自生成的條款與 KYC 告知（docs/legal 的唯一來源），隱私頁內嵌 KYC 告知，取兩者較新者。
	const latest = [SELLER_TERMS.version, KYC_PRIVACY_NOTICE.version].sort().at(-1);
	assert.ok(privacy.includes(`lastUpdated="${latest}"`), latest);
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
