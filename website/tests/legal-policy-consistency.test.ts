import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

function source(relativePath: string) {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

const publicPolicyPaths = [
	"features/legal/pages/TermsPage.tsx",
	"features/legal/pages/ReturnPolicyPage.tsx",
	"features/legal/pages/UserPolicyPage.tsx",
	"features/legal/pages/PrivacyPage.tsx",
];

const sellerLegalPaths = [
	"features/legal/pages/SellerTermsPage.tsx",
	"docs/legal/seller-terms.md",
	"docs/legal/seller-kyc-notice.md",
];

test("public legal policies keep private fee percentages out of public copy", () => {
	for (const path of [...publicPolicyPaths, ...sellerLegalPaths]) {
		assert.doesNotMatch(
			source(path),
			/(?<![\d.])(?:94|6)\s*[%％]|百分之(?:九十四|六)/,
			`${path} must not publish the negotiated 6% / 94% terms`,
		);
	}
});

test("public legal pages declare that this is a demo project", () => {
	const notice = source("features/legal/DemoNotice.tsx");
	assert.match(notice, /Hawkr 是作品集的示範專案，不是實際營運的服務/);
	assert.match(notice, /不收集、不處理真實的個人資料，請勿輸入真實資料/);
	assert.match(notice, /不構成任何契約或法律義務/);
	for (const path of publicPolicyPaths) {
		assert.match(source(path), /<DemoNotice topic=/, `${path} must render it`);
	}
});

test("historical backend notes are clearly marked as implementation snapshots", () => {
	const backendSnapshot = source("docs/transaction-policy.md");
	const sellerFlow = source("docs/seller-payment-logistics-flow.md");

	assert.match(backendSnapshot, /非現行商務契約/);
	assert.match(backendSnapshot, /禁止誤用/);
	assert.match(sellerFlow, /現行版本：2026-08-13/);
	assert.match(sellerFlow, /費率基準已改為三方案版本化/);
	assert.match(
		sellerFlow,
		/成交服務費是 Hawkr 完整的平台交易服務對價，費率依該筆訂單鎖定的方案版本/,
	);
	assert.match(sellerFlow, /金流手續費由賣家另行負擔/);
	assert.doesNotMatch(sellerFlow, /6% 完整平台抽成|6% 平台服務費/);
});

interface Manifest {
	agreements: { kind: string; url: string }[];
}

function manifestUrl(kind: string): string {
	const manifest = JSON.parse(
		source("docs/legal/seller-agreements.json"),
	) as Manifest;
	const found = manifest.agreements.find((a) => a.kind === kind);
	assert.ok(found, `manifest 找不到 ${kind}`);
	return found.url;
}

test("seller legal pages render the generated documents", () => {
	assert.match(
		source("features/legal/pages/SellerTermsPage.tsx"),
		/lastUpdated=\{SELLER_TERMS\.version\}/,
	);
	assert.match(
		source("features/legal/pages/PrivacyPage.tsx"),
		/本節版本：\$\{KYC_PRIVACY_NOTICE\.version\}/,
	);
});

test("seller agreement URLs resolve to real routes and anchors", () => {
	const terms = manifestUrl("SELLER_TERMS");
	assert.equal(terms, "/seller-terms");
	assert.ok(
		existsSync(
			new URL(
				"../app/[locale]/(content)/seller-terms/page.tsx",
				import.meta.url,
			),
		),
		"/seller-terms 沒有對應的 app route",
	);

	const notice = manifestUrl("KYC_PRIVACY_NOTICE");
	assert.equal(notice, "/privacy#seller-kyc-notice");
	const [noticePath, noticeAnchor] = notice.split("#");
	assert.ok(
		existsSync(
			new URL(
				`../app/[locale]/(content)${noticePath}/page.tsx`,
				import.meta.url,
			),
		),
		`${noticePath} 沒有對應的 app route`,
	);
	assert.match(
		source("features/legal/pages/PrivacyPage.tsx"),
		new RegExp(`id: "${noticeAnchor}"`),
	);
});
