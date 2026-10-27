import { expect, type Page, test } from "@playwright/test";

const PUBLIC_SECTIONS = [
	"sp-hero",
	"sp-plans",
	"sp-faq",
	"sp-application",
] as const;

const PLAN_CARDS = [
	{
		id: "starter",
		name: "Free 免費",
		latinName: "Free",
		localName: "免費",
		monthlyFee: "NT$0／月",
		annualFee: "NT$0／年",
		rate: "6%／有效交易",
		radioName: "選擇 Free 免費",
	},
	{
		id: "pro",
		name: "Pro 專業",
		latinName: "Pro",
		localName: "專業",
		monthlyFee: "NT$2,990／月",
		annualFee: "NT$29,900／年",
		rate: "0%／有效交易",
		radioName: "選擇 Pro 專業",
	},
] as const;

const FAQ_CATEGORIES = [
	{
		id: "plans",
		label: "方案與費用",
		description: "Free、Pro 與年繳計算",
		count: 4,
	},
	{
		id: "service",
		label: "代行與責任",
		description: "加值服務與合作分工",
		count: 2,
	},
	{
		id: "changes",
		label: "退款與變更",
		description: "退款計費與方案調整",
		count: 2,
	},
] as const;

type PricingGeometry = {
	gridTop: number;
	gridHeight: number;
	addOnTop: number;
	addOnHeight: number;
	nextSectionTop: number;
	documentHeight: number;
	cards: Array<{
		top: number;
		width: number;
		height: number;
	}>;
};

type FaqNavigationGeometry = {
	width: number;
	height: number;
	documentHeight: number;
	links: Array<{
		top: number;
		left: number;
		width: number;
		height: number;
	}>;
	groups: Array<{
		top: number;
		height: number;
	}>;
};

function watchRuntimeErrors(page: Page) {
	const errors: string[] = [];
	page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
	page.on("console", (message) => {
		if (message.type() !== "error") return;
		// A failed subresource load (image/font/tracking beacon from an
		// external CDN) is not a JS runtime error — it's the browser's own
		// network-layer console log, distinct from an uncaught exception.
		// This function's name and every caller's intent is "did the page's
		// own code crash", not "did every third-party asset load"; folding
		// the two together made an external CDN having a bad moment
		// indistinguishable from a real regression on this page.
		if (message.text().startsWith("Failed to load resource")) return;
		errors.push(`console: ${message.text()}`);
	});
	return errors;
}

async function openPricing(page: Page) {
	await page.goto("/seller/pricing", { waitUntil: "domcontentloaded" });
	await expect(page.locator(".initial-loader")).toHaveCount(0, {
		timeout: 10_000,
	});
	await expect(page.locator(".seller-pricing-page")).toHaveAttribute(
		"data-hydrated",
		"true",
	);
	await expect(page.locator(".seller-pricing-page")).toHaveAttribute(
		"data-sp-effects",
		/enhanced|static/u,
	);
}

async function hasHorizontalOverflow(page: Page) {
	return page.evaluate(
		() =>
			document.documentElement.scrollWidth >
			document.documentElement.clientWidth + 1,
	);
}

async function finishPlanAnimations(page: Page) {
	const grid = page.locator("#plans .sp-plan-grid");
	await grid.scrollIntoViewIfNeeded();
	await expect(grid).toHaveAttribute("data-sp-reveal-state", "visible");
	await grid.evaluate(async (element) => {
		await Promise.all(
			element
				.getAnimations({ subtree: true })
				.map((animation) => animation.finished.catch(() => undefined)),
		);
	});
}

async function finishFaqAnimations(page: Page) {
	const faq = page.locator("#faq");
	await faq.evaluate(async (element) => {
		await Promise.all(
			element
				.getAnimations({ subtree: true })
				.map((animation) => animation.finished.catch(() => undefined)),
		);
	});
}

async function readPricingGeometry(page: Page): Promise<PricingGeometry> {
	return page.evaluate(() => {
		// 版面位置：沿 offsetParent 鏈累加 offsetTop，得到不含 CSS transform 的文件
		// 座標。reveal 動效以 translate3d 位移元素，getBoundingClientRect 會把位移一
		// 併算進去，於是過渡期間「畫出來的位置」與「版面位置」不同；這些斷言要問的是
		// 「選方案有沒有造成重排」，量的必須是後者。
		//
		// 一定要累加到根：.sp-plan-anchor 是 position: relative，單取 offsetTop 會讓
		// 兩張卡片各自相對自己的 anchor（同為 0），expectPlanCardsAligned 就恆真了。
		const layoutTop = (element: HTMLElement) => {
			let top = 0;
			let node: HTMLElement | null = element;
			while (node) {
				top += node.offsetTop;
				node = node.offsetParent as HTMLElement | null;
			}
			return top;
		};
		const grid = document.querySelector<HTMLElement>(".sp-plan-grid");
		const addOn = document.querySelector<HTMLElement>(
			"#plans #managed-service",
		);
		const nextSection = document.querySelector<HTMLElement>(".sp-faq");
		const cards = [
			...document.querySelectorAll<HTMLElement>("#plans .sp-plan-card"),
		];
		if (!grid || !addOn || !nextSection || cards.length !== 2) {
			throw new Error("pricing geometry is incomplete");
		}
		return {
			gridTop: layoutTop(grid),
			gridHeight: grid.getBoundingClientRect().height,
			addOnTop: layoutTop(addOn),
			addOnHeight: addOn.getBoundingClientRect().height,
			nextSectionTop: layoutTop(nextSection),
			documentHeight: document.documentElement.scrollHeight,
			cards: cards.map((card) => {
				const bounds = card.getBoundingClientRect();
				return {
					top: layoutTop(card),
					width: bounds.width,
					height: bounds.height,
				};
			}),
		};
	});
}

// #managed-service 帶 data-sp-reveal="up"，由 IntersectionObserver 觸發
// pending(translate3d(0, 22px, 0)) → visible(transform: none) 的 480ms 過渡。捲動把
// 它帶進視窗的那一刻起，前後兩次量測會落在過渡曲線的不同點上。等的是可觀測終態
// （transform 已收斂成 none），不是固定秒數，所以不隨 runner 快慢改變。
async function settleManagedServiceReveal(page: Page) {
	const addOn = page.locator("#plans #managed-service");
	await expect(addOn).toHaveAttribute("data-sp-reveal-state", "visible");
	await expect
		.poll(() =>
			addOn.evaluate((element) => getComputedStyle(element).transform),
		)
		.toBe("none");
}

async function readFaqNavigationGeometry(
	page: Page,
): Promise<FaqNavigationGeometry> {
	return page.evaluate(() => {
		const navigation = document.querySelector<HTMLElement>(".sp-faq__nav");
		const links = [
			...document.querySelectorAll<HTMLElement>(".sp-faq__nav-link"),
		];
		const groups = [...document.querySelectorAll<HTMLElement>(".sp-faq-group")];
		if (!navigation || links.length !== 3 || groups.length !== 3) {
			throw new Error("FAQ navigation geometry is incomplete");
		}
		const navigationBounds = navigation.getBoundingClientRect();
		return {
			width: navigationBounds.width,
			height: navigationBounds.height,
			documentHeight: document.documentElement.scrollHeight,
			links: links.map((link) => {
				const bounds = link.getBoundingClientRect();
				return {
					top: bounds.top - navigationBounds.top,
					left: bounds.left - navigationBounds.left,
					width: bounds.width,
					height: bounds.height,
				};
			}),
			groups: groups.map((group) => {
				const bounds = group.getBoundingClientRect();
				return {
					top: bounds.top + window.scrollY,
					height: bounds.height,
				};
			}),
		};
	});
}

function expectWithinOnePixel(actual: number, expected: number) {
	expect(Math.abs(actual - expected)).toBeLessThanOrEqual(1);
}

function expectFaqNavigationGeometryStable(
	actual: FaqNavigationGeometry,
	expected: FaqNavigationGeometry,
) {
	expectWithinOnePixel(actual.width, expected.width);
	expectWithinOnePixel(actual.height, expected.height);
	expectWithinOnePixel(actual.documentHeight, expected.documentHeight);
	for (const [index, expectedLink] of expected.links.entries()) {
		const actualLink = actual.links[index];
		expect(actualLink).toBeDefined();
		expectWithinOnePixel(actualLink?.top ?? 0, expectedLink.top);
		expectWithinOnePixel(actualLink?.left ?? 0, expectedLink.left);
		expectWithinOnePixel(actualLink?.width ?? 0, expectedLink.width);
		expectWithinOnePixel(actualLink?.height ?? 0, expectedLink.height);
	}
	for (const [index, expectedGroup] of expected.groups.entries()) {
		const actualGroup = actual.groups[index];
		expect(actualGroup).toBeDefined();
		expectWithinOnePixel(actualGroup?.top ?? 0, expectedGroup.top);
		expectWithinOnePixel(actualGroup?.height ?? 0, expectedGroup.height);
	}
}

function expectPlanCardsAligned(geometry: PricingGeometry) {
	const [freeCard, proCard] = geometry.cards;
	expect(freeCard).toBeDefined();
	expect(proCard).toBeDefined();
	if (!freeCard || !proCard) return;
	expectWithinOnePixel(freeCard.top, proCard.top);
	expectWithinOnePixel(freeCard.height, proCard.height);
}

async function expectScrollPosition(page: Page, expected: number) {
	await expect
		.poll(() => page.evaluate(() => window.scrollY))
		.toBeGreaterThanOrEqual(expected - 1);
	await expect
		.poll(() => page.evaluate(() => window.scrollY))
		.toBeLessThanOrEqual(expected + 1);
}

test.describe("seller pricing Free and Pro", { tag: ["@ui"] }, () => {
	test("desktop publishes two plans, an embedded add-on and the four-section hierarchy", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "desktop project only");
		const runtimeErrors = watchRuntimeErrors(page);
		await page.setViewportSize({ width: 1280, height: 720 });

		await openPricing(page);

		await expect(page.getByRole("heading", { level: 1 })).toHaveText(
			"小農、鮮食與食品品牌的自助開店市集",
		);
		await expect(
			page.getByRole("link", { name: "開始建立賣場", exact: true }),
		).toHaveAttribute("href", "/seller/onboarding");
		await expect(
			page.getByRole("link", { name: "先看方案與費用" }),
		).toHaveAttribute("href", "#plans");

		const heroViewport = await page.evaluate(() => {
			const navigation = document.querySelector<HTMLElement>(".auto-nav");
			const pricingPage = document.querySelector<HTMLElement>(
				".seller-pricing-page",
			);
			const pageBackground =
				document.querySelector<HTMLElement>(".site-grainient");
			const hero = document.querySelector<HTMLElement>(".sp-hero");
			const copy = document.querySelector<HTMLElement>(".sp-hero__copy");
			const media = document.querySelector<HTMLElement>(".sp-hero__media-mask");
			if (
				!navigation ||
				!pricingPage ||
				!pageBackground ||
				!hero ||
				!copy ||
				!media
			) {
				throw new Error("pricing hero geometry is incomplete");
			}
			const pageStyle = getComputedStyle(pricingPage);
			const navigationBounds = navigation.getBoundingClientRect();
			const pricingPageBounds = pricingPage.getBoundingClientRect();
			const pageBackgroundBounds = pageBackground.getBoundingClientRect();
			const heroBounds = hero.getBoundingClientRect();
			const copyBounds = copy.getBoundingClientRect();
			const mediaBounds = media.getBoundingClientRect();
			return {
				viewportHeight: window.innerHeight,
				pageTop: pricingPageBounds.top,
				pageBackgroundTop: pageBackgroundBounds.top,
				pageBackgroundPosition: getComputedStyle(pageBackground).position,
				pageCanvas: pageStyle.backgroundColor,
				pageEdgeGutter: Number.parseFloat(
					pageStyle.getPropertyValue("--sp-page-edge-gutter"),
				),
				pagePaddingTop: Number.parseFloat(pageStyle.paddingTop),
				pagePaddingBottom: Number.parseFloat(pageStyle.paddingBottom),
				navigationBottom: navigationBounds.bottom,
				heroTop: heroBounds.top,
				heroBottom: heroBounds.bottom,
				copyTop: copyBounds.top,
				copyBottom: copyBounds.bottom,
				mediaTop: mediaBounds.top,
				mediaBottom: mediaBounds.bottom,
			};
		});
		expectWithinOnePixel(heroViewport.pageTop, 0);
		expectWithinOnePixel(heroViewport.pageBackgroundTop, 0);
		expect(heroViewport.pageBackgroundPosition).toBe("fixed");
		expect(heroViewport.pageCanvas).toBe("rgba(0, 0, 0, 0)");
		expectWithinOnePixel(
			heroViewport.pagePaddingTop,
			heroViewport.navigationBottom -
				heroViewport.pageTop +
				heroViewport.pageEdgeGutter,
		);
		expect(heroViewport.heroTop).toBeGreaterThanOrEqual(
			heroViewport.navigationBottom + heroViewport.pageEdgeGutter - 1,
		);
		expect(heroViewport.heroBottom).toBeLessThanOrEqual(
			heroViewport.viewportHeight - heroViewport.pagePaddingBottom + 1,
		);
		expect(heroViewport.copyTop).toBeGreaterThanOrEqual(
			heroViewport.heroTop - 1,
		);
		expect(heroViewport.copyBottom).toBeLessThanOrEqual(
			heroViewport.heroBottom + 1,
		);
		expect(heroViewport.mediaTop).toBeGreaterThanOrEqual(
			heroViewport.heroTop - 1,
		);
		expect(heroViewport.mediaBottom).toBeLessThanOrEqual(
			heroViewport.heroBottom + 1,
		);

		const publicSections = page.locator(".seller-pricing-page > section");
		await expect(publicSections).toHaveCount(PUBLIC_SECTIONS.length);
		expect(
			await publicSections.evaluateAll((sections) =>
				sections.map((section) =>
					[...section.classList].find(
						(className) =>
							className.startsWith("sp-") && className !== "sp-section",
					),
				),
			),
		).toEqual(PUBLIC_SECTIONS);

		const cards = page.locator("#plans .sp-plan-card");
		await expect(cards).toHaveCount(PLAN_CARDS.length);
		const planGrid = page.locator("#plans .sp-plan-grid");
		const planAnchors = planGrid.locator(".sp-plan-anchor");
		await expect(planGrid).toHaveAttribute("data-sp-reveal", "up");
		await expect(planGrid).toHaveAttribute("data-sp-delay", "1");
		await expect(planGrid).toHaveAttribute("data-sp-reveal-state", "pending");
		await expect(planAnchors).toHaveCount(PLAN_CARDS.length);
		expect(
			await planAnchors.evaluateAll((anchors) =>
				anchors.map((anchor) => ({
					hasReveal: anchor.hasAttribute("data-sp-reveal"),
					hasDelay: anchor.hasAttribute("data-sp-delay"),
					hasRevealState: anchor.hasAttribute("data-sp-reveal-state"),
				})),
			),
		).toEqual([
			{ hasReveal: false, hasDelay: false, hasRevealState: false },
			{ hasReveal: false, hasDelay: false, hasRevealState: false },
		]);
		const pendingGeometry = await readPricingGeometry(page);
		expectPlanCardsAligned(pendingGeometry);
		for (const [index, expected] of PLAN_CARDS.entries()) {
			const card = cards.nth(index);
			await expect(card).toHaveAttribute("data-plan", expected.id);
			const heading = card.getByRole("heading", { level: 3 });
			await expect(heading).toHaveText(expected.name);
			await expect(heading.locator(".sp-plan-card__title-latin")).toHaveText(
				expected.latinName,
			);
			await expect(heading.locator(".sp-plan-card__title-local")).toHaveText(
				expected.localName,
			);
			await expect(card.locator(".sp-plan-card__pricing dd")).toHaveText([
				expected.monthlyFee,
				expected.rate,
			]);
			await expect(
				page.getByRole("radio", { name: expected.radioName }),
			).not.toBeChecked();
		}

		await expect(
			page.getByRole("button", { name: "請團隊協助評估" }),
		).toBeDisabled();
		await finishPlanAnimations(page);
		const monthlyGeometry = await readPricingGeometry(page);
		expectPlanCardsAligned(monthlyGeometry);
		expectWithinOnePixel(
			monthlyGeometry.cards[0]?.width ?? 0,
			monthlyGeometry.cards[1]?.width ?? 0,
		);

		const billingToggle = page.locator(".sp-billing-cycle");
		const monthlyRadio = billingToggle.getByRole("radio", { name: "月繳" });
		const annualRadio = billingToggle.getByRole("radio", { name: /年繳/u });
		const billingSelect = page.getByLabel("付款週期");
		await expect(monthlyRadio).toBeChecked();
		await expect(annualRadio).not.toBeChecked();
		await expect(billingSelect).toHaveValue("monthly");
		const monthlyPriceMotion = await cards
			.nth(1)
			.locator(".sp-plan-card__price-value--fixed")
			.evaluate((element) => {
				const style = getComputedStyle(element);
				return {
					animationName: style.animationName,
					animationIterationCount: style.animationIterationCount,
					fontVariantNumeric: style.fontVariantNumeric,
				};
			});
		expect(monthlyPriceMotion.animationName).toBe("sp-pricing-monthly");
		expect(monthlyPriceMotion.animationIterationCount).toBe("1");
		expect(monthlyPriceMotion.fontVariantNumeric).toContain("lining-nums");
		expect(monthlyPriceMotion.fontVariantNumeric).toContain("tabular-nums");

		await billingToggle.locator('label[for="sp-billing-annual"]').click();
		await expect(monthlyRadio).not.toBeChecked();
		await expect(annualRadio).toBeChecked();
		await expect(billingSelect).toHaveValue("annual");
		for (const [index, expected] of PLAN_CARDS.entries()) {
			const card = cards.nth(index);
			await expect(card).toHaveAttribute("data-billing", "annual");
			await expect(card.locator(".sp-plan-card__pricing dd")).toHaveText([
				expected.annualFee,
				expected.rate,
			]);
		}
		await expect(
			page.locator(
				'.sp-plan-card[data-plan="pro"] .sp-plan-card__billing-note p[data-active="true"]',
			),
		).toContainText("省 NT$5,980，平均每月 NT$2,492");
		const annualPriceMotion = await cards
			.nth(1)
			.locator(".sp-plan-card__price-value--fixed")
			.evaluate((element) => {
				const style = getComputedStyle(element);
				const duration = style.animationDuration.trim();
				return {
					animationName: style.animationName,
					animationIterationCount: style.animationIterationCount,
					durationSeconds: duration.endsWith("ms")
						? Number.parseFloat(duration) / 1000
						: Number.parseFloat(duration),
				};
			});
		expect(annualPriceMotion.animationName).toBe("sp-pricing-annual");
		expect(annualPriceMotion.animationIterationCount).toBe("1");
		expect(annualPriceMotion.durationSeconds).toBeGreaterThan(0);
		expect(annualPriceMotion.durationSeconds).toBeLessThanOrEqual(0.4);
		const annualGeometry = await readPricingGeometry(page);
		expectWithinOnePixel(annualGeometry.gridTop, monthlyGeometry.gridTop);
		expectWithinOnePixel(annualGeometry.gridHeight, monthlyGeometry.gridHeight);
		expectWithinOnePixel(annualGeometry.addOnTop, monthlyGeometry.addOnTop);
		expectWithinOnePixel(
			annualGeometry.addOnHeight,
			monthlyGeometry.addOnHeight,
		);
		expectWithinOnePixel(
			annualGeometry.nextSectionTop,
			monthlyGeometry.nextSectionTop,
		);
		expectWithinOnePixel(
			annualGeometry.documentHeight,
			monthlyGeometry.documentHeight,
		);
		for (const [index, monthlyCard] of monthlyGeometry.cards.entries()) {
			const annualCard = annualGeometry.cards[index];
			expect(annualCard).toBeDefined();
			expectWithinOnePixel(annualCard?.top ?? 0, monthlyCard.top);
			expectWithinOnePixel(annualCard?.height ?? 0, monthlyCard.height);
		}
		const proSelectHint = page.locator(
			'.sp-plan-card[data-plan="pro"] .sp-plan-card__select-hint',
		);
		await proSelectHint.scrollIntoViewIfNeeded();
		const proSelectBounds = await proSelectHint.boundingBox();
		if (!proSelectBounds) throw new Error("Pro selection control is missing");
		const planPointerScrollY = await page.evaluate(() => window.scrollY);
		await settleManagedServiceReveal(page);
		const beforePlanPointerGeometry = await readPricingGeometry(page);
		await page.mouse.click(
			proSelectBounds.x + proSelectBounds.width / 2,
			proSelectBounds.y + proSelectBounds.height / 2,
		);
		await expect(
			page.getByRole("radio", { name: "選擇 Pro 專業" }),
		).toBeChecked();
		await expect(
			page.getByRole("radio", { name: "選擇 Pro 專業" }),
		).toBeFocused();
		await expectScrollPosition(page, planPointerScrollY);
		const selectedAnnualGeometry = await readPricingGeometry(page);
		expectPlanCardsAligned(selectedAnnualGeometry);
		expectWithinOnePixel(
			selectedAnnualGeometry.gridTop,
			beforePlanPointerGeometry.gridTop,
		);
		expectWithinOnePixel(
			selectedAnnualGeometry.gridHeight,
			beforePlanPointerGeometry.gridHeight,
		);
		expectWithinOnePixel(
			selectedAnnualGeometry.nextSectionTop,
			beforePlanPointerGeometry.nextSectionTop,
		);
		expectWithinOnePixel(
			selectedAnnualGeometry.addOnTop,
			beforePlanPointerGeometry.addOnTop,
		);
		expectWithinOnePixel(
			selectedAnnualGeometry.addOnHeight,
			beforePlanPointerGeometry.addOnHeight,
		);
		expectWithinOnePixel(
			selectedAnnualGeometry.documentHeight,
			beforePlanPointerGeometry.documentHeight,
		);
		for (const [
			index,
			beforeCard,
		] of beforePlanPointerGeometry.cards.entries()) {
			const selectedCard = selectedAnnualGeometry.cards[index];
			expect(selectedCard).toBeDefined();
			expectWithinOnePixel(selectedCard?.top ?? 0, beforeCard.top);
			expectWithinOnePixel(selectedCard?.width ?? 0, beforeCard.width);
			expectWithinOnePixel(selectedCard?.height ?? 0, beforeCard.height);
		}

		const managedService = page.locator("#plans #managed-service");
		await expect(managedService).toContainText("加值代行包");
		await expect(managedService).toContainText("+6%／每筆有效交易");
		await expect(
			page.locator(".seller-pricing-page > #managed-service"),
		).toHaveCount(0);
		expect(
			await managedService.evaluate(
				(element) => element.closest("section")?.id,
			),
		).toBe("plans");
		await expect(
			managedService.getByRole("checkbox", { name: /加購代行包/u }),
		).not.toBeChecked();
		// 2026-09-10：ThemeToggle 改 APG toggle（名稱固定「主題切換」+ aria-pressed）。
		await page.getByRole("button", { name: "主題切換" }).click();
		await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
		const darkIndicatorColors = await page
			.locator("#plans")
			.evaluate((plans) => {
				const colorOf = (selector: string) => {
					const element = plans.querySelector<HTMLElement>(selector);
					if (!element)
						throw new Error(`missing selection indicator: ${selector}`);
					return getComputedStyle(element).color;
				};
				return {
					free: colorOf(
						'.sp-plan-card[data-plan="starter"] .sp-plan-card__select-indicator',
					),
					pro: colorOf(
						'.sp-plan-card[data-plan="pro"] .sp-plan-card__select-indicator',
					),
					addOn: colorOf(".sp-managed-service__toggle-indicator"),
				};
			});
		expect(darkIndicatorColors.free).toBe("rgba(0, 0, 0, 0)");
		expect(darkIndicatorColors.addOn).toBe("rgba(0, 0, 0, 0)");
		expect(darkIndicatorColors.pro).not.toBe("rgba(0, 0, 0, 0)");

		const publicCopy = await page.locator(".seller-pricing-page").innerText();
		expect(publicCopy).not.toMatch(/Growth|NT\$990|成交服務費\s*3%/u);
		await expect(page.getByLabel("想了解的方案（選填）")).toHaveValue("pro");

		const annualQuestion = page
			.locator(".sp-faq-item")
			.filter({ hasText: "Pro 年繳怎麼計算？" });
		const annualSummary = annualQuestion.locator("summary");
		await expect(annualQuestion).not.toHaveAttribute("open", "");
		await annualSummary.click();
		await expect(annualQuestion).toHaveAttribute("open", "");
		await expect(annualSummary).toBeFocused();
		await expect(annualQuestion.locator(".sp-faq-item__answer")).toContainText(
			"Pro 年繳優惠價為 NT$29,900",
		);
		await expect(annualQuestion.locator(".sp-faq-item__answer")).toContainText(
			"共省 NT$5,980",
		);

		expect(await hasHorizontalOverflow(page)).toBeFalsy();
		expect(runtimeErrors).toEqual([]);
	});

	test("native plan, billing and add-on controls stay put, sync the recap, and only continue scrolls", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "desktop project only");
		const runtimeErrors = watchRuntimeErrors(page);
		await page.setViewportSize({ width: 1280, height: 720 });
		await openPricing(page);
		await finishPlanAnimations(page);

		const starterRadio = page.getByRole("radio", {
			name: "選擇 Free 免費",
		});

		const proRadio = page.getByRole("radio", { name: "選擇 Pro 專業" });
		const continueButton = page.getByRole("button", {
			name: "請團隊協助評估",
		});
		const planSelect = page.getByLabel("想了解的方案（選填）");
		const billingSelect = page.getByLabel("付款週期");
		const serviceSelect = page.getByLabel("加值服務（選填）");
		const recap = page.locator(".sp-selection-recap");

		await expect(continueButton).toBeDisabled();
		await expect(recap.locator("strong")).toHaveText([
			"尚未選擇",
			"月繳",
			"未加購",
		]);

		await starterRadio.focus();
		const planScrollY = await page.evaluate(() => window.scrollY);
		await page.keyboard.press("Space");
		await expect(starterRadio).toBeChecked();
		await expect(planSelect).toHaveValue("starter");
		await expectScrollPosition(page, planScrollY);

		await page.keyboard.press("ArrowRight");
		await expect(proRadio).toBeChecked();
		await expect(proRadio).toBeFocused();
		await expect(planSelect).toHaveValue("pro");
		await expectScrollPosition(page, planScrollY);
		await expect(continueButton).toBeEnabled();

		const monthlyRadio = page
			.locator(".sp-billing-cycle")
			.getByRole("radio", { name: "月繳" });
		const annualRadio = page
			.locator(".sp-billing-cycle")
			.getByRole("radio", { name: /年繳/u });
		await monthlyRadio.focus();
		const billingScrollY = await page.evaluate(() => window.scrollY);
		await page.keyboard.press("ArrowRight");
		await expect(annualRadio).toBeChecked();
		await expect(annualRadio).toBeFocused();
		await expect(billingSelect).toHaveValue("annual");
		await expectScrollPosition(page, billingScrollY);
		await expect(recap.locator("strong")).toHaveText([
			"Pro 專業",
			"年繳",
			"未加購",
		]);

		const managedToggle = page.locator(".sp-managed-service__toggle");
		await managedToggle.scrollIntoViewIfNeeded();
		const managedCheckbox = managedToggle.getByRole("checkbox");
		await managedCheckbox.focus();
		const addOnScrollY = await page.evaluate(() => window.scrollY);
		await settleManagedServiceReveal(page);
		const beforeAddOnGeometry = await readPricingGeometry(page);
		await page.keyboard.press("Space");
		await expect(managedCheckbox).toBeChecked();
		await expect(serviceSelect).toHaveValue("managed");
		await expectScrollPosition(page, addOnScrollY);
		const selectedAddOnGeometry = await readPricingGeometry(page);
		expectWithinOnePixel(
			selectedAddOnGeometry.addOnTop,
			beforeAddOnGeometry.addOnTop,
		);
		expectWithinOnePixel(
			selectedAddOnGeometry.addOnHeight,
			beforeAddOnGeometry.addOnHeight,
		);
		expectWithinOnePixel(
			selectedAddOnGeometry.nextSectionTop,
			beforeAddOnGeometry.nextSectionTop,
		);
		expectWithinOnePixel(
			selectedAddOnGeometry.documentHeight,
			beforeAddOnGeometry.documentHeight,
		);
		await expect(recap.locator("strong")).toHaveText([
			"Pro 專業",
			"年繳",
			"代行包 +6%",
		]);

		await continueButton.scrollIntoViewIfNeeded();
		const beforeContinueY = await page.evaluate(() => window.scrollY);
		await page.evaluate(() => {
			const original = Element.prototype.scrollIntoView;
			const state = window as Window & {
				__pricingScrollIntoView?: {
					behavior?: ScrollBehavior;
					block?: ScrollLogicalPosition;
				};
			};
			Element.prototype.scrollIntoView = function scrollIntoView(
				options?: boolean | ScrollIntoViewOptions,
			) {
				if (typeof options === "object") {
					state.__pricingScrollIntoView = {
						behavior: options.behavior,
						block: options.block,
					};
				}
				return original.call(this, options);
			};
		});
		await continueButton.click();
		const applicationTitle = page.getByRole("heading", {
			name: "需要協助時，再留下聯絡方式。",
		});
		await expect(applicationTitle).toBeFocused();
		await expect
			.poll(() => page.evaluate(() => window.scrollY))
			.toBeGreaterThan(beforeContinueY + 100);
		expect(
			await page.evaluate(
				() =>
					(
						window as Window & {
							__pricingScrollIntoView?: {
								behavior?: ScrollBehavior;
								block?: ScrollLogicalPosition;
							};
						}
					).__pricingScrollIntoView,
			),
		).toEqual({ behavior: "smooth", block: "start" });

		expect(runtimeErrors).toEqual([]);
	});

	test("groups all eight FAQ disclosures behind anchor navigation and scrollspy", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "desktop project only");
		const runtimeErrors = watchRuntimeErrors(page);
		await page.setViewportSize({ width: 1280, height: 720 });
		await openPricing(page);

		const faq = page.locator("#faq");
		await faq.scrollIntoViewIfNeeded();
		await expect(faq.getByRole("heading", { level: 2 })).toHaveText("常見問題");
		await expect(
			faq.getByRole("link", { name: "查看完整賣家說明書" }),
		).toHaveAttribute("href", "/seller-guide");

		const categoryNavigation = faq.getByRole("navigation", {
			name: "常見問題分類",
		});
		const navigationLinks = categoryNavigation.locator(".sp-faq__nav-link");
		const groups = faq.locator(".sp-faq-group");
		const items = faq.locator(".sp-faq-item");
		await expect(categoryNavigation.locator(".sp-faq__nav-meta")).toHaveText(
			"共 8 題",
		);
		await expect(navigationLinks).toHaveCount(FAQ_CATEGORIES.length);
		await expect(groups).toHaveCount(FAQ_CATEGORIES.length);
		await expect(items).toHaveCount(8);
		expect(
			await items.evaluateAll((details) =>
				details.every(
					(item) => item.getAttribute("name") === "seller-pricing-faq",
				),
			),
		).toBeTruthy();

		for (const category of FAQ_CATEGORIES) {
			const link = categoryNavigation.getByRole("link", {
				name: new RegExp(category.label, "u"),
			});
			const group = faq.locator(`#sp-faq-category-${category.id}`);
			await expect(link).toHaveAttribute(
				"href",
				`#sp-faq-category-${category.id}`,
			);
			await expect(link).toContainText(category.description);
			await expect(link.locator(".sp-faq__nav-count")).toHaveText(
				String(category.count),
			);
			await expect(group).toHaveAttribute("data-faq-category", category.id);
			await expect(group.getByRole("heading", { level: 3 })).toHaveText(
				category.label,
			);
			await expect(group.locator(".sp-faq-group__count")).toHaveText(
				`${category.count} 題`,
			);
			await expect(group.locator(".sp-faq-item")).toHaveCount(category.count);
		}

		for (const item of await items.all()) {
			await expect(item.locator("summary h4")).toHaveCount(1);
			await expect(item.locator(".sp-faq-item__toggle")).toHaveCount(1);
			await expect(item.locator(".sp-faq-item__toggle")).toHaveAttribute(
				"aria-hidden",
				"true",
			);
			await expect(item.locator(".sp-faq-item__answer")).toHaveCount(1);
		}

		const plansLink = categoryNavigation.getByRole("link", {
			name: /方案與費用/u,
		});
		const serviceLink = categoryNavigation.getByRole("link", {
			name: /代行與責任/u,
		});
		const changesLink = categoryNavigation.getByRole("link", {
			name: /退款與變更/u,
		});
		await plansLink.click();
		await expect(plansLink).toHaveAttribute("aria-current", "location");
		await serviceLink.hover();
		const activeAndHoverState = await categoryNavigation.evaluate(
			(navigation) => {
				const active = navigation.querySelector<HTMLElement>(
					'.sp-faq__nav-link[aria-current="location"]',
				);
				const hovered = navigation.querySelector<HTMLElement>(
					".sp-faq__nav-link:hover",
				);
				if (!active || !hovered) {
					throw new Error("FAQ active and hover states are incomplete");
				}
				return {
					activeBackground: getComputedStyle(active).backgroundColor,
					hoverBackground: getComputedStyle(hovered).backgroundColor,
				};
			},
		);
		expect(activeAndHoverState.activeBackground).not.toBe(
			activeAndHoverState.hoverBackground,
		);
		await page.mouse.move(0, 0);
		await finishFaqAnimations(page);
		const beforeCategoryChange = await readFaqNavigationGeometry(page);
		await serviceLink.click();
		await expect
			.poll(() => page.evaluate(() => window.location.hash))
			.toBe("#sp-faq-category-service");
		await expect(serviceLink).toHaveAttribute("aria-current", "location");
		await expect(plansLink).not.toHaveAttribute("aria-current", /.+/u);
		await expect
			.poll(() =>
				faq
					.locator("#sp-faq-category-service")
					.evaluate((group) => group.getBoundingClientRect().top),
			)
			.toBeGreaterThanOrEqual(110);
		await expect
			.poll(() =>
				faq
					.locator("#sp-faq-category-service")
					.evaluate((group) => group.getBoundingClientRect().top),
			)
			.toBeLessThanOrEqual(114);
		await finishFaqAnimations(page);
		expectFaqNavigationGeometryStable(
			await readFaqNavigationGeometry(page),
			beforeCategoryChange,
		);

		await faq.locator("#sp-faq-category-changes").evaluate((group) => {
			const top =
				group.getBoundingClientRect().top +
				window.scrollY -
				window.innerHeight * 0.2;
			window.scrollTo({ top, behavior: "auto" });
		});
		await expect(changesLink).toHaveAttribute("aria-current", "location");
		await expect(
			categoryNavigation.locator('[aria-current="location"]'),
		).toHaveCount(1);

		const firstItem = items.first();
		const firstToggle = firstItem.locator(".sp-faq-item__toggle");
		const closedVisual = await firstItem.evaluate((item) => {
			const style = getComputedStyle(item);
			const toggle = item.querySelector<HTMLElement>(".sp-faq-item__toggle");
			if (!toggle) throw new Error("FAQ toggle is missing");
			const toggleStyle = getComputedStyle(toggle);
			const toggleBounds = toggle.getBoundingClientRect();
			return {
				borderStyle: style.borderStyle,
				borderRadius: Number.parseFloat(style.borderRadius),
				toggleBorderStyle: toggleStyle.borderStyle,
				toggleWidth: toggleBounds.width,
				toggleHeight: toggleBounds.height,
			};
		});
		expect(closedVisual.borderStyle).toBe("solid");
		expect(closedVisual.borderRadius).toBeGreaterThan(0);
		expect(closedVisual.toggleBorderStyle).toBe("solid");
		expect(closedVisual.toggleWidth).toBeGreaterThanOrEqual(36);
		expectWithinOnePixel(closedVisual.toggleWidth, closedVisual.toggleHeight);
		await firstItem.locator("summary").click();
		await expect(firstItem).toHaveAttribute("open", "");
		await expect(firstToggle.locator("svg")).toHaveCSS(
			"transform",
			/matrix\(-1, 0, 0, -1/u,
		);

		expect(await hasHorizontalOverflow(page)).toBeFalsy();
		expect(runtimeErrors).toEqual([]);
	});

	test("progressively enhances reveal, spotlight and native FAQ disclosure", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "desktop project only");
		const runtimeErrors = watchRuntimeErrors(page);
		await page.setViewportSize({ width: 1280, height: 720 });
		await openPricing(page);

		const root = page.locator(".seller-pricing-page");
		await expect(root).toHaveAttribute("data-sp-effects", "enhanced");
		const applicationForm = page.locator(".sp-application__form-wrap");
		await expect(applicationForm).toHaveAttribute(
			"data-sp-reveal-state",
			"pending",
		);
		await applicationForm.scrollIntoViewIfNeeded();
		await expect(applicationForm).toHaveAttribute(
			"data-sp-reveal-state",
			"visible",
		);
		await expect
			.poll(() =>
				applicationForm.evaluate(
					(element) => getComputedStyle(element).opacity,
				),
			)
			.toBe("1");

		const proCard = page.locator('.sp-plan-card[data-plan="pro"]');
		const titleAccent = proCard.locator(".sp-plan-card__title-local");
		const idleTitleAccent = await titleAccent.evaluate((element) => {
			const style = getComputedStyle(element, "::after");
			return {
				opacity: Number(style.opacity),
				scaleX: new DOMMatrix(style.transform).a,
			};
		});
		await proCard.hover();
		await expect
			.poll(() =>
				titleAccent.evaluate((element) => {
					const style = getComputedStyle(element, "::after");
					return new DOMMatrix(style.transform).a;
				}),
			)
			.toBeGreaterThan(0.95);
		const activeTitleAccent = await titleAccent.evaluate((element) => {
			const style = getComputedStyle(element, "::after");
			const durations = style.transitionDuration.split(",").map((duration) => {
				const value = duration.trim();
				return value.endsWith("ms")
					? Number.parseFloat(value) / 1000
					: Number.parseFloat(value);
			});
			return {
				opacity: Number(style.opacity),
				scaleX: new DOMMatrix(style.transform).a,
				maxTransitionSeconds: Math.max(0, ...durations),
			};
		});
		expect(activeTitleAccent.opacity).toBeGreaterThan(idleTitleAccent.opacity);
		expect(activeTitleAccent.scaleX).toBeGreaterThan(idleTitleAccent.scaleX);
		expect(activeTitleAccent.maxTransitionSeconds).toBeGreaterThan(0);
		expect(activeTitleAccent.maxTransitionSeconds).toBeLessThanOrEqual(0.4);
		await expect
			.poll(() =>
				proCard.evaluate((element) =>
					element.style.getPropertyValue("--sp-spotlight-opacity"),
				),
			)
			.toBe("1");
		const spotlightLayer = await proCard.evaluate((element) => {
			const style = getComputedStyle(element, "::before");
			return { opacity: Number(style.opacity), zIndex: style.zIndex };
		});
		expect(spotlightLayer.opacity).toBeGreaterThan(0);
		expect(spotlightLayer.zIndex).toBe("0");

		const managedService = page.locator("#plans #managed-service");
		await managedService.scrollIntoViewIfNeeded();
		await expect(managedService).toHaveAttribute(
			"data-sp-reveal-state",
			"visible",
		);
		expect(await managedService.evaluate((element) => element.tagName)).toBe(
			"FIELDSET",
		);

		const firstFaq = page.locator(".sp-faq-item").first();
		const annualFaq = page
			.locator(".sp-faq-item")
			.filter({ hasText: "Pro 年繳怎麼計算？" });
		await firstFaq.locator("summary").focus();
		await page.keyboard.press("Enter");
		await expect(firstFaq).toHaveAttribute("open", "");
		await annualFaq.locator("summary").focus();
		await page.keyboard.press("Space");
		await expect(annualFaq).toHaveAttribute("open", "");
		await expect(firstFaq).not.toHaveAttribute("open", "");
		await expect(annualFaq.locator("summary")).toBeFocused();

		await page.emulateMedia({ reducedMotion: "reduce" });
		await expect(root).toHaveAttribute("data-sp-effects", "static");
		await expect
			.poll(() =>
				root.evaluate((element) => {
					const reveals = [
						...element.querySelectorAll<HTMLElement>("[data-sp-reveal]"),
					];
					return {
						allVisible: reveals.every((reveal) => {
							const style = getComputedStyle(reveal);
							return style.opacity === "1" && style.transform === "none";
						}),
						runningAnimations: element
							.getAnimations({ subtree: true })
							.filter((animation) => animation.playState === "running").length,
					};
				}),
			)
			.toEqual({ allVisible: true, runningAnimations: 0 });

		await page.emulateMedia({
			reducedMotion: "no-preference",
			forcedColors: "active",
		});
		await page.locator('label[for="seller-plan-pro"]').click();
		await expect(proCard).toHaveAttribute("data-selected", "true");
		const forcedColorState = await proCard.evaluate((element) => {
			const selected = getComputedStyle(element);
			const spotlight = getComputedStyle(element, "::before");
			const titleLocal = element.querySelector<HTMLElement>(
				".sp-plan-card__title-local",
			);
			return {
				outlineStyle: selected.outlineStyle,
				outlineWidth: selected.outlineWidth,
				spotlightDisplay: spotlight.display,
				titleAccentDisplay: titleLocal
					? getComputedStyle(titleLocal, "::after").display
					: "missing",
			};
		});
		expect(forcedColorState).toEqual({
			outlineStyle: "solid",
			outlineWidth: "3px",
			spotlightDisplay: "none",
			titleAccentDisplay: "none",
		});
		expect(runtimeErrors).toEqual([]);
	});

	test("plan, annual billing and managed service sync into the lead payload", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "desktop project only");
		const runtimeErrors = watchRuntimeErrors(page);
		let requestCount = 0;
		let submittedLead: Record<string, unknown> | null = null;
		await page.route("**/api/public/seller-leads", async (route) => {
			requestCount += 1;
			submittedLead = route.request().postDataJSON() as Record<string, unknown>;
			await route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ ok: true }),
			});
		});

		await openPricing(page);
		const planSelect = page.getByLabel("想了解的方案（選填）");

		await page.locator('label[for="seller-plan-starter"]').click();
		await expect(planSelect).toHaveValue("starter");
		await expect(
			page.locator('.sp-plan-card[data-plan="starter"]'),
		).toHaveAttribute("data-selected", "true");

		await page.locator('label[for="seller-plan-pro"]').click();
		await expect(planSelect).toHaveValue("pro");
		await expect(
			page.locator('.sp-plan-card[data-plan="pro"]'),
		).toHaveAttribute("data-selected", "true");

		await page.locator('label[for="sp-billing-annual"]').click();
		await expect(page.getByLabel("付款週期")).toHaveValue("annual");
		await expect(
			page
				.locator('.sp-plan-card[data-plan="pro"] .sp-plan-card__pricing dd')
				.first(),
		).toHaveText("NT$29,900／年");

		await page.locator(".sp-managed-service__toggle").click();
		await expect(page.getByLabel("加值服務（選填）")).toHaveValue("managed");
		await expect(
			page.locator("#plans #managed-service").getByRole("checkbox"),
		).toBeChecked();
		await expect(page.locator(".sp-managed-service__toggle")).toHaveAttribute(
			"data-selected",
			"true",
		);
		await expect(page.locator(".sp-selection-recap strong")).toHaveText([
			"Pro 專業",
			"年繳",
			"代行包 +6%",
		]);

		const submit = page.getByRole("button", { name: "送出聯絡需求" });
		await expect(submit).toBeDisabled();
		await page.getByLabel("品牌／農場名稱").fill("範例鮮果");
		await page.getByLabel("聯絡人").fill("測試聯絡人");
		await page.getByLabel("商品品類").selectOption("蔬果與農產");
		await expect(submit).toBeDisabled();
		await page.getByLabel("Email").fill("seller@example.com");
		await expect(submit).toBeEnabled();

		await submit.click();
		const success = page.locator('.sp-lead-success[role="status"]');
		await expect(success).toBeVisible();
		await expect(success).toBeFocused();
		await expect(success).toContainText("我們已收到你的聯絡需求。");
		await expect(
			success.getByRole("link", { name: "繼續建立賣場" }),
		).toHaveAttribute("href", "/seller/onboarding");

		expect(requestCount).toBe(1);
		expect(submittedLead).toEqual({
			brandName: "範例鮮果",
			contactName: "測試聯絡人",
			phone: "",
			email: "seller@example.com",
			productType: "蔬果與農產",
			plan: "pro",
			billingCycle: "annual",
			serviceInterest: "managed",
		});
		expect(runtimeErrors).toEqual([]);
	});

	test("390px mobile stacks both cards and never overflows horizontally", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "mobile", "mobile project only");
		const runtimeErrors = watchRuntimeErrors(page);
		await page.setViewportSize({ width: 390, height: 844 });

		await openPricing(page);
		expect(await page.evaluate(() => window.innerWidth)).toBe(390);
		await page.locator('label[for="sp-billing-annual"]').click();
		await expect(page.getByLabel("付款週期")).toHaveValue("annual");
		await finishPlanAnimations(page);

		const cards = page.locator("#plans .sp-plan-card");
		await expect(cards).toHaveCount(2);
		const cardBounds = await cards.evaluateAll((elements) =>
			elements.map((element) => {
				const bounds = element.getBoundingClientRect();
				return {
					x: Math.round(bounds.x),
					y: Math.round(bounds.y),
					width: Math.round(bounds.width),
					height: Math.round(bounds.height),
				};
			}),
		);
		expect(cardBounds[0]?.x).toBe(cardBounds[1]?.x);
		expect(cardBounds[0]?.width).toBe(cardBounds[1]?.width);
		expect(cardBounds[1]?.y ?? 0).toBeGreaterThan(
			(cardBounds[0]?.y ?? 0) + (cardBounds[0]?.height ?? 0),
		);

		for (const [index, expected] of PLAN_CARDS.entries()) {
			const card = cards.nth(index);
			await expect(card.locator(".sp-plan-card__pricing dd")).toHaveText([
				expected.annualFee,
				expected.rate,
			]);
			const titleFits = await card
				.locator(".sp-plan-card__title")
				.evaluate((title) => title.scrollWidth <= title.clientWidth + 1);
			expect(titleFits).toBeTruthy();
			const pricingGeometry = await card
				.locator(".sp-plan-card__pricing")
				.evaluate((pricing) => {
					const columns = [...pricing.children] as HTMLElement[];
					const values = [...pricing.querySelectorAll<HTMLElement>("dd")];
					const first = columns[0]?.getBoundingClientRect();
					const second = columns[1]?.getBoundingClientRect();
					const container = pricing.getBoundingClientRect();
					return {
						columnsDoNotOverlap: Boolean(
							first && second && first.right <= second.left + 1,
						),
						valuesFit: values.every(
							(value) => value.scrollWidth <= value.clientWidth + 1,
						),
						columnsStayInside: columns.every((column) => {
							const bounds = column.getBoundingClientRect();
							return (
								bounds.left >= container.left - 1 &&
								bounds.right <= container.right + 1
							);
						}),
					};
				});
			expect(pricingGeometry).toEqual({
				columnsDoNotOverlap: true,
				valuesFit: true,
				columnsStayInside: true,
			});
			const selectionBounds = await card
				.locator(".sp-plan-card__select-hint")
				.boundingBox();
			expect(selectionBounds?.height ?? 0).toBeGreaterThanOrEqual(44);
		}

		const proSelectHint = cards.last().locator(".sp-plan-card__select-hint");
		await proSelectHint.evaluate((element) =>
			element.scrollIntoView({ block: "center", inline: "nearest" }),
		);
		const proSelectBounds = await proSelectHint.boundingBox();
		if (!proSelectBounds) throw new Error("Pro selection control is missing");
		const planTouchScrollY = await page.evaluate(() => window.scrollY);
		await settleManagedServiceReveal(page);
		const beforePlanTouchGeometry = await readPricingGeometry(page);
		await page.touchscreen.tap(
			proSelectBounds.x + proSelectBounds.width / 2,
			proSelectBounds.y + proSelectBounds.height / 2,
		);
		await expect(page.getByLabel("想了解的方案（選填）")).toHaveValue("pro");
		await expect(
			page.getByRole("radio", { name: "選擇 Pro 專業" }),
		).toBeChecked();
		await expectScrollPosition(page, planTouchScrollY);
		const selectedPlanGeometry = await readPricingGeometry(page);
		expectWithinOnePixel(
			selectedPlanGeometry.gridHeight,
			beforePlanTouchGeometry.gridHeight,
		);
		expectWithinOnePixel(
			selectedPlanGeometry.addOnTop,
			beforePlanTouchGeometry.addOnTop,
		);
		expectWithinOnePixel(
			selectedPlanGeometry.addOnHeight,
			beforePlanTouchGeometry.addOnHeight,
		);
		expectWithinOnePixel(
			selectedPlanGeometry.nextSectionTop,
			beforePlanTouchGeometry.nextSectionTop,
		);
		expectWithinOnePixel(
			selectedPlanGeometry.documentHeight,
			beforePlanTouchGeometry.documentHeight,
		);
		for (const [index, beforeCard] of beforePlanTouchGeometry.cards.entries()) {
			const selectedCard = selectedPlanGeometry.cards[index];
			expect(selectedCard).toBeDefined();
			expectWithinOnePixel(selectedCard?.top ?? 0, beforeCard.top);
			expectWithinOnePixel(selectedCard?.width ?? 0, beforeCard.width);
			expectWithinOnePixel(selectedCard?.height ?? 0, beforeCard.height);
		}
		expect(
			await cards
				.last()
				.evaluate((card) => Number(getComputedStyle(card, "::before").opacity)),
		).toBe(0);

		const managedService = page.locator("#plans #managed-service");
		const managedToggle = managedService.locator(".sp-managed-service__toggle");
		await managedToggle.scrollIntoViewIfNeeded();
		const managedBounds = await managedService.boundingBox();
		const managedToggleBounds = await managedToggle.boundingBox();
		expect(managedBounds).not.toBeNull();
		expect(managedToggleBounds).not.toBeNull();
		expect(managedBounds?.x ?? -1).toBeGreaterThanOrEqual(0);
		expect(
			(managedBounds?.x ?? 0) + (managedBounds?.width ?? 0),
		).toBeLessThanOrEqual(390);
		expect(managedToggleBounds?.height ?? 0).toBeGreaterThanOrEqual(44);
		const managedScrollY = await page.evaluate(() => window.scrollY);
		await settleManagedServiceReveal(page);
		const beforeManagedGeometry = await readPricingGeometry(page);
		if (!managedToggleBounds)
			throw new Error("managed-service toggle is missing");
		await page.touchscreen.tap(
			managedToggleBounds.x + managedToggleBounds.width / 2,
			managedToggleBounds.y + managedToggleBounds.height / 2,
		);
		await expect(managedToggle.getByRole("checkbox")).toBeChecked();
		await expect(page.getByLabel("加值服務（選填）")).toHaveValue("managed");
		await expectScrollPosition(page, managedScrollY);
		const selectedManagedGeometry = await readPricingGeometry(page);
		expectWithinOnePixel(
			selectedManagedGeometry.addOnHeight,
			beforeManagedGeometry.addOnHeight,
		);
		expectWithinOnePixel(
			selectedManagedGeometry.nextSectionTop,
			beforeManagedGeometry.nextSectionTop,
		);
		expectWithinOnePixel(
			selectedManagedGeometry.documentHeight,
			beforeManagedGeometry.documentHeight,
		);

		const faq = page.locator("#faq");
		await faq.scrollIntoViewIfNeeded();
		const faqNavigation = faq.getByRole("navigation", {
			name: "常見問題分類",
		});
		const faqNavigationLinks = faqNavigation.locator(".sp-faq__nav-link");
		const faqSummaries = faq.locator(".sp-faq-item summary");
		await expect(faqNavigationLinks).toHaveCount(FAQ_CATEGORIES.length);
		await expect(faqSummaries).toHaveCount(8);
		const mobileTargets = await faq
			.locator(".sp-faq__nav-link, .sp-faq-item summary")
			.evaluateAll((targets) =>
				targets.map((target) => {
					const bounds = target.getBoundingClientRect();
					return { width: bounds.width, height: bounds.height };
				}),
			);
		expect(
			mobileTargets.every(
				(target) => target.width >= 44 && target.height >= 44,
			),
		).toBeTruthy();
		const mobileNavigationBounds = await faqNavigation.evaluate(
			(navigation) => {
				const container = navigation.getBoundingClientRect();
				const links = [
					...navigation.querySelectorAll<HTMLElement>(".sp-faq__nav-link"),
				].map((link) => {
					const bounds = link.getBoundingClientRect();
					return {
						left: bounds.left,
						right: bounds.right,
						width: bounds.width,
					};
				});
				return {
					left: container.left,
					right: container.right,
					links,
				};
			},
		);
		expect(
			mobileNavigationBounds.links.every(
				(link) =>
					link.left >= mobileNavigationBounds.left - 1 &&
					link.right <= mobileNavigationBounds.right + 1,
			),
		).toBeTruthy();
		for (const link of mobileNavigationBounds.links.slice(1)) {
			expectWithinOnePixel(
				link.width,
				mobileNavigationBounds.links[0]?.width ?? 0,
			);
		}
		await finishFaqAnimations(page);
		const beforeMobileCategoryChange = await readFaqNavigationGeometry(page);
		const serviceCategoryLink = faqNavigation.getByRole("link", {
			name: /代行與責任/u,
		});
		await serviceCategoryLink.click();
		await expect(serviceCategoryLink).toHaveAttribute(
			"aria-current",
			"location",
		);
		await expect
			.poll(() =>
				faq
					.locator("#sp-faq-category-service")
					.evaluate((group) => group.getBoundingClientRect().top),
			)
			.toBeGreaterThanOrEqual(78);
		await expect
			.poll(() =>
				faq
					.locator("#sp-faq-category-service")
					.evaluate((group) => group.getBoundingClientRect().top),
			)
			.toBeLessThanOrEqual(82);
		await finishFaqAnimations(page);
		expectFaqNavigationGeometryStable(
			await readFaqNavigationGeometry(page),
			beforeMobileCategoryChange,
		);
		await faqSummaries.first().click();
		await expect(faq.locator(".sp-faq-item").first()).toHaveAttribute(
			"open",
			"",
		);
		expect(await hasHorizontalOverflow(page)).toBeFalsy();
		expect(await page.locator(".seller-pricing-page").innerText()).not.toMatch(
			/Growth|NT\$990|成交服務費\s*3%/u,
		);
		expect(runtimeErrors).toEqual([]);
	});

	test("reduced motion keeps the hero visible without sustained motion", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "desktop project only");
		const runtimeErrors = watchRuntimeErrors(page);
		await page.emulateMedia({ reducedMotion: "reduce" });

		await openPricing(page);
		expect(
			await page.evaluate(
				() => window.matchMedia("(prefers-reduced-motion: reduce)").matches,
			),
		).toBeTruthy();

		const heroImage = page.locator(".sp-hero__media-mask img");
		await expect(heroImage).toBeVisible();
		await expect
			.poll(() =>
				heroImage.evaluate(
					(image: HTMLImageElement) => image.complete && image.naturalWidth > 0,
				),
			)
			.toBe(true);
		await expect(page.locator(".seller-pricing-page")).toHaveAttribute(
			"data-sp-effects",
			"static",
		);

		await page.locator('.sp-plan-card[data-plan="pro"]').hover();
		await page
			.locator(".sp-faq-item")
			.filter({ hasText: "Pro 年繳怎麼計算？" })
			.locator("summary")
			.click();
		await page.locator('label[for="sp-billing-annual"]').click();

		const motionState = await page
			.locator(".seller-pricing-page")
			.evaluate((root) => {
				const spotlight = root.querySelector<HTMLElement>(
					"[data-sp-spotlight]",
				);
				const durationInSeconds = (value: string) =>
					value.endsWith("ms")
						? Number.parseFloat(value) / 1000
						: Number.parseFloat(value);
				const transitionDurations = [
					...root.querySelectorAll<HTMLElement>(
						"[data-sp-reveal], [data-sp-spotlight], .sp-plan-card__title-latin, .sp-plan-card__title-local, .sp-plan-card__price-value--fixed, .sp-button, .sp-faq__nav-link, .sp-faq__nav-icon, .sp-faq-item__answer",
					),
				].flatMap((element) =>
					getComputedStyle(element)
						.transitionDuration.split(",")
						.map((duration) => durationInSeconds(duration.trim())),
				);
				const detailsContentTransitionDurations = [
					...root.querySelectorAll<HTMLElement>(".sp-faq-item"),
				].flatMap((element) =>
					getComputedStyle(element, "::details-content")
						.transitionDuration.split(",")
						.map((duration) => durationInSeconds(duration.trim())),
				);
				const titleAccentTransitionDurations = [
					...root.querySelectorAll<HTMLElement>(".sp-plan-card__title-local"),
				].flatMap((element) =>
					getComputedStyle(element, "::after")
						.transitionDuration.split(",")
						.map((duration) => durationInSeconds(duration.trim())),
				);
				const fixedPriceAnimationNames = [
					...root.querySelectorAll<HTMLElement>(
						".sp-plan-card__price-value--fixed",
					),
				].map((element) => getComputedStyle(element).animationName);
				return {
					maxTransitionSeconds: Math.max(0, ...transitionDurations),
					maxDetailsContentTransitionSeconds: Math.max(
						0,
						...detailsContentTransitionDurations,
					),
					maxTitleAccentTransitionSeconds: Math.max(
						0,
						...titleAccentTransitionDurations,
					),
					fixedPriceAnimationNames,
					revealStates: [
						...root.querySelectorAll<HTMLElement>("[data-sp-reveal]"),
					].map((element) => element.dataset.spRevealState),
					spotlightDisplay: spotlight
						? getComputedStyle(spotlight, "::before").display
						: "missing",
					runningAnimations: root
						.getAnimations({ subtree: true })
						.filter((animation) => animation.playState === "running").length,
				};
			});
		expect(motionState.maxTransitionSeconds).toBeLessThanOrEqual(0.001);
		expect(motionState.maxDetailsContentTransitionSeconds).toBeLessThanOrEqual(
			0.001,
		);
		expect(motionState.maxTitleAccentTransitionSeconds).toBeLessThanOrEqual(
			0.001,
		);
		expect(new Set(motionState.fixedPriceAnimationNames)).toEqual(
			new Set(["none"]),
		);
		expect(new Set(motionState.revealStates)).toEqual(new Set(["visible"]));
		expect(motionState.spotlightDisplay).toBe("none");
		expect(motionState.runningAnimations).toBe(0);
		expect(runtimeErrors).toEqual([]);
	});

	test("forced colors distinguishes the current FAQ category and open disclosure", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "desktop project only");
		const runtimeErrors = watchRuntimeErrors(page);
		await page.emulateMedia({ forcedColors: "active" });
		await page.setViewportSize({ width: 1280, height: 720 });
		await openPricing(page);

		const faq = page.locator("#faq");
		await faq.scrollIntoViewIfNeeded();
		const serviceLink = faq.getByRole("link", { name: /代行與責任/u });
		await serviceLink.click();
		await expect(serviceLink).toHaveAttribute("aria-current", "location");
		const firstItem = faq.locator(".sp-faq-item").first();
		await firstItem.locator("summary").click();
		await expect(firstItem).toHaveAttribute("open", "");

		const forcedColorState = await faq.evaluate((root) => {
			const activeLink = root.querySelector<HTMLElement>(
				'.sp-faq__nav-link[aria-current="location"]',
			);
			const openItem = root.querySelector<HTMLElement>(".sp-faq-item[open]");
			const openToggle = openItem?.querySelector<HTMLElement>(
				".sp-faq-item__toggle",
			);
			if (!activeLink || !openItem || !openToggle) {
				throw new Error("forced-colors FAQ state is incomplete");
			}
			const activeStyle = getComputedStyle(activeLink);
			const openStyle = getComputedStyle(openItem);
			const toggleStyle = getComputedStyle(openToggle);
			return {
				mediaMatches: window.matchMedia("(forced-colors: active)").matches,
				activeOutlineStyle: activeStyle.outlineStyle,
				activeOutlineWidth: Number.parseFloat(activeStyle.outlineWidth),
				openOutlineStyle: openStyle.outlineStyle,
				openOutlineWidth: Number.parseFloat(openStyle.outlineWidth),
				openBorderWidth: Number.parseFloat(openStyle.borderTopWidth),
				toggleBorderWidth: Number.parseFloat(toggleStyle.borderTopWidth),
				toggleBackground: toggleStyle.backgroundColor,
			};
		});
		expect(forcedColorState.mediaMatches).toBeTruthy();
		expect(forcedColorState.activeOutlineStyle).not.toBe("none");
		expect(forcedColorState.activeOutlineWidth).toBeGreaterThanOrEqual(2);
		expect(forcedColorState.openOutlineStyle).not.toBe("none");
		expect(forcedColorState.openOutlineWidth).toBeGreaterThanOrEqual(2);
		expect(forcedColorState.openBorderWidth).toBeGreaterThanOrEqual(2);
		expect(forcedColorState.toggleBorderWidth).toBeGreaterThanOrEqual(1);
		expect(forcedColorState.toggleBackground).not.toBe("rgba(0, 0, 0, 0)");
		expect(runtimeErrors).toEqual([]);
	});
});

test.describe("seller pricing without JavaScript", { tag: ["@ui"] }, () => {
	test.use({ javaScriptEnabled: false });

	test("keeps pricing, native choices and FAQ disclosures usable", async ({
		page,
	}, testInfo) => {
		test.skip(testInfo.project.name !== "chromium", "desktop project only");
		await page.setViewportSize({ width: 1280, height: 720 });
		await page.goto("/seller/pricing", { waitUntil: "domcontentloaded" });

		const root = page.locator(".seller-pricing-page");
		await expect(root).toHaveAttribute("data-hydrated", "false");
		await expect(root).not.toHaveAttribute("data-sp-effects", /.+/u);
		await expect(page.locator("#plans .sp-plan-card")).toHaveCount(2);
		await expect(page.getByRole("heading", { level: 1 })).toHaveText(
			"小農、鮮食與食品品牌的自助開店市集",
		);

		const fallbackState = await root.evaluate((element) => {
			const reveals = [
				...element.querySelectorAll<HTMLElement>("[data-sp-reveal]"),
			];
			return reveals.every((reveal) => {
				const style = getComputedStyle(reveal);
				return style.opacity === "1" && style.transform === "none";
			});
		});
		expect(fallbackState).toBeTruthy();

		const starterRadio = page.getByRole("radio", {
			name: "選擇 Free 免費",
		});
		await starterRadio.focus();
		await page.keyboard.press("Space");
		await expect(starterRadio).toBeChecked();

		const managedService = page.locator("#plans #managed-service");
		const managedCheckbox = managedService.getByRole("checkbox");
		await expect(managedService).toHaveAttribute("data-selected", "false");
		await managedCheckbox.focus();
		await page.keyboard.press("Space");
		await expect(managedCheckbox).toBeChecked();
		await expect
			.poll(() =>
				managedService
					.locator(".sp-managed-service__toggle-indicator")
					.evaluate((indicator) => getComputedStyle(indicator).color),
			)
			.not.toBe("rgba(0, 0, 0, 0)");

		const faq = page.locator("#faq");
		await expect(faq.locator(".sp-faq-group")).toHaveCount(
			FAQ_CATEGORIES.length,
		);
		await expect(faq.locator(".sp-faq__nav-link")).toHaveCount(
			FAQ_CATEGORIES.length,
		);
		await expect(
			faq.locator('.sp-faq__nav-link[aria-current="location"]'),
		).toHaveCount(0);
		const noJsServiceLink = faq.getByRole("link", { name: /代行與責任/u });
		await noJsServiceLink.click();
		await expect
			.poll(() => page.evaluate(() => window.location.hash))
			.toBe("#sp-faq-category-service");
		await expect(
			faq.locator('.sp-faq__nav-link[aria-current="location"]'),
		).toHaveCount(0);
		await expect
			.poll(() =>
				faq
					.locator("#sp-faq-category-service")
					.evaluate((group) => group.getBoundingClientRect().top),
			)
			.toBeGreaterThanOrEqual(110);
		await expect
			.poll(() =>
				faq
					.locator("#sp-faq-category-service")
					.evaluate((group) => group.getBoundingClientRect().top),
			)
			.toBeLessThanOrEqual(114);
		const faqItems = faq.locator(".sp-faq-item");
		await expect(faqItems).toHaveCount(8);
		expect(
			await faqItems.evaluateAll((items) =>
				items.every((item) => {
					const style = getComputedStyle(item);
					return (
						style.display !== "none" &&
						style.visibility !== "hidden" &&
						item.getBoundingClientRect().height > 0
					);
				}),
			),
		).toBeTruthy();
		const firstFaq = faqItems.first();
		await firstFaq.locator("summary").focus();
		await page.keyboard.press("Enter");
		await expect(firstFaq).toHaveAttribute("open", "");
		await expect(firstFaq.locator(".sp-faq-item__answer")).toBeVisible();
		const annualFaq = faq
			.locator(".sp-faq-item")
			.filter({ hasText: "Pro 年繳怎麼計算？" });
		await annualFaq.locator("summary").focus();
		await page.keyboard.press("Space");
		await expect(annualFaq).toHaveAttribute("open", "");
		await expect(firstFaq).not.toHaveAttribute("open", "");

		expect(await hasHorizontalOverflow(page)).toBeFalsy();
	});
});
