import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
	COLLABORATION_ROWS,
	CROSSOVER_GROWTH_PRO,
	CROSSOVER_STARTER_GROWTH,
	calculatePlanFees,
	lowestCostPlans,
	SELLER_PLANS,
	SELLER_PRICING_FAQS,
	SERVICE_BUNDLES,
} from "@/features/seller/pricing/data";
import {
	PUBLIC_MANAGED_SERVICE_ADD_ON,
	PUBLIC_SELLER_PLANS,
	PUBLIC_SELLER_PRICING_FAQ_CATEGORIES,
	PUBLIC_SELLER_PRICING_FAQS,
	PUBLIC_TAX_NOTE,
} from "@/features/seller/pricing/public-data";

function source(relativePath: string) {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

function cssRule(css: string, selector: string) {
	const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	const match = css.match(
		new RegExp(`(?:^|\\})\\s*${escaped}\\s*\\{([^{}]*)\\}`, "m"),
	);
	assert.ok(match, `missing CSS rule for ${selector}`);
	return match[1];
}

function cssDeclaration(rule: string, property: string) {
	const escaped = property.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	const match = rule.match(
		new RegExp(`(?:^|;)\\s*${escaped}\\s*:\\s*([^;]+);`),
	);
	assert.ok(match, `missing ${property} declaration`);
	return match[1].trim();
}

test("legacy billing data retains all three agreed plans", () => {
	assert.deepEqual(
		SELLER_PLANS.map(({ id, code, name, monthlyFee, transactionFeeRate }) => ({
			id,
			code,
			name,
			monthlyFee,
			transactionFeeRate,
		})),
		[
			{
				id: "starter",
				code: "STARTER",
				name: "Starter 起步",
				monthlyFee: 0,
				transactionFeeRate: 0.06,
			},
			{
				id: "growth",
				code: "GROWTH",
				name: "Growth 成長",
				monthlyFee: 990,
				transactionFeeRate: 0.03,
			},
			{
				id: "pro",
				code: "PRO",
				name: "Pro 專業",
				monthlyFee: 2_990,
				transactionFeeRate: 0,
			},
		],
	);

	for (const plan of SELLER_PLANS) {
		assert.ok(plan.zeroSalesNote.length > 0);
		assert.ok(plan.excluded.length > 0);
		assert.ok(plan.stage.length > 0);
	}
});

test("legacy fee calculations and crossover points remain compatible", () => {
	const totals = (amount: number) =>
		calculatePlanFees(amount).map((entry) => entry.total);
	const transactionFees = (amount: number) =>
		calculatePlanFees(amount).map((entry) => entry.transactionFee);

	assert.deepEqual(transactionFees(10_000), [600, 300, 0]);
	assert.deepEqual(totals(0), [0, 990, 2_990]);
	assert.deepEqual(totals(50_000), [3_000, 2_490, 2_990]);
	assert.equal(CROSSOVER_STARTER_GROWTH, 33_000);
	assert.equal(CROSSOVER_GROWTH_PRO, 66_667);

	const at33k = calculatePlanFees(CROSSOVER_STARTER_GROWTH);
	assert.equal(at33k[0]?.total, at33k[1]?.total);
	assert.deepEqual(
		lowestCostPlans(at33k).map((entry) => entry.plan.id),
		["starter", "growth"],
	);
	assert.deepEqual(
		lowestCostPlans(calculatePlanFees(80_000)).map((entry) => entry.plan.id),
		["pro"],
	);
});

test("legacy responsibility, service, and FAQ records stay available", () => {
	assert.deepEqual(
		COLLABORATION_ROWS.map((row) => row.name),
		[
			"商品資料與食品事實",
			"商品上架與頁面內容",
			"價格、庫存與促銷",
			"訂單、客服、售後與出貨",
			"平台交易、結算與對帳",
			"行銷與廣告",
		],
	);
	for (const row of COLLABORATION_ROWS) {
		assert.deepEqual(Object.keys(row.plans), ["starter", "growth", "pro"]);
	}

	assert.deepEqual(
		SERVICE_BUNDLES.map(({ id, name, price }) => ({ id, name, price })),
		[
			{ id: "launch", name: "開店上線包", price: 2_990 },
			{ id: "content", name: "成交內容包", price: 5_990 },
			{ id: "full", name: "完整行銷包", price: 7_990 },
		],
	);
	assert.equal(SELLER_PRICING_FAQS.length, 13);
});

test("the public pricing contract exports only Free and Pro with monthly and annual fees", () => {
	assert.deepEqual(
		PUBLIC_SELLER_PLANS.map(
			({
				id,
				code,
				name,
				contractName,
				monthlyFee,
				annualFee,
				annualSavings,
				transactionFeeRate,
			}) => ({
				id,
				code,
				name,
				contractName,
				monthlyFee,
				annualFee,
				annualSavings,
				transactionFeeRate,
			}),
		),
		[
			{
				id: "starter",
				code: "STARTER",
				name: "Free 免費",
				contractName: "正式合約與帳務沿用 Starter 名稱",
				monthlyFee: 0,
				annualFee: 0,
				annualSavings: 0,
				transactionFeeRate: 0.06,
			},
			{
				id: "pro",
				code: "PRO",
				name: "Pro 專業",
				contractName: "正式合約與帳務名稱同為 Pro",
				monthlyFee: 2_990,
				annualFee: 29_900,
				annualSavings: 5_980,
				transactionFeeRate: 0,
			},
		],
	);
	const pro = PUBLIC_SELLER_PLANS.find((plan) => plan.id === "pro");
	assert.ok(pro);
	assert.equal(pro.monthlyFee * 12 - pro.annualFee, pro.annualSavings);
	assert.match(PUBLIC_TAX_NOTE, /含 5% 營業稅/);

	const publicCopy = JSON.stringify({
		plans: PUBLIC_SELLER_PLANS,
		managedService: PUBLIC_MANAGED_SERVICE_ADD_ON,
		faqs: PUBLIC_SELLER_PRICING_FAQS,
		taxNote: PUBLIC_TAX_NOTE,
	});
	assert.doesNotMatch(publicCopy, /Growth|GROWTH/i);
	assert.doesNotMatch(publicCopy, /(?<![\d,])990(?!\d)/);
	assert.doesNotMatch(publicCopy, /3%/);
});

test("the managed add-on publishes its fee, work scope, and seller boundary", () => {
	assert.deepEqual(
		{
			id: PUBLIC_MANAGED_SERVICE_ADD_ON.id,
			name: PUBLIC_MANAGED_SERVICE_ADD_ON.name,
			feeRate: PUBLIC_MANAGED_SERVICE_ADD_ON.feeRate,
			feeLabel: PUBLIC_MANAGED_SERVICE_ADD_ON.feeLabel,
			includes: PUBLIC_MANAGED_SERVICE_ADD_ON.includes,
		},
		{
			id: "managed",
			name: "加值代行包",
			feeRate: 0.06,
			feeLabel: "每筆有效交易額外 6%",
			includes: [
				"上架支援團隊依賣家確認的商品資料協助上架",
				"精修賣家提供且有權使用的電商圖片",
			],
		},
	);
	assert.match(
		PUBLIC_MANAGED_SERVICE_ADD_ON.sellerKeeps,
		/接單確認、備貨、出貨/,
	);
	assert.match(PUBLIC_MANAGED_SERVICE_ADD_ON.boundary, /Free 加購後合計 12%/);
	assert.match(PUBLIC_MANAGED_SERVICE_ADD_ON.boundary, /Pro 加購後合計 6%/);
});

test("the public FAQ covers the eight Free, Pro, annual, and add-on decisions", () => {
	assert.deepEqual(
		PUBLIC_SELLER_PRICING_FAQ_CATEGORIES.map(({ id, label, description }) => ({
			id,
			label,
			description,
		})),
		[
			{
				id: "plans",
				label: "方案與費用",
				description: "Free、Pro 與年繳計算",
			},
			{
				id: "service",
				label: "代行與責任",
				description: "加值服務與合作分工",
			},
			{
				id: "changes",
				label: "退款與變更",
				description: "退款計費與方案調整",
			},
		],
	);
	assert.deepEqual(
		Object.fromEntries(
			PUBLIC_SELLER_PRICING_FAQ_CATEGORIES.map((category) => [
				category.id,
				PUBLIC_SELLER_PRICING_FAQS.filter((faq) => faq.category === category.id)
					.length,
			]),
		),
		{ plans: 4, service: 2, changes: 2 },
	);
	assert.equal(PUBLIC_SELLER_PRICING_FAQS.length, 8);
	assert.deepEqual(
		PUBLIC_SELLER_PRICING_FAQS.map((faq) => faq.question),
		[
			"Free 包含哪些費用？",
			"Pro 的 0% 是不是所有費用都免了？",
			"Pro 年繳怎麼計算？",
			"加值代行包包含什麼？",
			"當月沒有成交還要付費嗎？",
			"退款後成交服務費怎麼處理？",
			"可以變更方案嗎？",
			"平台和賣家各自負責什麼？",
		],
	);
	assert.match(PUBLIC_SELLER_PRICING_FAQS[0]?.answer ?? "", /Starter 名稱/);
	assert.match(PUBLIC_SELLER_PRICING_FAQS[1]?.answer ?? "", /^不是。/);
	assert.match(PUBLIC_SELLER_PRICING_FAQS[2]?.answer ?? "", /NT\$29,900/);
	assert.match(PUBLIC_SELLER_PRICING_FAQS[2]?.answer ?? "", /NT\$5,980/);
	assert.match(PUBLIC_SELLER_PRICING_FAQS[3]?.answer ?? "", /額外收取 6%/);
	assert.match(PUBLIC_SELLER_PRICING_FAQS[6]?.answer ?? "", /不回溯/);
});

test("the categorized FAQ keeps anchor navigation and native disclosure semantics", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const css = source("styles/features/seller-pricing.css");
	const faqSource = page.slice(
		page.indexOf("function PricingFaqSection"),
		page.indexOf("function SellerLeadForm"),
	);

	assert.match(faqSource, /PUBLIC_SELLER_PRICING_FAQ_CATEGORIES\.map/);
	assert.match(
		faqSource,
		/PUBLIC_SELLER_PRICING_FAQS\.filter\([\s\S]*?faq\.category === category\.id/,
	);
	assert.match(faqSource, /<nav[\s\S]*?aria-label="常見問題分類"/);
	assert.match(faqSource, /className="sp-faq__nav-meta"/);
	assert.match(faqSource, /共 \{PUBLIC_SELLER_PRICING_FAQS\.length\} 題/);
	assert.match(faqSource, /href=\{`#sp-faq-category-\$\{category\.id\}`\}/);
	assert.match(
		faqSource,
		/aria-current=\{isActive \? "location" : undefined\}/,
	);
	assert.match(faqSource, /id=\{`sp-faq-category-\$\{category\.id\}`\}/);
	assert.match(faqSource, /data-faq-category=\{category\.id\}/);
	assert.match(faqSource, /new IntersectionObserver/);
	assert.match(faqSource, /observer\.disconnect\(\)/);
	assert.match(
		faqSource,
		/<details[\s\S]*?className="sp-faq-item"[\s\S]*?name="seller-pricing-faq"/,
	);
	assert.match(
		faqSource,
		/<summary>[\s\S]*?<h4>[\s\S]*?className="sp-faq-item__toggle"[\s\S]*?aria-hidden="true"/,
	);
	assert.match(faqSource, /className="sp-faq-item__answer"/);
	assert.doesNotMatch(faqSource, /<button[^>]*className="sp-faq-item/);

	assert.equal(
		cssDeclaration(cssRule(css, ".sp-faq__nav-link"), "min-height"),
		"64px",
	);
	assert.match(
		cssRule(css, '.sp-faq__nav-link[aria-current="location"]'),
		/outline|box-shadow|border-color/,
	);
	const faqCountRule = cssRule(css, ".sp-faq__nav-count");
	assert.equal(cssDeclaration(faqCountRule, "min-width"), "36px");
	assert.doesNotMatch(faqCountRule, /border:/);
	assert.match(
		css,
		/@media \(hover: hover\) and \(pointer: fine\)[\s\S]*?\.sp-faq__nav-link:not\(\[aria-current="location"\]\):hover\s*\{[\s\S]*?border-color:\s*transparent;[\s\S]*?background:/,
	);
	assert.equal(
		cssDeclaration(cssRule(css, ".sp-faq-item"), "border"),
		"1px solid var(--sp-border)",
	);
	assert.equal(
		cssDeclaration(cssRule(css, ".sp-faq-item"), "border-radius"),
		"var(--radius-lg)",
	);
	assert.match(
		cssRule(css, ".sp-faq-item[open]"),
		/border-color:\s*var\(--sp-accent-border\)/,
	);
	assert.equal(
		cssDeclaration(cssRule(css, ".sp-faq-item__toggle"), "width"),
		"36px",
	);
	assert.equal(
		cssDeclaration(cssRule(css, ".sp-faq-item__toggle"), "height"),
		"36px",
	);
	assert.match(
		css,
		/@supports selector\(details::details-content\)[\s\S]*?\.sp-faq-item::details-content[\s\S]*?block-size:\s*0;[\s\S]*?\.sp-faq-item\[open\]::details-content[\s\S]*?block-size:\s*auto;/,
	);
	assert.match(
		css,
		/@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.seller-pricing-page \.sp-faq-item::details-content[\s\S]*?transition:\s*none;/,
	);
	assert.match(
		css,
		/@media \(forced-colors: active\)[\s\S]*?\.sp-faq__nav-link\[aria-current="location"\],[\s\S]*?\.sp-faq-item\[open\][\s\S]*?outline:\s*2px solid Highlight;/,
	);
});

test("the client pricing surface is isolated from legacy three-plan data", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const effects = source("features/seller/pricing/SellerPricingEffects.ts");
	const publicData = source("features/seller/pricing/public-data.ts");
	const metadata = source("app/[locale]/(seller)/seller/pricing/page.tsx");
	const publicRuntimeSources = `${page}\n${effects}\n${metadata}`;

	assert.match(page, /from "\.\/public-data"/);
	assert.match(metadata, /from "@\/features\/seller\/pricing\/public-data"/);
	assert.doesNotMatch(page, /from "\.\/data"/);
	assert.doesNotMatch(publicData, /from "\.\/data"/);
	assert.doesNotMatch(publicRuntimeSources, /\bgrowth\b/i);
	assert.doesNotMatch(publicRuntimeSources, /(?<![\d,])990(?!\d)/);
	assert.doesNotMatch(publicRuntimeSources, /3%/);
	assert.doesNotMatch(
		page,
		/\bSELLER_PLANS\b|\bSELLER_PRICING_FAQS\b|\bSERVICE_BUNDLES\b/,
	);
});

test("the desktop hero budgets one viewport for the navbar and page gutters", () => {
	const css = source("styles/features/seller-pricing.css");
	const pageRule = cssRule(css, ".seller-pricing-page");
	const heroRule = cssRule(css, ".sp-hero");
	const heroGridRule = cssRule(css, ".sp-hero__grid");
	const mediaRule = cssRule(css, ".sp-hero__media-mask");

	assert.match(
		cssDeclaration(pageRule, "--sp-top-offset"),
		/--floating-nav-height[\s\S]*?--floating-nav-top-gap/,
	);
	assert.equal(cssDeclaration(pageRule, "--sp-page-edge-gutter"), "18px");
	assert.equal(cssDeclaration(pageRule, "--sp-page-block-gutters"), "36px");
	assert.equal(
		cssDeclaration(pageRule, "margin-top"),
		"calc(var(--sp-top-offset) * -1)",
	);
	assert.match(
		cssDeclaration(pageRule, "padding-block"),
		/^calc\(var\(--sp-top-offset\) \+ var\(--sp-page-edge-gutter\)\)[\s\S]*?var\(--sp-page-edge-gutter\)$/,
	);
	assert.match(
		css,
		/@media \(max-width: 767px\)[\s\S]*?\.seller-pricing-page\s*\{[\s\S]*?--sp-page-edge-gutter:\s*10px;[\s\S]*?--sp-page-block-gutters:\s*20px;/,
	);
	assert.equal(cssDeclaration(heroRule, "display"), "grid");
	assert.equal(cssDeclaration(heroRule, "box-sizing"), "border-box");
	assert.match(
		cssDeclaration(heroRule, "min-height"),
		/100dvh[\s\S]*?--floating-nav-height[\s\S]*?--floating-nav-top-gap[\s\S]*?--sp-page-block-gutters/,
	);
	assert.equal(cssDeclaration(heroRule, "align-items"), "center");
	assert.equal(cssDeclaration(heroGridRule, "min-height"), "0");
	assert.match(
		cssDeclaration(mediaRule, "width"),
		/100dvh[\s\S]*?--floating-nav-height[\s\S]*?--sp-page-block-gutters/,
	);
});

test("the public page renders four concise sections with the add-on inside plans", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const sectionMarkers = [
		'<section className="sp-hero"',
		"<PlanSection",
		"<PricingFaqSection",
		"<ApplicationSection",
	];
	const positions = sectionMarkers.map((marker) => {
		const position = page.indexOf(marker);
		assert.notEqual(position, -1, `missing section marker ${marker}`);
		return position;
	});
	assert.deepEqual(
		positions,
		[...positions].sort((a, b) => a - b),
	);

	assert.match(page, /小農、鮮食與食品品牌的自助開店市集/);
	assert.match(page, /先建立賣場，再依步驟完成必要審核/);
	assert.match(page, /href="\/seller\/onboarding"/);
	assert.match(page, /開始建立賣場/);
	assert.match(page, /先看方案與費用/);
	assert.match(page, /自助開店不必先填這張表/);
	assert.doesNotMatch(page, /登入查看目前方案/);
	assert.match(page, /Hawkr 只有兩個方案。/);
	assert.match(page, /PUBLIC_SELLER_PLANS\.map/);
	assert.match(page, /PUBLIC_MANAGED_SERVICE_ADD_ON/);
	const planSource = page.slice(
		page.indexOf("function PlanSection"),
		page.indexOf("function PricingFaqSection"),
	);
	const managedSource = page.slice(
		page.indexOf("function ManagedServiceOption"),
		page.indexOf("function PlanSection"),
	);
	assert.match(planSource, /<ManagedServiceOption/);
	assert.match(managedSource, /className="sp-managed-service"/);
	assert.match(managedSource, /<fieldset/);
	assert.match(managedSource, /<legend className="visually-hidden">/);
	assert.doesNotMatch(managedSource, /<section/);
	assert.doesNotMatch(page, /ManagedServiceSection/);
	assert.doesNotMatch(
		page,
		/ResponsibilitySection|RESPONSIBILITY_ITEMS|方案不同，責任不變。/,
	);
	assert.match(page, /PUBLIC_SELLER_PRICING_FAQ_CATEGORIES\.map/);
	assert.match(page, /PUBLIC_SELLER_PRICING_FAQS\.filter/);
	assert.match(page, /questions\.map/);
	assert.match(page, /placeholders\/vision\.svg/);
	assert.match(page, /data-sp-reveal/);
	assert.match(page, /data-sp-spotlight/);
	assert.doesNotMatch(page, /data-sp-glare/);
	assert.match(page, /<details/);
	assert.match(page, /name="seller-pricing-faq"/);
	assert.doesNotMatch(page, /sp-page-bg|page-bg-tall(?:-mobile)?\.webp/);
	assert.doesNotMatch(
		page,
		/FeeCalculator|CollaborationComparison|ServiceBundlesSection|BundleChooserSection|StickyApplyBar/,
	);
	assert.doesNotMatch(
		`${page}\n${source("features/seller/pricing/SellerPricingEffects.ts")}`,
		/window\.addEventListener\("scroll"/,
	);
});

test("pricing effects progressively enhance and fail open", () => {
	const effects = source("features/seller/pricing/SellerPricingEffects.ts");
	const css = source("styles/features/seller-pricing.css");
	const notice = source("third_party/react-bits/NOTICE.md");

	assert.match(effects, /React\.useLayoutEffect/);
	assert.match(effects, /typeof IntersectionObserver === "undefined"/);
	assert.match(effects, /root\.dataset\.spEffects = "static"/);
	assert.match(effects, /showAllReveals\(\)/);
	assert.match(effects, /prefers-reduced-motion: reduce/);
	assert.match(effects, /\(hover: hover\) and \(pointer: fine\)/);
	assert.match(effects, /requestAnimationFrame/);
	assert.match(effects, /cancelAnimationFrame/);
	assert.match(effects, /revealObserver\?\.disconnect\(\)/);
	assert.match(effects, /resizeObserver\?\.disconnect\(\)/);
	assert.doesNotMatch(effects, /addEventListener\("scroll"/);

	assert.match(css, /data-sp-reveal-state="pending"/);
	assert.match(css, /data-sp-spotlight/);
	assert.doesNotMatch(css, /data-sp-glare|@keyframes sp-glare-pass/);
	assert.match(css, /@media \(prefers-reduced-motion: reduce\)/);
	assert.match(css, /@media print/);
	assert.match(css, /@media \(forced-colors: active\)/);
	assert.match(notice, /React Bits/);
	assert.match(notice, /MIT \+ Commons Clause/);
});

test("native plan and billing radios switch fees without remounting their note", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const css = source("styles/features/seller-pricing.css");
	const cardSource = page.slice(
		page.indexOf("function PricingCard"),
		page.indexOf("function BillingCycleToggle"),
	);
	const billingSource = page.slice(
		page.indexOf("function BillingCycleToggle"),
		page.indexOf("function continueToApplication"),
	);

	assert.match(
		cardSource,
		/<input[\s\S]*?className="sp-plan-card__radio visually-hidden"[\s\S]*?type="radio"[\s\S]*?name="seller-plan"[\s\S]*?checked=\{isSelected\}/,
	);
	assert.match(
		cardSource,
		/<label[\s\S]*?className="sp-plan-card__hit-area"[\s\S]*?htmlFor=\{inputId\}[\s\S]*?onClick=\{\(event\) => \{/,
	);
	assert.match(
		cardSource,
		/const inputRef = React\.useRef<HTMLInputElement>\(null\)/,
	);
	assert.match(cardSource, /event\.preventDefault\(\)/);
	assert.match(
		cardSource,
		/inputRef\.current\?\.focus\(\{ preventScroll: true \}\)/,
	);
	assert.match(cardSource, /if \(!isSelected\) inputRef\.current\?\.click\(\)/);
	assert.match(
		cardSource,
		/className="sp-plan-card__select-indicator"[\s\S]*?<AppIcon name="check" size=\{20\} \/>/,
	);
	assert.match(cardSource, /onSelectPlan\(plan\.id\)/);
	assert.doesNotMatch(cardSource, /href="#seller-application"/);
	assert.equal((billingSource.match(/type="radio"/g) ?? []).length, 2);
	assert.equal(
		(billingSource.match(/name="pricing-billing-cycle"/g) ?? []).length,
		2,
	);
	assert.match(billingSource, /value="monthly"/);
	assert.match(billingSource, /checked=\{value === "monthly"\}/);
	assert.match(billingSource, /value="annual"/);
	assert.match(billingSource, /checked=\{value === "annual"\}/);
	assert.doesNotMatch(billingSource, /<button/);

	assert.match(cardSource, /isAnnual \? "年繳方案費" : "方案月費"/);
	assert.match(cardSource, /<dt>成交服務費<\/dt>/);
	assert.match(
		cardSource,
		/const fixedFee = isAnnual \? plan\.annualFee : plan\.monthlyFee/,
	);
	assert.match(cardSource, /formatTwd\(fixedFee\)/);
	assert.match(cardSource, /formatTwd\(plan\.annualSavings\)/);
	assert.match(cardSource, /effectiveMonthlyFee\(plan\.annualFee\)/);
	assert.match(cardSource, /formatRate\(plan\.transactionFeeRate\)/);
	assert.match(cardSource, /\{plan\.contractName\}/);
	assert.match(cardSource, /plan\.zeroSalesNote/);
	assert.match(cardSource, /plan\.excluded/);
	assert.match(
		cardSource,
		/<div className="sp-plan-card__billing-note" data-cycle=\{billingCycle\}>/,
	);
	assert.equal((cardSource.match(/<p data-active=/g) ?? []).length, 2);
	assert.match(cardSource, /aria-hidden=\{isAnnual\}/);
	assert.match(cardSource, /aria-hidden=\{!isAnnual\}/);
	assert.doesNotMatch(cardSource, /sp-plan-card__annual-note/);

	assert.equal(
		cssDeclaration(
			cssRule(css, ".sp-plan-card__pricing"),
			"grid-template-columns",
		),
		"repeat(2, minmax(0, 1fr))",
	);
	assert.match(
		cssDeclaration(cssRule(css, ".sp-plan-card__pricing dd"), "font-size"),
		/clamp\(/,
	);
});

test("plan typography stays semantic, finite, and accessible across display modes", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const css = source("styles/features/seller-pricing.css");
	const cardSource = page.slice(
		page.indexOf("function PricingCard"),
		page.indexOf("function BillingCycleToggle"),
	);
	const reducedMotion = css.slice(
		css.indexOf("@media (prefers-reduced-motion: reduce)"),
		css.indexOf("@media print"),
	);
	const forcedColors = css.slice(css.indexOf("@media (forced-colors: active)"));

	assert.match(cardSource, /plan\.name\.split\(" "\)/);
	assert.match(cardSource, /planNameLocalParts\.join\(" "\)/);
	assert.match(
		cardSource,
		/<h3 className="sp-plan-card__title">[\s\S]*?className="sp-plan-card__title-latin"[\s\S]*?\{planNameLatin\}[\s\S]*?className="sp-plan-card__title-local"[\s\S]*?\{planNameLocal\}[\s\S]*?<\/h3>/,
	);
	assert.match(
		cardSource,
		/className="sp-plan-card__price-value sp-plan-card__price-value--fixed"/,
	);
	assert.match(cardSource, /className="sp-plan-card__price-number"/);
	assert.match(cardSource, /className="sp-plan-card__price-affix"/);

	const priceValueRule = cssRule(css, ".sp-plan-card__price-value");
	assert.equal(
		cssDeclaration(priceValueRule, "font-variant-numeric"),
		"lining-nums tabular-nums",
	);
	assert.match(
		cssDeclaration(priceValueRule, "font-feature-settings"),
		/"lnum" 1,[\s\S]*?"tnum" 1/,
	);
	assert.match(
		cssRule(css, ".sp-plan-card__title-local::after"),
		/transition:[\s\S]*?opacity 220ms[\s\S]*?transform 280ms/,
	);
	assert.match(
		css,
		/@media \(hover: hover\) and \(pointer: fine\)[\s\S]*?:where\(\.sp-plan-card:hover\) \.sp-plan-card__title-local::after\s*\{[\s\S]*?transform:\s*scaleX\(1\) skewX\(-7deg\);/,
	);
	assert.match(
		css,
		/:where\([\s\S]*?\.sp-plan-card\[data-selected="true"\][\s\S]*?\)\s*\.sp-plan-card__title-local::after\s*\{[\s\S]*?transform:\s*scaleX\(1\) skewX\(-7deg\);/,
	);
	assert.match(
		css,
		/\.sp-plan-card\[data-billing="monthly"\][\s\S]*?\.sp-plan-card__price-value--fixed\s*\{[\s\S]*?animation:\s*sp-pricing-monthly 280ms[^;]+;/,
	);
	assert.match(
		css,
		/\.sp-plan-card\[data-billing="annual"\][\s\S]*?\.sp-plan-card__price-value--fixed\s*\{[\s\S]*?animation:\s*sp-pricing-annual 280ms[^;]+;/,
	);
	assert.match(css, /@keyframes sp-pricing-monthly\s*\{/);
	assert.match(css, /@keyframes sp-pricing-annual\s*\{/);
	assert.match(
		reducedMotion,
		/\.seller-pricing-page \.sp-plan-card__title-latin,[\s\S]*?\.seller-pricing-page \.sp-plan-card__title-local::after,[\s\S]*?transition:\s*none;/,
	);
	assert.match(
		reducedMotion,
		/\.sp-plan-card__price-value--fixed,[\s\S]*?animation:\s*none;/,
	);
	assert.match(
		forcedColors,
		/\.sp-plan-card__title-local::after,[\s\S]*?display:\s*none;/,
	);
});

test("selection handoff, managed add-on, and recap use stable native interactions", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const css = source("styles/features/seller-pricing.css");
	const continueSource = page.slice(
		page.indexOf("function continueToApplication"),
		page.indexOf("function ManagedServiceOption"),
	);
	const managedSource = page.slice(
		page.indexOf("function ManagedServiceOption"),
		page.indexOf("function PlanSection"),
	);
	const planSource = page.slice(
		page.indexOf("function PlanSection"),
		page.indexOf("function PricingFaqSection"),
	);
	const applicationSource = page.slice(
		page.indexOf("function ApplicationSection"),
		page.indexOf("export default function SellerPricingPage"),
	);

	assert.match(
		continueSource,
		/document\.getElementById\("sp-application-title"\)/,
	);
	assert.match(continueSource, /prefers-reduced-motion: reduce/);
	assert.match(continueSource, /title\.focus\(\{ preventScroll: true \}\)/);
	assert.match(continueSource, /title\.scrollIntoView\(\{/);
	assert.match(continueSource, /behavior: reduceMotion \? "auto" : "smooth"/);
	assert.match(planSource, /<fieldset className="sp-plan-chooser">/);
	assert.doesNotMatch(
		css,
		/\.sp-plan-card__select-indicator \.hk-app-icon path:first-child/,
	);
	assert.match(planSource, /<div className="sp-plan-action">/);
	assert.match(planSource, /<p aria-live="polite" aria-atomic="true">/);
	assert.match(planSource, /type="button"/);
	assert.match(planSource, /disabled=\{selectedPlan === "undecided"\}/);
	assert.match(
		planSource,
		/continueToApplication\(selectedPlan, billingCycle\)/,
	);
	assert.match(planSource, /請團隊協助評估/);
	assert.doesNotMatch(planSource, /href="#seller-application"/);
	assert.match(planSource, /<ManagedServiceOption/);

	assert.match(
		managedSource,
		/<label[\s\S]*?className="sp-managed-service__toggle"[\s\S]*?<input[\s\S]*?type="checkbox"[\s\S]*?checked=\{isSelected\}/,
	);
	assert.match(managedSource, /event\.target\.checked \? addOn\.id : "none"/);
	assert.match(
		managedSource,
		/const inputRef = React\.useRef<HTMLInputElement>\(null\)/,
	);
	assert.match(managedSource, /event\.preventDefault\(\)/);
	assert.match(
		managedSource,
		/inputRef\.current\?\.focus\(\{ preventScroll: true \}\)/,
	);
	assert.match(managedSource, /inputRef\.current\?\.click\(\)/);
	assert.match(managedSource, /id="managed-service"/);
	assert.match(managedSource, /<fieldset/);
	assert.doesNotMatch(managedSource, /<section/);
	assert.doesNotMatch(managedSource, /href="#seller-application"/);
	assert.doesNotMatch(
		css,
		/\.sp-managed-service__toggle-indicator \.hk-app-icon path:first-child/,
	);
	assert.equal(
		cssDeclaration(cssRule(css, ".sp-managed-service__toggle"), "min-height"),
		"64px",
	);
	assert.match(
		css,
		/\.sp-managed-service__toggle:is\([\s\S]*?:has\(input:checked\)\)/,
	);

	assert.match(applicationSource, /className="sp-selection-recap"/);
	assert.match(applicationSource, /role="status"/);
	assert.match(applicationSource, /aria-live="polite"/);
	assert.match(applicationSource, /aria-atomic="true"/);
	assert.match(applicationSource, /aria-label="目前選擇"/);
	for (const label of ["方案", "付款", "加值"]) {
		assert.match(applicationSource, new RegExp(`<span>${label}<\\/span>`));
	}
	for (const value of ["selectedPlanName", "billingLabel", "serviceLabel"]) {
		assert.match(applicationSource, new RegExp(`\\{${value}\\}`));
	}
});

test("metadata leads with self-service onboarding while FAQ schema stays complete", () => {
	const route = source("app/[locale]/(seller)/seller/pricing/page.tsx");

	assert.match(route, /Hawkr 自助開店/);
	assert.match(route, /先建立賣場，完成必要審核後開始販售/);
	assert.match(route, /Free、Pro 方案、費用與平台／賣家分工/);
	assert.match(route, /path: "\/seller\/pricing"/);
	assert.match(route, /breadcrumbSchema\(\[/);
	assert.match(route, /faqSchema\(/);
	assert.match(route, /PUBLIC_SELLER_PRICING_FAQS\.map/);
});

test("the concise lead form sends plan, billing cycle, and managed-service interest", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const leadSubmission = source("features/seller/pricing/lead-submission.ts");
	const leadState = page.slice(
		page.indexOf("type LeadState"),
		page.indexOf("const EMPTY_LEAD"),
	);

	for (const field of [
		"brandName",
		"contactName",
		"phone",
		"email",
		"productType",
	]) {
		assert.match(leadState, new RegExp(`\\b${field}: string`));
	}
	assert.doesNotMatch(
		leadState,
		/salesChannel|monthlyOrders|businessSize|address|serviceInterest/,
	);
	assert.equal((page.match(/\bisRequired\b/g) ?? []).length, 3);
	assert.match(
		page,
		/const hasContact = lead\.email\.trim\(\) !== "" \|\| lead\.phone\.trim\(\) !== ""/,
	);
	assert.match(page, /Email 或手機（至少填一項）/);
	assert.match(page, /name="plan"/);
	assert.match(page, /name="billingCycle"/);
	assert.match(page, /value="monthly"/);
	assert.match(page, /value="annual"/);
	assert.match(page, /name="serviceInterest"/);
	assert.match(page, /value="none"/);
	assert.match(page, /value="managed"/);
	assert.match(page, /React\.useState<PublicBillingCycle>\("monthly"\)/);
	assert.match(page, /React\.useState<SellerLeadServiceInterest>\("none"\)/);
	assert.match(page, /PUBLIC_SELLER_PLANS\.map\(\(plan\) =>/);
	assert.match(page, /fetch\("\/api\/public\/seller-leads"/);
	assert.match(
		page,
		/submitSellerLeadRequest\([\s\S]*?\.\.\.lead,[\s\S]*?plan: planAtSubmission,[\s\S]*?billingCycle: billingAtSubmission,[\s\S]*?serviceInterest: serviceAtSubmission/,
	);
	assert.match(page, /body: JSON\.stringify\(submission\)/);
	assert.match(leadSubmission, /deps\.post\(submission\)/);
	assert.doesNotMatch(page, /docs\.google\.com\/forms|mode: "no-cors"/);
});

test("the server accepts legacy Growth without exposing it as a public option", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const route = source("app/api/public/seller-leads/route.ts");

	assert.match(
		route,
		/z\s*\.enum\(\["starter", "growth", "pro", "undecided"\]\)/,
	);
	assert.match(route, /growth: "Growth 成長（舊版提交，需人工確認）"/);
	assert.match(
		route,
		/starter: "Free 免費（STARTER；月費 0 元＋成交服務費 6%）"/,
	);
	assert.match(route, /pro: "Pro 專業（成交服務費 0%）"/);
	assert.match(
		route,
		/billingCycle: z\.enum\(\["monthly", "annual"\]\)\.optional\(\)\.default\("monthly"\)/,
	);
	assert.match(
		route,
		/\.enum\(\["none", "managed", "launch", "content", "full"\]\)/,
	);
	assert.match(route, /\.default\("none"\)/);
	assert.doesNotMatch(page, /value="growth"|growth:/i);
});

test("the server derives billing labels from both plan and cycle", () => {
	const route = source("app/api/public/seller-leads/route.ts");
	const starterBranch = route.slice(
		route.indexOf('if (plan === "starter")'),
		route.indexOf('if (plan === "growth")'),
	);

	assert.match(route, /function billingCycleLabel\(/);
	assert.doesNotMatch(route, /BILLING_CYCLE_LABELS/);
	assert.match(route, /"月繳（NT\$2,990／月）"/);
	assert.match(route, /"年繳（NT\$29,900／年，省 NT\$5,980）"/);
	assert.match(route, /"Free 無固定月費"/);
	assert.match(route, /"Free 無固定方案年費（Pro 年繳優惠不適用）"/);
	assert.match(route, /月繳"}偏好（舊版方案，需人工確認）/);
	assert.match(route, /月繳"}偏好（方案與金額待確認）/);
	assert.match(
		route,
		/付款週期：\$\{billingCycleLabel\(lead\.plan, lead\.billingCycle\)\}/,
	);
	assert.doesNotMatch(starterBranch, /NT\$29,900|NT\$5,980/);
});

test("seller recruitment and shared navigation retain pricing entry points", () => {
	const sellerLanding = source("features/seller/landing/SellerLandingPage.tsx");
	const footer = source("components/shared/Footer.tsx");
	const sellerFooter = source("components/shared/SellerFooter.tsx");

	assert.ok(
		(sellerLanding.match(/href="\/seller\/pricing"/g) ?? []).length >= 2,
		"seller recruitment should retain both pricing entry points",
	);
	assert.match(footer, /href="\/seller\/pricing"[\s\S]{0,100}合作方案/);
	assert.match(sellerFooter, /label: "合作方案", href: "\/seller\/pricing"/);
});

test("the responsive surface keeps equal card geometry and a stable billing-note overlay", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const css = source("styles/features/seller-pricing.css");
	const planSource = page.slice(
		page.indexOf("function PlanSection"),
		page.indexOf("function PricingFaqSection"),
	);
	const mobile = css.slice(css.indexOf("@media (max-width: 767px)"));
	const gridRule = cssRule(css, ".sp-plan-grid");
	const anchorRule = cssRule(css, ".sp-plan-anchor");
	const cardRule = cssRule(css, ".sp-plan-card");
	const noteRule = cssRule(css, ".sp-plan-card__billing-note");
	const noteParagraphRule = cssRule(css, ".sp-plan-card__billing-note p");
	const activeNoteRule = cssRule(
		css,
		'.sp-plan-card__billing-note p[data-active="true"]',
	);
	const planAnchorOpeningTag = planSource.match(
		/<div\s+id=\{`plan-\$\{plan\.id\}`\}[\s\S]*?>/,
	)?.[0];
	const planGridOpeningTag = planSource.match(
		/<div\s+className="sp-plan-grid"[\s\S]*?>/,
	)?.[0];
	assert.ok(planAnchorOpeningTag, "missing plan anchor opening tag");
	assert.ok(planGridOpeningTag, "missing plan grid opening tag");
	assert.match(planGridOpeningTag, /data-sp-reveal="up"/);
	assert.doesNotMatch(planAnchorOpeningTag, /data-sp-reveal|data-sp-delay/);

	assert.equal(
		cssDeclaration(gridRule, "grid-template-columns"),
		"repeat(2, minmax(0, 1fr))",
	);
	assert.equal(cssDeclaration(gridRule, "grid-auto-rows"), "1fr");
	assert.equal(cssDeclaration(gridRule, "align-items"), "stretch");
	assert.equal(cssDeclaration(anchorRule, "display"), "flex");
	assert.equal(cssDeclaration(anchorRule, "align-self"), "stretch");
	assert.equal(cssDeclaration(anchorRule, "margin-block"), "0");
	assert.equal(cssDeclaration(cardRule, "width"), "100%");
	assert.equal(cssDeclaration(cardRule, "height"), "100%");
	assert.equal(
		cssDeclaration(cardRule, "border"),
		"1px solid var(--sp-border)",
	);
	assert.doesNotMatch(css, /\.sp-plan-anchor:first-child/);
	assert.doesNotMatch(css, /\.sp-plan-card\[data-emphasis=/);
	assert.doesNotMatch(page, /data-emphasis=/);

	assert.equal(cssDeclaration(noteRule, "display"), "grid");
	assert.equal(cssDeclaration(noteRule, "min-height"), "62px");
	assert.equal(cssDeclaration(noteParagraphRule, "grid-area"), "1 / 1");
	assert.equal(cssDeclaration(noteParagraphRule, "opacity"), "0");
	assert.equal(cssDeclaration(noteParagraphRule, "visibility"), "hidden");
	assert.equal(cssDeclaration(activeNoteRule, "opacity"), "1");
	assert.equal(cssDeclaration(activeNoteRule, "visibility"), "visible");
	assert.doesNotMatch(css, /sp-plan-card__annual-note/);

	const pageCanvasRule = cssRule(css, ".seller-pricing-page");
	assert.equal(cssDeclaration(pageCanvasRule, "background"), "transparent");
	assert.doesNotMatch(css, /\.sp-page-bg/);
	assert.match(
		mobile,
		/\.sp-plan-grid\s*\{[\s\S]*?grid-template-columns:\s*1fr;/,
	);
	assert.match(css, /@media \(prefers-reduced-motion: reduce\)/);
	assert.match(css, /@media \(forced-colors: active\)/);
	assert.doesNotMatch(css, /#[0-9a-f]{3,8}\b/i);
	assert.doesNotMatch(css, /\.sp-(?:calculator|bundle|sticky|comparison)/);
});

test("analytics cover the concise four-section funnel and new selections", () => {
	const page = source("features/seller/pricing/SellerPricingPage.tsx");
	const eventNames = new Set(
		[...page.matchAll(/trackPricing\(\s*"([^"]+)"/g)].map(
			([, eventName]) => eventName,
		),
	);

	for (const eventName of [
		"pricing_page_view",
		"pricing_plan_view",
		"billing_cycle_selected",
		"plan_selected",
		"managed_service_selected",
		"pricing_faq_category_select",
		"pricing_faq_expand",
		"seller_plan_help_click",
		"seller_contact_request_started",
		"seller_contact_request_submit",
	]) {
		assert.ok(
			eventNames.has(eventName),
			`missing analytics event ${eventName}`,
		);
	}
	for (const retiredEvent of [
		"fee_calculator_used",
		"plan_recommendation_shown",
		"add_on_inquiry_submitted",
		"bundle_chooser_used",
	]) {
		assert.equal(eventNames.has(retiredEvent), false);
	}
	for (const contextField of ["location", "device"]) {
		assert.match(page, new RegExp(`\\b${contextField}\\b`));
	}
	assert.doesNotMatch(page, /document\.referrer|\breferrer\s*:/);
});
