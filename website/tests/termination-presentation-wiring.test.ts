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

test("隱私政策的生效日不早於它引用的生成文件中最新的一次修訂", () => {
	const privacy = compact("features/legal/pages/PrivacyPage.tsx");
	const latest = [SELLER_TERMS.version, KYC_PRIVACY_NOTICE.version]
		.sort()
		.at(-1) as string;
	const match = privacy.match(/lastUpdated="(\d{4}-\d{2}-\d{2})"/);
	assert.ok(match, "PrivacyPage 要有 lastUpdated");
	assert.ok(match[1] >= latest, `${match[1]} 早於 ${latest}`);
});

test("隱私政策有貼文瀏覽統計一節，內文是核可句", () => {
	const privacy = readFileSync(
		new URL("../features/legal/pages/PrivacyPage.tsx", import.meta.url),
		"utf8",
	);
	assert.ok(privacy.includes('title: "三、貼文瀏覽統計"'));
	assert.ok(
		privacy.includes(
			"我們會統計貼文的瀏覽次數；未登入的瀏覽只以每日更換的雜湊值去重，不儲存您的 IP 位址或瀏覽器資訊。",
		),
	);
});

test("預告橫幅掛在賣家中心外框、措施與申覆入口掛在帳號總覽", () => {
	assert.match(
		compact("features/seller-center/components/SellerCenterShell.tsx"),
		/<divclassName="seller-main"><TerminationNoticeBanner\/>\{children\}<\/div>/,
	);
	assert.match(
		compact("features/account/screens/overview/AccountOverviewClient.tsx"),
		/<MeasuresAccountEntry\/>/,
	);
});
