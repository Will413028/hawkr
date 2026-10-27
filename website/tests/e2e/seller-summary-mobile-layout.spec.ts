import { expect, type Locator, type Page, test } from "@playwright/test";
import {
	type MockStore,
	mockMyDashboard,
	mockMyKyc,
	mockMyStore,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const SELLER_ID = "90010001";
const CUSTOMER_ID = "9001";
const LOADER_KEY = "hawkr:initial-loader-seen";

const SELLER_STORE: MockStore = {
	id: SELLER_ID,
	storeName: "手機版型測試商店",
};

const CUSTOMER = {
	id: 1,
	buyerId: CUSTOMER_ID,
	nickname: "版型測試客戶",
	phone: "0912345678",
	orderCount: 3,
	totalSpent: 1280,
	customerType: "normal",
	tags: [],
};

const SUMMARY_SURFACES = [
	{
		name: "賣家總覽",
		path: "/seller-center/dashboard",
		selector: ".seller-stats.seller-summary-rail",
		count: 4,
		compactOnPhone: false,
	},
	{
		name: "商品管理",
		path: "/seller-center/products",
		selector: ".seller-inventory__stats.seller-summary-rail--compact-phone",
		count: 5,
		compactOnPhone: true,
	},
	{
		name: "庫存管理",
		path: "/seller-center/inventory",
		selector: ".seller-inventory__stats.seller-summary-rail--compact-phone",
		count: 4,
		compactOnPhone: true,
	},
	{
		name: "客戶管理",
		path: "/seller-center/customers",
		selector: ".seller-customers__stats.seller-summary-rail",
		count: 3,
		compactOnPhone: false,
	},
] as const;

async function prepareSellerFixtures(
	page: Page,
	baseURL: string,
): Promise<void> {
	// Lowest-priority safety net for secondary seller-shell queries. Exact
	// fixtures are registered afterwards and therefore win in Playwright.
	await page.route("**/api/v1/**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		}),
	);
	await setupGatewayMocks(page, {
		identity: {
			accountId: SELLER_ID,
			roles: ["seller"],
		},
		session: "identity",
	});

	await mockMyStore(page, SELLER_STORE);
	await mockMyKyc(page, { status: "APPROVED" });
	await mockMyDashboard(page, {
		revenue: { current: 12800, previous: 12000 },
		productViews: { current: 64, previous: 60 },
		newFollowers: { current: 2, previous: 1 },
		orderStatusCounts: {
			pendingShipmentCount: 1,
			inProgressCount: 2,
			closedCount: 3,
			aftersalesCount: 0,
		},
	});
	await page.route("**/api/v1/app/products?**", (route) => {
		const url = new URL(route.request().url());
		return route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: {
					records: [],
					total: 0,
					page: Number(url.searchParams.get("page") ?? 1),
					pageSize: Number(url.searchParams.get("page_size") ?? 20),
				},
			}),
		});
	});
	await page.route("**/api/v1/app/sellers/me/customers?**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: { list: [CUSTOMER], total: 1, page: 1 },
			}),
		}),
	);
	await page.route(
		`**/api/v1/app/sellers/me/customers/${CUSTOMER_ID}`,
		(route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					data: { ...CUSTOMER, couponCount: 2 },
				}),
			}),
	);
	await page.route(
		`**/api/v1/app/sellers/me/customers/${CUSTOMER_ID}/tags`,
		(route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: [] }),
			}),
	);
	await page.route("**/api/v1/app/sellers/me/tags", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: [] }),
		}),
	);
	await page.route("**/api/v1/app/orders?**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: { records: [], total: 0, page: 1, page_size: 100 },
			}),
		}),
	);

	await page
		.context()
		.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
	await seedStoredActiveRole(page, "seller");
	await page.addInitScript((loaderKey) => {
		sessionStorage.setItem(loaderKey, "1");
	}, LOADER_KEY);
}

interface RailGeometry {
	cardWidths: number[];
	cardHeights: number[];
	railLeft: number;
	railRight: number;
	railClientWidth: number;
	railScrollWidth: number;
	fullyVisibleCount: number;
	documentOverflow: number;
	firstLeft: number;
	firstRight: number;
	lastLeft: number;
	lastRight: number;
}

async function railGeometry(rail: Locator): Promise<RailGeometry> {
	return rail.evaluate((node) => {
		const host = node as HTMLElement;
		const railRect = host.getBoundingClientRect();
		const cards = Array.from(host.children) as HTMLElement[];
		const rects = cards.map((card) => card.getBoundingClientRect());
		return {
			cardWidths: rects.map((rect) => rect.width),
			cardHeights: rects.map((rect) => rect.height),
			railLeft: railRect.left,
			railRight: railRect.right,
			railClientWidth: host.clientWidth,
			railScrollWidth: host.scrollWidth,
			fullyVisibleCount: rects.filter(
				(rect) =>
					rect.left >= railRect.left - 1 && rect.right <= railRect.right + 1,
			).length,
			// Chromium on Linux reserves a physical scrollbar gutter, so an
			// overflow-free document can report scrollWidth - clientWidth = -15.
			// Only a positive range is user-scrollable horizontal overflow.
			documentOverflow: Math.max(
				0,
				document.documentElement.scrollWidth -
					document.documentElement.clientWidth,
			),
			firstLeft: rects[0]?.left ?? 0,
			firstRight: rects[0]?.right ?? 0,
			lastLeft: rects.at(-1)?.left ?? 0,
			lastRight: rects.at(-1)?.right ?? 0,
		};
	});
}

function expectEqualCards(geometry: RailGeometry, expectedWidth: number): void {
	expect(geometry.cardWidths.length).toBeGreaterThan(0);
	expect(
		Math.max(...geometry.cardWidths) - Math.min(...geometry.cardWidths),
	).toBeLessThanOrEqual(1);
	expect(
		Math.max(...geometry.cardHeights) - Math.min(...geometry.cardHeights),
	).toBeLessThanOrEqual(1);
	expect(geometry.cardWidths[0]).toBeGreaterThanOrEqual(expectedWidth - 1);
	expect(geometry.cardWidths[0]).toBeLessThanOrEqual(expectedWidth + 1);
	const contentWidth =
		geometry.cardWidths.length * expectedWidth +
		Math.max(0, geometry.cardWidths.length - 1) * 8;
	if (contentWidth > geometry.railClientWidth) {
		expect(geometry.railScrollWidth).toBeGreaterThan(geometry.railClientWidth);
		expect(geometry.fullyVisibleCount).toBeLessThan(geometry.cardWidths.length);
	} else {
		expect(geometry.railScrollWidth).toBeLessThanOrEqual(
			geometry.railClientWidth,
		);
		expect(geometry.fullyVisibleCount).toBe(geometry.cardWidths.length);
	}
	expect(geometry.documentOverflow).toBe(0);
	expect(geometry.firstLeft).toBeGreaterThanOrEqual(geometry.railLeft - 1);
	expect(geometry.firstRight).toBeLessThanOrEqual(geometry.railRight + 1);
}

function expectCompactPhoneCards(geometry: RailGeometry): void {
	expect(geometry.cardWidths.length).toBeGreaterThan(0);
	expect(
		Math.max(...geometry.cardWidths) - Math.min(...geometry.cardWidths),
	).toBeLessThanOrEqual(1);
	expect(
		Math.max(...geometry.cardHeights) - Math.min(...geometry.cardHeights),
	).toBeLessThanOrEqual(1);
	expect(Math.min(...geometry.cardWidths)).toBeGreaterThanOrEqual(63);
	expect(Math.max(...geometry.cardWidths)).toBeLessThanOrEqual(100);
	expect(Math.min(...geometry.cardHeights)).toBeGreaterThanOrEqual(67);
	expect(Math.max(...geometry.cardHeights)).toBeLessThanOrEqual(70);
	expect(geometry.documentOverflow).toBe(0);
	expect(geometry.firstLeft).toBeGreaterThanOrEqual(geometry.railLeft - 1);
	expect(geometry.firstRight).toBeLessThanOrEqual(geometry.railRight + 1);
}

async function expectLastCardReachable(rail: Locator): Promise<void> {
	await rail.evaluate((node) => {
		const host = node as HTMLElement;
		host.scrollLeft = host.scrollWidth;
	});
	await expect
		.poll(async () => {
			const geometry = await railGeometry(rail);
			return {
				leftInside: geometry.lastLeft >= geometry.railLeft - 1,
				rightInside: geometry.lastRight <= geometry.railRight + 1,
			};
		})
		.toEqual({ leftInside: true, rightInside: true });
}

async function expectCardTextFullyVisible(card: Locator): Promise<void> {
	const textBoxes = await card.locator("strong, small").evaluateAll((nodes) =>
		nodes.map((node) => {
			const element = node as HTMLElement;
			return {
				text: element.textContent ?? "",
				clientWidth: element.clientWidth,
				scrollWidth: element.scrollWidth,
				clientHeight: element.clientHeight,
				scrollHeight: element.scrollHeight,
			};
		}),
	);

	for (const box of textBoxes) {
		expect(
			box.scrollWidth,
			`${box.text} must not be clipped horizontally`,
		).toBeLessThanOrEqual(box.clientWidth + 1);
		// Chromium rounds the font's ink box separately from its CSS line box;
		// allow one rounded glyph pixel at each vertical edge. The production
		// rule uses overflow: visible, so this ink is painted rather than clipped.
		expect(
			box.scrollHeight,
			`${box.text} must not be clipped vertically`,
		).toBeLessThanOrEqual(box.clientHeight + 2);
	}
}

test.describe("賣家中心手機摘要卡滑軌", { tag: ["@ui"] }, () => {
	test.beforeEach(async ({ page, baseURL }, testInfo) => {
		test.skip(
			testInfo.project.name !== "chromium",
			"Explicit viewport matrix runs once in the Chromium project.",
		);
		test.skip(!baseURL, "Playwright baseURL is required");
		await prepareSellerFixtures(page, baseURL as string);
	});

	for (const viewport of [
		{ width: 320, height: 800, cardWidth: 186 },
		{ width: 360, height: 800, cardWidth: 186 },
		{ width: 393, height: 852, cardWidth: 186 },
		{ width: 430, height: 932, cardWidth: 186 },
		{ width: 768, height: 1024, cardWidth: 186 },
		{ width: 960, height: 900, cardWidth: 186 },
	]) {
		test(`${viewport.width}px：摘要卡使用對應手機／平板幾何且整頁不溢位`, async ({
			page,
		}) => {
			test.setTimeout(60_000);
			await page.setViewportSize(viewport);

			for (const surface of SUMMARY_SURFACES) {
				await page.goto(surface.path);
				const rail = page.locator(surface.selector);
				await expect(rail, surface.name).toBeVisible();
				await expect(rail.locator(":scope > *"), surface.name).toHaveCount(
					surface.count,
				);
				if (
					surface.path === "/seller-center/inventory" &&
					viewport.width <= 540
				) {
					const value = rail.locator(":scope > *").last().locator("strong");
					await value.evaluate((node) => {
						node.textContent = "NT$ 27.7k";
					});
					await expect(value).toHaveText("NT$ 27.7k");
					await expectCardTextFullyVisible(rail.locator(":scope > *").last());
				}
				const geometry = await railGeometry(rail);
				if (surface.compactOnPhone && viewport.width <= 540) {
					expectCompactPhoneCards(geometry);
				} else {
					expectEqualCards(geometry, viewport.cardWidth);
				}
				await expectLastCardReachable(rail);
			}
		});
	}

	test("393px 暗色模式維持相同滑卡幾何與頁面邊界", async ({ page }) => {
		test.setTimeout(60_000);
		await page.setViewportSize({ width: 393, height: 852 });
		await page.addInitScript(() => localStorage.setItem("hk:theme", "dark"));

		for (const surface of SUMMARY_SURFACES) {
			await page.goto(surface.path);
			await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
			const rail = page.locator(surface.selector);
			await expect(rail, surface.name).toBeVisible();
			const geometry = await railGeometry(rail);
			if (surface.compactOnPhone) {
				expectCompactPhoneCards(geometry);
			} else {
				expectEqualCards(geometry, 186);
			}
		}
	});

	test("320–430px 訂單搜尋會自動縮寬且匯出完整留在畫面內", async ({ page }) => {
		test.setTimeout(60_000);

		for (const width of [320, 360, 393, 430]) {
			await page.setViewportSize({ width, height: 852 });
			await page.goto("/seller-center/orders");

			const actions = page.locator(".seller-orders__header-actions");
			await expect(actions).toBeVisible();
			await expect(actions.locator(":scope > *")).toHaveCount(3);

			const geometry = await actions.evaluate((node) => {
				const host = node as HTMLElement;
				const hostRect = host.getBoundingClientRect();
				const controls = Array.from(host.children) as HTMLElement[];
				const rects = controls.map((control) =>
					control.getBoundingClientRect(),
				);
				const refreshIdleContent = controls[1]?.querySelector(
					":scope > span > span:first-child",
				) as HTMLElement | null;
				const refreshIdleRect = refreshIdleContent?.getBoundingClientRect();
				return {
					clientWidth: host.clientWidth,
					scrollWidth: host.scrollWidth,
					hostLeft: hostRect.left,
					hostRight: hostRect.right,
					firstLeft: rects[0]?.left ?? 0,
					lastRight: rects.at(-1)?.right ?? 0,
					widths: rects.map((rect) => rect.width),
					controlClientWidths: controls.map((control) => control.clientWidth),
					controlScrollWidths: controls.map((control) => control.scrollWidth),
					refreshIdleLeft: refreshIdleRect?.left ?? 0,
					refreshIdleRight: refreshIdleRect?.right ?? 0,
					refreshButtonLeft: rects[1]?.left ?? 0,
					refreshButtonRight: rects[1]?.right ?? 0,
					documentOverflow: Math.max(
						0,
						document.documentElement.scrollWidth -
							document.documentElement.clientWidth,
					),
				};
			});

			expect(geometry.scrollWidth).toBeLessThanOrEqual(
				geometry.clientWidth + 1,
			);
			expect(geometry.firstLeft).toBeGreaterThanOrEqual(geometry.hostLeft - 1);
			expect(geometry.lastRight).toBeLessThanOrEqual(geometry.hostRight + 1);
			expect(geometry.widths[0]).toBeGreaterThan(0);
			expect(geometry.widths[1]).toBeGreaterThanOrEqual(95);
			expect(geometry.widths[1]).toBeLessThanOrEqual(97);
			expect(geometry.widths[2]).toBeGreaterThanOrEqual(71);
			expect(geometry.widths[2]).toBeLessThanOrEqual(73);
			expect(geometry.refreshIdleLeft).toBeGreaterThanOrEqual(
				geometry.refreshButtonLeft - 1,
			);
			expect(geometry.refreshIdleRight).toBeLessThanOrEqual(
				geometry.refreshButtonRight + 1,
			);
			expect(geometry.controlScrollWidths[2]).toBeLessThanOrEqual(
				geometry.controlClientWidths[2] + 1,
			);
			expect(geometry.documentOverflow).toBe(0);

			await expect(
				actions.getByRole("button", { name: "重新整理" }),
			).toBeVisible();
			await expect(actions.getByRole("button", { name: "匯出" })).toBeVisible();
		}

		await page.setViewportSize({ width: 320, height: 800 });
		await page.goto("/seller-center/orders?tab=aftersale");
		await expect(page.locator(".seller-orders__header-actions")).toHaveCount(0);

		const afterSaleActions = page.locator(
			".seller-orders .seller-page-shell__actions",
		);
		await expect(afterSaleActions).toBeVisible();
		await expect(afterSaleActions.locator(":scope > *")).toHaveCount(4);
		const afterSaleGeometry = await afterSaleActions.evaluate((node) => {
			const host = node as HTMLElement;
			const search = host.children[1] as HTMLElement | undefined;
			return {
				clientWidth: host.clientWidth,
				scrollWidth: host.scrollWidth,
				searchWidth: search?.getBoundingClientRect().width ?? 0,
				documentOverflow: Math.max(
					0,
					document.documentElement.scrollWidth -
						document.documentElement.clientWidth,
				),
			};
		});
		expect(afterSaleGeometry.scrollWidth).toBeGreaterThan(
			afterSaleGeometry.clientWidth,
		);
		expect(afterSaleGeometry.searchWidth).toBeGreaterThanOrEqual(147);
		expect(afterSaleGeometry.documentOverflow).toBe(0);

		await afterSaleActions.evaluate((node) => {
			const host = node as HTMLElement;
			host.scrollLeft = host.scrollWidth;
		});
		await expect
			.poll(async () => {
				return afterSaleActions.evaluate((node) => {
					const host = node as HTMLElement;
					const hostRect = host.getBoundingClientRect();
					const exportButton = host.lastElementChild as HTMLElement | null;
					const exportRect = exportButton?.getBoundingClientRect();
					return {
						leftInside: (exportRect?.left ?? 0) >= hostRect.left - 1,
						rightInside: (exportRect?.right ?? 0) <= hostRect.right + 1,
					};
				});
			})
			.toEqual({ leftInside: true, rightInside: true });

		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/seller-center/orders");
		const desktopButtons = page.locator(
			".seller-orders__header-actions > .seller-orders__btn",
		);
		await expect(desktopButtons).toHaveCount(2);
		const desktopButtonWidths = await desktopButtons.evaluateAll((buttons) =>
			buttons.map(
				(button) => (button as HTMLElement).getBoundingClientRect().width,
			),
		);
		expect(desktopButtonWidths).toHaveLength(2);
		for (const width of desktopButtonWidths) {
			expect(width).toBeGreaterThanOrEqual(143);
		}
	});

	test("320–430px 快速操作維持單行且任務名稱不裁切", async ({ page }) => {
		test.setTimeout(60_000);

		for (const width of [320, 360, 393, 430]) {
			await page.setViewportSize({ width, height: 852 });
			await page.goto("/seller-center/dashboard");

			const rail = page.locator(".seller-quickactions");
			const actions = rail.locator(":scope > .seller-quickaction");
			await expect(rail).toBeVisible();
			await expect(actions).toHaveCount(4);
			const geometry = await rail.evaluate((node) => {
				const host = node as HTMLElement;
				const hostRect = host.getBoundingClientRect();
				const controls = Array.from(host.children) as HTMLElement[];
				return {
					clientWidth: host.clientWidth,
					scrollWidth: host.scrollWidth,
					hostLeft: hostRect.left,
					hostRight: hostRect.right,
					actions: controls.map((control) => {
						const controlRect = control.getBoundingClientRect();
						const textNode = Array.from(control.childNodes).find(
							(child) =>
								child.nodeType === Node.TEXT_NODE &&
								Boolean(child.textContent?.trim()),
						);
						const range = document.createRange();
						if (textNode) range.selectNodeContents(textNode);
						const textRects = textNode
							? Array.from(range.getClientRects()).filter(
									(rect) => rect.width > 0 && rect.height > 0,
								)
							: [];
						return {
							label: textNode?.textContent?.trim() ?? "",
							controlLeft: controlRect.left,
							controlRight: controlRect.right,
							controlHeight: controlRect.height,
							textLineCount: textRects.length,
							textLeft: textRects[0]?.left ?? 0,
							textRight: textRects.at(-1)?.right ?? 0,
						};
					}),
					documentOverflow: Math.max(
						0,
						document.documentElement.scrollWidth -
							document.documentElement.clientWidth,
					),
				};
			});

			expect(geometry.scrollWidth).toBeGreaterThan(geometry.clientWidth);
			expect(geometry.documentOverflow).toBe(0);
			for (const action of geometry.actions) {
				expect(action.label).not.toBe("");
				expect(action.textLineCount, action.label).toBe(1);
				expect(action.textLeft, action.label).toBeGreaterThanOrEqual(
					action.controlLeft - 1,
				);
				expect(action.textRight, action.label).toBeLessThanOrEqual(
					action.controlRight + 1,
				);
				expect(action.controlHeight, action.label).toBeGreaterThanOrEqual(44);
			}

			const onboardingTitleWidths = await page
				.locator(".seller-onboarding-step__title")
				.evaluateAll((titles) =>
					titles.map((title) => ({
						text: title.textContent?.trim() ?? "",
						clientWidth: (title as HTMLElement).clientWidth,
						scrollWidth: (title as HTMLElement).scrollWidth,
					})),
				);
			expect(onboardingTitleWidths.length).toBeGreaterThan(0);
			for (const title of onboardingTitleWidths) {
				expect(title.scrollWidth, title.text).toBeLessThanOrEqual(
					title.clientWidth + 1,
				);
			}

			await rail.evaluate((node) => {
				const host = node as HTMLElement;
				host.scrollLeft = host.scrollWidth;
			});
			await expect
				.poll(async () => {
					return rail.evaluate((node) => {
						const host = node as HTMLElement;
						const hostRect = host.getBoundingClientRect();
						const lastRect = host.lastElementChild?.getBoundingClientRect();
						return {
							leftInside: (lastRect?.left ?? 0) >= hostRect.left - 1,
							rightInside: (lastRect?.right ?? 0) <= hostRect.right + 1,
						};
					});
				})
				.toEqual({ leftInside: true, rightInside: true });
		}

		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto("/seller-center/dashboard");
		const desktopRail = page.locator(".seller-quickactions");
		await expect(desktopRail).toBeVisible();
		const desktopGeometry = await desktopRail.evaluate((node) => ({
			display: getComputedStyle(node).display,
			overflowX: getComputedStyle(node).overflowX,
			firstDirection: getComputedStyle(node.firstElementChild as HTMLElement)
				.flexDirection,
		}));
		expect(desktopGeometry).toEqual({
			display: "flex",
			overflowX: "visible",
			firstDirection: "row",
		});
	});

	test("320–480px 共用分頁維持單行且只在元件內捲動", async ({ page }) => {
		test.setTimeout(60_000);
		await page.route(
			"**/api/v1/app/seller/invoices/documents?**",
			async (route) => {
				const url = new URL(route.request().url());
				await route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							records: [],
							total: 200,
							page: Number(url.searchParams.get("page") ?? 5),
							pageSize: 20,
							shipmentStatusDegraded: false,
						},
					}),
				});
			},
		);

		for (const width of [320, 393, 480, 481, 768, 1440]) {
			await page.setViewportSize({ width, height: 900 });
			await page.goto("/seller-center/invoices/documents?page=5");

			const previous = page.getByRole("button", {
				name: "上一頁",
				exact: true,
			});
			await expect(previous).toBeVisible();
			const rail = previous.locator("..");
			const viewport = rail.locator("..");
			await expect(rail.locator(":scope > *")).toHaveCount(9);
			await expect(rail.getByRole("button")).toHaveCount(7);
			await expect(
				rail.getByRole("button", { name: "下一頁", exact: true }),
			).toBeVisible();

			const geometry = await viewport.evaluate((node) => {
				const host = node as HTMLElement;
				const inner = host.firstElementChild as HTMLElement;
				const hostRect = host.getBoundingClientRect();
				const controls = Array.from(inner.children) as HTMLElement[];
				const rects = controls.map((control) =>
					control.getBoundingClientRect(),
				);
				return {
					clientWidth: host.clientWidth,
					scrollWidth: host.scrollWidth,
					firstLeft: rects[0]?.left ?? 0,
					lastRight: rects.at(-1)?.right ?? 0,
					hostLeft: hostRect.left,
					hostRight: hostRect.right,
					lineCounts: controls.map((control) => {
						const range = document.createRange();
						range.selectNodeContents(control);
						return {
							label: control.textContent?.trim() ?? "",
							lines: Array.from(range.getClientRects()).filter(
								(rect) => rect.width > 0 && rect.height > 0,
							).length,
						};
					}),
					documentOverflow: Math.max(
						0,
						document.documentElement.scrollWidth -
							document.documentElement.clientWidth,
					),
				};
			});

			expect(geometry.firstLeft).toBeGreaterThanOrEqual(geometry.hostLeft - 1);
			expect(geometry.documentOverflow).toBe(0);
			for (const label of geometry.lineCounts) {
				expect(label.lines, label.label).toBe(1);
			}

			if (width <= 481) {
				expect(geometry.scrollWidth).toBeGreaterThan(geometry.clientWidth);
			} else {
				expect(geometry.scrollWidth).toBeLessThanOrEqual(
					geometry.clientWidth + 1,
				);
				expect(geometry.lastRight).toBeLessThanOrEqual(geometry.hostRight + 1);
			}

			await viewport.evaluate((node) => {
				const host = node as HTMLElement;
				host.scrollLeft = host.scrollWidth;
			});
			await expect
				.poll(async () => {
					return viewport.evaluate((node) => {
						const host = node as HTMLElement;
						const hostRect = host.getBoundingClientRect();
						const rail = host.firstElementChild as HTMLElement;
						const lastRect = rail.lastElementChild?.getBoundingClientRect();
						return {
							leftInside: (lastRect?.left ?? 0) >= hostRect.left - 1,
							rightInside: (lastRect?.right ?? 0) <= hostRect.right + 1,
						};
					});
				})
				.toEqual({ leftInside: true, rightInside: true });
		}
	});

	test("商品滑卡支援方向鍵、Home／End 與單一 Tab 停駐點", async ({ page }) => {
		await page.setViewportSize({ width: 393, height: 852 });
		await page.goto("/seller-center/products");
		const rail = page.locator(
			".seller-inventory__stats.seller-summary-rail--compact-phone[role='tablist']",
		);
		const tabs = rail.getByRole("tab");
		await expect(tabs).toHaveCount(5);
		await expect(rail.locator("[role='tab'][tabindex='0']")).toHaveCount(1);

		await tabs.first().focus();
		await page.keyboard.press("ArrowLeft");
		await expect(tabs.last()).toBeFocused();
		await expect(tabs.last()).toHaveAttribute("aria-selected", "true");
		await expect(tabs.last()).toHaveAttribute("tabindex", "0");
		await expect(tabs.first()).toHaveAttribute("tabindex", "-1");
		await expect
			.poll(async () => {
				const geometry = await railGeometry(rail);
				return (
					geometry.lastLeft >= geometry.railLeft - 1 &&
					geometry.lastRight <= geometry.railRight + 1
				);
			})
			.toBe(true);

		await page.keyboard.press("Home");
		await expect(tabs.first()).toBeFocused();
		await expect(tabs.first()).toHaveAttribute("aria-selected", "true");
		await page.keyboard.press("End");
		await expect(tabs.last()).toBeFocused();
		await expect(tabs.last()).toHaveAttribute("aria-selected", "true");
	});

	test("320px 客戶抽屜也套用滑軌；訂單狀態則維持四顆緊湊按鈕", async ({
		page,
	}) => {
		await page.setViewportSize({ width: 320, height: 800 });
		await page.goto("/seller-center/customers");
		// Exercise the actual phone interaction: reveal the single-line record,
		// then use its detail action to open the customer drawer.
		const customerList = page.getByRole("list", { name: "客戶清單" });
		await customerList
			.getByRole("button", { name: "展開客戶：版型測試客戶" })
			.click();
		await expect(
			customerList.getByRole("button", {
				name: "收合客戶：版型測試客戶",
			}),
		).toHaveAttribute("aria-expanded", "true");
		await customerList.getByRole("button", { name: "客戶資料" }).click();

		const drawerRail = page.locator(
			".seller-customer-detail__panel .seller-summary-rail",
		);
		await expect(drawerRail).toBeVisible();
		await expect(drawerRail.locator(":scope > *")).toHaveCount(3);
		const drawerGeometry = await railGeometry(drawerRail);
		expect(
			Math.max(...drawerGeometry.cardWidths) -
				Math.min(...drawerGeometry.cardWidths),
		).toBeLessThanOrEqual(1);
		expect(drawerGeometry.railScrollWidth).toBeGreaterThan(
			drawerGeometry.railClientWidth,
		);
		expect(drawerGeometry.documentOverflow).toBe(0);
		await expectLastCardReachable(drawerRail);

		await page.goto("/seller-center/orders");
		const statusTabs = page.locator(
			".seller-orders > .seller-orders__status-tabs",
		);
		await expect(statusTabs).toBeVisible();
		const statusGeometry = await statusTabs.evaluate((node) => {
			const host = node as HTMLElement;
			const rects = Array.from(host.children).map((child) =>
				(child as HTMLElement).getBoundingClientRect(),
			);
			return {
				widths: rects.map((rect) => rect.width),
				heights: rects.map((rect) => rect.height),
				documentOverflow: Math.max(
					0,
					document.documentElement.scrollWidth -
						document.documentElement.clientWidth,
				),
			};
		});
		expect(statusGeometry.widths).toHaveLength(4);
		expect(
			Math.max(...statusGeometry.widths) - Math.min(...statusGeometry.widths),
		).toBeLessThanOrEqual(1);
		expect(Math.max(...statusGeometry.widths)).toBeLessThanOrEqual(70);
		expect(Math.max(...statusGeometry.heights)).toBeLessThanOrEqual(70);
		expect(statusGeometry.documentOverflow).toBe(0);
	});
});
