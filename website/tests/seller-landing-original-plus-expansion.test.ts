import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";
import { SELLER_FEATURE_OVERVIEW } from "../features/seller/landing/sellerFeatureOverviewData";

function source(relativePath: string) {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

test("seller landing adds Scroll World before the untouched original story", () => {
	const landing = source("features/seller/landing/SellerLandingPage.tsx");
	const openingIndex = landing.indexOf("<SellerRecruitmentScrollWorld");
	const mainIndex = landing.indexOf("<main");
	const originalHeroIndex = landing.indexOf(
		'className="seller-hero seller-snap-section"',
	);

	assert.ok(openingIndex > -1, "Scroll World should be rendered");
	assert.ok(
		openingIndex < mainIndex,
		"Scroll World should sit before the old main",
	);
	assert.ok(
		mainIndex < originalHeroIndex,
		"the original seller hero should remain inside its existing main",
	);
	assert.match(
		landing,
		/import SellerRecruitmentScrollWorld from "\.\/components\/SellerRecruitmentScrollWorld"/,
	);

	const orderedMarkers = [
		'className="seller-hero seller-snap-section"',
		"<SellerFeatureOverview />",
		"<SellerPartnerExpansion />",
		'className="seller-process seller-snap-section"',
		"<SellerShowcaseCarousel />",
		'className="story-trail seller-story-trail"',
		'className="seller-faq seller-snap-section"',
		'className="seller-cta seller-snap-section"',
	];

	let previousIndex = -1;
	for (const marker of orderedMarkers) {
		const markerIndex = landing.indexOf(marker);
		assert.ok(
			markerIndex > previousIndex,
			`${marker} should preserve page order`,
		);
		previousIndex = markerIndex;
	}

	assert.match(landing, /生鮮商品/);
	assert.match(landing, /小農、鮮食與食品品牌的自助開店市集/);
	assert.match(landing, /開始建立賣場/);
	assert.doesNotMatch(landing, /申請成為商家|立即申請/);
	assert.match(landing, /<SellerFeatureOverview \/>/);
	assert.match(landing, /自主營運路徑/);
	assert.doesNotMatch(
		landing,
		/訂單穩定進來|累積回購粉絲|第一批商品由我們陪你/,
	);
	assert.match(landing, /href="\/seller\/onboarding"/);
	assert.match(
		landing,
		/href="\/seller\/pricing"/,
		"seller recruitment should link to the exact pricing route",
	);
});

test("public seller acquisition copy keeps commercial fee terms private", () => {
	const landing = source("features/seller/landing/SellerLandingPage.tsx");
	const faqData = source("lib/content/data.ts");
	const guideData = source("lib/content/seller-guide-data.ts");
	const terms = source("features/legal/pages/TermsPage.tsx");
	const returnPolicy = source("features/legal/pages/ReturnPolicyPage.tsx");
	const userPolicy = source("features/legal/pages/UserPolicyPage.tsx");
	const privacy = source("features/legal/pages/PrivacyPage.tsx");

	for (const publicSource of [
		landing,
		faqData,
		guideData,
		terms,
		returnPolicy,
		userPolicy,
		privacy,
	]) {
		assert.doesNotMatch(
			publicSource,
			/(?<![\d.])(?:94|6)\s*[%％]|百分之(?:九十四|六)/,
		);
	}
	assert.doesNotMatch(landing, /drive\.google\.com\/file/);
	assert.doesNotMatch(landing, /下載合作說明/);
	assert.match(landing, /href="\/contact"/);
	assert.match(landing, /聯繫合作顧問/);
});

test("seller expansion keeps the three Hawkr partner stories without the removed preparation block", () => {
	const expansion = source(
		"features/seller/landing/components/SellerPartnerExpansion.tsx",
	);

	for (const phrase of ["產地小農", "市場與生鮮門市", "食品品牌與加工職人"]) {
		assert.match(expansion, new RegExp(phrase));
	}

	assert.doesNotMatch(expansion, /seller-prep/);
	assert.doesNotMatch(expansion, /申請前，先準備這些/);
	assert.doesNotMatch(expansion, /PREPARATION_ITEMS/);
	assert.doesNotMatch(expansion, /dashboard|mockup|手機畫面|電腦畫面/i);
	assert.match(expansion, /scene-irrigation\.svg/);
	assert.match(expansion, /scene-market\.svg/);
	assert.match(expansion, /scene-greenhouse\.svg/);
	const styles = source("styles/features/seller-onboarding.css");
	assert.match(
		styles,
		/\.seller-fit\s*\{[\s\S]*?border-radius:\s*36px;[\s\S]*?backdrop-filter:\s*blur\(20px\)/,
	);
	assert.match(
		styles,
		/\[data-theme="dark"\] \.seller-onboarding \.seller-fit\s*\{/,
	);
});

test("seller recruitment scroll world ends at the self-service storefront entry", () => {
	const scrollWorld = source(
		"features/seller/landing/components/SellerRecruitmentScrollWorld.tsx",
	);

	assert.match(scrollWorld, /食品品牌自助開店/);
	assert.match(scrollWorld, /href="\/seller\/onboarding"/);
	assert.match(scrollWorld, /開始建立賣場/);
});

test("seller showcase keeps autoplay optional and exposes equivalent manual controls", () => {
	const carousel = source(
		"features/seller/landing/components/SellerShowcaseCarousel.tsx",
	);
	const styles = source("styles/features/seller-showcase.css");

	assert.match(carousel, /prefers-reduced-motion: reduce/);
	assert.match(carousel, /停止自動輪播/);
	assert.match(carousel, /開始自動輪播/);
	assert.match(carousel, /aria-live="polite"/);
	assert.match(carousel, /inert=\{index !== active\}/);
	assert.match(carousel, /onPointerDown=\{handlePointerDown\}/);
	assert.match(carousel, /aria-controls="showcaseTrack"/);
	assert.match(carousel, /IntersectionObserver/);
	for (const tag of ["#經營儀表板", "#訂單管理", "#商品管理", "#通知中心"]) {
		assert.match(carousel, new RegExp(tag));
	}
	assert.doesNotMatch(
		carousel,
		/\d{2} \/ (?:DASHBOARD|ORDERS|PRODUCTS|NOTIFY)/,
	);
	assert.match(
		styles,
		/\.cp-seg::after\s*\{[\s\S]*?background:\s*var\(--showcase-primary\);/,
	);
	assert.doesNotMatch(
		styles,
		/\.cp-seg::after\s*\{[\s\S]*?background:\s*linear-gradient\(/,
	);
});

test("seller process shows web screen placeholders in a mobile snap row", () => {
	const landing = source("features/seller/landing/SellerLandingPage.tsx");
	const styles = source("styles/features/seller-onboarding.css");

	assert.match(landing, /\/placeholders\/web-screen\.svg/);
	assert.ok(
		existsSync(
			new URL("../public/placeholders/web-screen.svg", import.meta.url),
		),
		"web-screen.svg should exist in public/placeholders",
	);

	assert.match(landing, /<ol className="process-steps"/);
	assert.match(landing, /<figure className="process-step__visual">/);
	assert.match(landing, /<AppImage/);
	assert.match(landing, /width=\{1040\}/);
	assert.match(landing, /height=\{650\}/);
	// Eager, not lazy: the long-page stability E2E gate requires every image on
	// this landing page to materialise before scroll (see SellerShowcaseImageWarmer
	// + eager carousel slides). A lazy process image fetched only on scroll, which
	// the gate rejects as an unscheduled request.
	assert.match(landing, /loading="eager"/);
	assert.match(landing, /商家名稱、所在地、寄件地址與商店介紹/);
	assert.doesNotMatch(landing, /基本資料、商品類型與產地/);
	assert.match(styles, /\.process-step__visual/);
	assert.match(styles, /scroll-snap-type: x mandatory/);
	assert.match(styles, /scroll-snap-align: start/);
	assert.match(styles, /\.process-steps::-webkit-scrollbar/);
});

test("seller feature overview is a transparent nine-category icon directory", () => {
	const landing = source("features/seller/landing/SellerLandingPage.tsx");
	const overview = source(
		"features/seller/landing/components/SellerFeatureOverview.tsx",
	);
	const animations = source(
		"features/seller/landing/components/SellerAnimations.tsx",
	);
	const overviewStyles = source("styles/features/seller-feature-overview.css");
	const overviewIcon = source(
		"features/seller/landing/components/SellerFeatureOverviewIcon.tsx",
	);

	assert.deepEqual(
		SELLER_FEATURE_OVERVIEW.map((group) => group.title),
		[
			"品牌店面",
			"商品管理",
			"庫存管理",
			"訂單管理",
			"出貨與售後",
			"顧客經營",
			"內容與優惠",
			"訊息與通知",
			"營運管理",
		],
	);
	assert.equal(SELLER_FEATURE_OVERVIEW.length, 9);
	assert.deepEqual(
		SELLER_FEATURE_OVERVIEW.map((group) => group.icon),
		[
			"storefront",
			"products",
			"inventory",
			"orders",
			"fulfillment",
			"customers",
			"content",
			"messages",
			"operations",
		],
	);
	assert.equal(
		new Set(SELLER_FEATURE_OVERVIEW.map((group) => group.id)).size,
		9,
		"every chapter target should have a unique id",
	);

	const allFeatures = SELLER_FEATURE_OVERVIEW.flatMap(
		(group) => group.features,
	);
	const normalizedFeatures = allFeatures.map((feature) =>
		feature.replaceAll(/[\s，、；。]/g, ""),
	);
	assert.equal(
		new Set(normalizedFeatures).size,
		normalizedFeatures.length,
		"specific feature statements should not repeat across categories",
	);

	const publicCopy = SELLER_FEATURE_OVERVIEW.map(
		(group) => `${group.summary}\n${group.features.join("\n")}`,
	).join("\n");
	for (const transparentLimit of [
		"市集內店面，不是獨立網站",
		"服務商開通",
		"只供補貨判斷參考",
		"已結束",
		"站內訊息",
		"PayUni",
		"不提供提款操作",
	]) {
		assert.match(publicCopy, new RegExp(transparentLimit));
	}
	assert.doesNotMatch(
		publicCopy,
		/POS|社群收單|直播購物|AI 助手|保證(?:流量|訂單|營收|回購)/,
	);

	assert.equal((landing.match(/<SellerFeatureOverview \/>/g) ?? []).length, 1);
	assert.doesNotMatch(landing, /SellerBenefitsInteractive|BENEFITS/);
	assert.equal(
		existsSync(
			new URL(
				"../features/seller/landing/components/SellerBenefitsInteractive.tsx",
				import.meta.url,
			),
		),
		false,
		"the unused benefits component stays deleted",
	);
	assert.match(overview, /<nav[\s\S]*aria-label="賣家中心功能分類"/);
	assert.match(overview, /href=\{`#\$\{group\.id\}`\}/);
	assert.match(overview, /aria-current=/);
	assert.match(overview, /<SellerFeatureOverviewIcon/);
	assert.match(overview, /icon=\{group\.icon\}/);
	assert.equal(
		(overview.match(/SELLER_FEATURE_OVERVIEW\.map/g) ?? []).length,
		4,
		"one data source should drive ids, observation, navigation and chapters",
	);
	assert.doesNotMatch(
		overview,
		/role="(?:tablist|tab|toolbar)"|\bhidden\b|\binert\b|data-seller-snap|<AppImage|<svg/,
	);
	assert.doesNotMatch(overview, /className="[^"]*(?:badge|chip|pill|tag)/i);
	assert.doesNotMatch(animations, /seller-benefits|benefit-card/);
	assert.match(overviewStyles, /position:\s*sticky/);
	assert.match(overviewStyles, /border-top:\s*1px solid/);
	assert.match(
		overviewStyles,
		/\.seller-feature-overview__icon\s*\{[\s\S]*?width:\s*40px;[\s\S]*?height:\s*40px;[\s\S]*?flex:\s*0 0 40px;/,
	);
	assert.match(overviewIcon, /aria-hidden="true"/);
	assert.match(overviewIcon, /<AppIcon name=\{SELLER_FEATURE_ICONS\[icon\]\}/);
	assert.doesNotMatch(
		overviewIcon,
		/24dayswarranty|Bookmark|Telephone|helicopter|dollar-box/i,
	);
	assert.match(
		overviewStyles,
		/background:\s*rgba\(255, 253, 247, 0\.62\);[\s\S]*?backdrop-filter:\s*blur\(20px\) saturate\(140%\);[\s\S]*?0 24px 64px -32px rgb\(var\(--color-ink-700-rgb\) \/ 0\.18\)/,
	);
	assert.match(overviewStyles, /prefers-reduced-transparency: reduce/);
});

test("seller section labels use copyable social hashtags without white pill chrome", () => {
	const landing = source("features/seller/landing/SellerLandingPage.tsx");
	const expansion = source(
		"features/seller/landing/components/SellerPartnerExpansion.tsx",
	);
	const styles = source("styles/features/seller-onboarding.css");
	const showcaseStyles = source("styles/features/seller-showcase.css");

	assert.equal((landing.match(/eyebrow--hashtags/g) ?? []).length, 5);
	assert.equal((expansion.match(/eyebrow--hashtags/g) ?? []).length, 1);
	assert.doesNotMatch(landing, /eb-dot/);
	assert.doesNotMatch(expansion, /eb-dot/);
	for (const tag of [
		"#商家招募",
		"#開店三步驟",
		"#自主營運路徑",
		"#開店FAQ",
		"#加入 Hawkr",
	]) {
		assert.match(landing, new RegExp(tag));
	}
	assert.match(expansion, /#適合誰加入/);
	assert.match(
		styles,
		/\.eyebrow\.eyebrow--hashtags\s*\{[\s\S]*?padding:\s*0;[\s\S]*?border:\s*0;[\s\S]*?background:\s*none;[\s\S]*?box-shadow:\s*none;/,
	);
	assert.match(
		showcaseStyles,
		/\.showcase-eyebrow\s*\{[\s\S]*?padding:\s*0;[\s\S]*?border:\s*0;[\s\S]*?background:\s*none;/,
	);
});

test("seller sections share a safe viewport stage and use proximity snapping", () => {
	const landing = source("features/seller/landing/SellerLandingPage.tsx");
	const animations = source(
		"features/seller/landing/components/SellerAnimations.tsx",
	);
	const styles = source("styles/features/seller-onboarding.css");

	assert.match(landing, /data-seller-snap/);
	assert.doesNotMatch(
		landing,
		/story-trail seller-story-trail seller-snap-section/,
	);
	assert.doesNotMatch(landing, /trail-stop seller-snap-beat/);
	assert.match(animations, /import\("lenis\/snap"\)/);
	assert.match(animations, /type: "proximity"/);
	assert.match(animations, /distanceThreshold: "20%"/);
	assert.match(animations, /align: "end"/);
	assert.match(styles, /--seller-section-height/);
	assert.match(styles, /--seller-section-height:\s*clamp\(/);
	assert.match(styles, /min-block-size: var\(--seller-section-height\)/);
	assert.match(styles, /scroll-snap-type: y proximity/);
	assert.match(styles, /prefers-reduced-motion: reduce/);
});

test("seller motion remains page scoped and preserves reduced-motion fallbacks", () => {
	const animations = source(
		"features/seller/landing/components/SellerAnimations.tsx",
	);
	const styles = source("styles/features/seller-onboarding.css");

	assert.match(animations, /gsap\.context/);
	assert.match(animations, /new Lenis/);
	assert.doesNotMatch(animations, /story-trail-path__progress/);
	assert.doesNotMatch(animations, /querySelector\(".seller-story-trail"\)/);
	assert.match(styles, /seller-onboarding \.eyebrow/);
	assert.match(styles, /prefers-reduced-motion: reduce/);
	assert.match(styles, /seller-vanta-atmosphere/);
});
