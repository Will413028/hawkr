import { expect, type Page, type TestInfo, test } from "@playwright/test";
import { LOCAL_PRIVATE_KEYS } from "../../lib/auth/local-private-keys";
import { CART_UPDATED_EVENT } from "../../lib/cart";
import type { CartGroupWire, CartItemWire } from "../../lib/cart/types";
import { ShippingMethod, ShippingTemperature } from "../../lib/codemeta/enums";
import {
	mockBuyerAddresses,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const FARM = "鮮果測試農場";
const FISHERY = "南方海味測試舖";
const SKU = {
	pear: "selection-farm-pear",
	pineapple: "selection-farm-pineapple",
	unavailable: "selection-farm-unavailable",
	fish: "selection-fishery-fish",
	shrimp: "selection-fishery-shrimp",
};

function item(
	skuId: string,
	name: string,
	price: string,
	quantity: string,
	extra: Partial<CartItemWire> = {},
): CartItemWire {
	return {
		skuId,
		productId: `${skuId}-product`,
		name,
		price,
		quantity,
		imageKey: "/brand/logo-horizontal.svg",
		specName: "KG",
		stockQty: "20",
		purchasable: true,
		deliveryTemps: [ShippingTemperature.CHILLED],
		...extra,
	};
}

function deferred() {
	let release = () => {};
	const promise = new Promise<void>((resolve) => {
		release = resolve;
	});
	return { promise, release };
}

async function setupCart(page: Page, baseURL: string | undefined) {
	const appUrl = new URL(baseURL ?? "http://localhost:3000");
	await page.context().routeWebSocket("**/*", (socket) => {
		const url = new URL(socket.url());
		if (
			url.protocol === (appUrl.protocol === "https:" ? "wss:" : "ws:") &&
			url.host === appUrl.host &&
			url.pathname.startsWith("/_next/")
		) {
			socket.connectToServer();
		} else {
			void socket.close();
		}
	});
	// Register the catch-all first: no browser API request reaches a real cart.
	await page.route("**/api/v1/**", (route) =>
		route.fulfill({ json: { code: 0, data: {} } }),
	);
	await seedStoredActiveRole(page, "buyer");
	await setupGatewayMocks(page, {
		session: "identity",
		identity: { accountId: "901", roles: ["buyer"] },
	});
	await page.context().addCookies([
		{
			name: "hk_session",
			value: "opaque",
			url: appUrl.origin,
			httpOnly: true,
		},
	]);
	let groups: CartGroupWire[] = [
		{
			storeId: "901",
			sellerName: FARM,
			items: [
				item(SKU.pear, "珍珠芭樂", "120", "2"),
				item(SKU.pineapple, "金鑽鳳梨", "80", "1"),
				item(SKU.unavailable, "暫時缺貨的季節水果", "999", "1", {
					purchasable: false,
					unavailableReason: "OUT_OF_STOCK",
					stockQty: "0",
				}),
			],
		},
		{
			storeId: "902",
			sellerName: FISHERY,
			items: [
				item(SKU.fish, "海鮮職人精選午仔魚家庭分享組", "350", "1"),
				item(SKU.shrimp, "白蝦", "90", "2"),
			],
		},
	];
	const mutations: { method: string; skuId: string; quantity?: string }[] = [];
	const validations: string[][] = [];
	const forbiddenTransactions: string[] = [];
	let quantityGate: ReturnType<typeof deferred> | undefined;
	let removeGate: ReturnType<typeof deferred> | undefined;
	let refreshGate: ReturnType<typeof deferred> | undefined;
	let fisheryGate: ReturnType<typeof deferred> | undefined;
	let cartReads = 0;
	const READ_ONLY_CHECKOUT = ["/validate", "/shipping_fee"];
	await page.route("**/api/v1/app/checkout/**", (route) => {
		const path = new URL(route.request().url()).pathname;
		if (!READ_ONLY_CHECKOUT.some((suffix) => path.endsWith(suffix))) {
			forbiddenTransactions.push(path);
			return route.fulfill({ status: 503, json: { code: 503, data: {} } });
		}
		return route.fallback();
	});
	await page.route("**/api/v1/public/carts**", async (route) => {
		const request = route.request();
		const pathname = new URL(request.url()).pathname;
		const method = request.method();
		const skuId = decodeURIComponent(
			pathname.split("/items/")[1]?.split("/")[0] ?? "",
		);
		if (method === "GET") {
			cartReads += 1;
			await refreshGate?.promise;
		} else if (method === "PUT" && pathname.endsWith("/quantity")) {
			const { quantity } = request.postDataJSON() as { quantity: string };
			mutations.push({ method, skuId, quantity });
			await quantityGate?.promise;
			groups = groups.map((group) => ({
				...group,
				items: group.items.map((row) =>
					row.skuId === skuId ? { ...row, quantity } : row,
				),
			}));
		} else if (method === "DELETE") {
			mutations.push({ method, skuId });
			await removeGate?.promise;
			groups = groups
				.map((group) => ({
					...group,
					items: group.items.filter((row) => row.skuId !== skuId),
				}))
				.filter((group) => group.items.length > 0);
		}
		const rows = groups.flatMap((group) => group.items);
		return route.fulfill({
			json: {
				code: 0,
				data: {
					groups,
					totalItems: rows.reduce((sum, row) => sum + Number(row.quantity), 0),
					estimatedTotalAmount: String(
						rows.reduce(
							(sum, row) => sum + Number(row.price) * Number(row.quantity),
							0,
						),
					),
				},
			},
		});
	});
	await mockBuyerAddresses(page, [
		{
			id: "selection-address",
			receiverName: "測試收件人",
			receiverPhone: "0912345678",
			townCode: "63000050",
			addressDetail: "測試路 1 號",
			defaultAddress: true,
		},
	]);
	await page.route("**/api/v1/app/checkout/validate", async (route) => {
		const { items } = route.request().postDataJSON() as {
			items: { skuId: string }[];
		};
		const ids = items.map((row) => row.skuId).sort();
		validations.push(ids);
		const fishery = ids.some((id) => id.startsWith("selection-fishery-"));
		if (fishery) await fisheryGate?.promise;
		return route.fulfill({
			json: {
				code: 0,
				data: {
					valid: true,
					errors: [],
					deliveryTempIntersection: [ShippingTemperature.CHILLED],
					availableShippingOptions: [
						{
							shippingMethod: ShippingMethod.TAQBIN,
							deliveryTemp: ShippingTemperature.CHILLED,
							label: fishery ? "海味店冷藏宅配" : "農場冷藏宅配",
							fee: fishery ? "160" : "90",
							freeShippingEligible: false,
						},
					],
				},
			},
		});
	});
	await page.route("**/api/v1/app/checkout/shipping_fee", (route) => {
		const { items } = route.request().postDataJSON() as {
			items: { skuId: string }[];
		};
		return route.fulfill({
			json: {
				code: 0,
				data: {
					shippingFee: items.some((row) => row.skuId === SKU.fish)
						? "160"
						: "90",
					freeShippingEligible: false,
				},
			},
		});
	});
	return {
		mutations,
		validations,
		forbiddenTransactions,
		get cartReads() {
			return cartReads;
		},
		holdQuantity: () => (quantityGate = deferred()),
		holdRemoval: () => (removeGate = deferred()),
		holdRefresh: () => (refreshGate = deferred()),
		holdFishery: () => (fisheryGate = deferred()),
	};
}

function seller(page: Page, name: string) {
	return page.getByRole("region", { name: `${name}購物車`, exact: true });
}

function selectStore(page: Page, name: string) {
	return page.getByRole("checkbox", {
		name: `選取${name}的全部商品`,
		exact: true,
	});
}

const checkout = (page: Page) => page.locator(".cart-page__checkout");

async function assertCheckedRows(page: Page, names: string[]) {
	await expect
		.poll(() =>
			page
				.locator(".cart-tile__check input:checked")
				.evaluateAll((inputs) =>
					inputs.map((input) => input.getAttribute("aria-label")).sort(),
				),
		)
		.toEqual(names.map((name) => `選擇 ${name}`).sort());
}

async function capture(page: Page, testInfo: TestInfo, name: string) {
	const groups = page.locator(".cart-page__seller-group");
	const measurements = await groups.evaluateAll((elements) =>
		elements.map((group) => ({
			name: group.getAttribute("aria-label"),
			clientWidth: group.clientWidth,
			scrollWidth: group.scrollWidth,
			buttons: [...group.querySelectorAll("button")].map((button) => ({
				name: button.getAttribute("aria-label") ?? button.textContent,
				height: button.getBoundingClientRect().height,
			})),
		})),
	);
	for (const group of measurements) {
		expect
			.soft(group.scrollWidth, group.name ?? "seller width")
			.toBeLessThanOrEqual(group.clientWidth + 1);
		for (const button of group.buttons) {
			expect
				.soft(button.height, button.name ?? "seller button")
				.toBeGreaterThanOrEqual(44);
		}
	}
	await testInfo.attach(`${name}-bounds`, {
		body: JSON.stringify(measurements, null, 2),
		contentType: "application/json",
	});
	await page.evaluate(() => window.scrollTo(0, 0));
	const path = testInfo.outputPath(`${name}.png`);
	await page.screenshot({ path, fullPage: true });
	await testInfo.attach(name, { path, contentType: "image/png" });
	// Keep readable component evidence alongside the long page/rec rail image.
	for (let index = 0; index < (await groups.count()); index += 1) {
		const groupPath = testInfo.outputPath(`${name}-store-${index + 1}.png`);
		await groups.nth(index).screenshot({ path: groupPath, scale: "css" });
		await testInfo.attach(`${name}-store-${index + 1}`, {
			path: groupPath,
			contentType: "image/png",
		});
	}
	const summaryPath = testInfo.outputPath(`${name}-summary.png`);
	await page.locator("aside").screenshot({ path: summaryPath, scale: "css" });
	await testInfo.attach(`${name}-summary`, {
		path: summaryPath,
		contentType: "image/png",
	});
}

test("依店家選取商品，切店等待物流更新且結帳只帶可購買商品 @ui", async ({
	page,
	baseURL,
	isMobile,
}, testInfo) => {
	test.setTimeout(60_000);
	await page.setViewportSize(
		isMobile ? { width: 320, height: 844 } : { width: 1440, height: 1000 },
	);
	const fixture = await setupCart(page, baseURL);
	await page.goto("/cart", { waitUntil: "domcontentloaded" });
	await expect(page.locator("li.cart-tile")).toHaveCount(5);
	await expect(checkout(page)).toHaveAttribute("aria-disabled", "false");
	await assertCheckedRows(page, ["珍珠芭樂", "金鑽鳳梨"]);
	await expect(page.getByRole("checkbox", { name: /^全選/ })).toHaveCount(0);
	await expect(
		page.getByRole("button", { name: "重新整理", exact: true }),
	).toHaveCount(0);
	await page
		.getByRole("checkbox", { name: "選擇 金鑽鳳梨", exact: true })
		.uncheck();
	await expect(selectStore(page, FARM)).toHaveJSProperty("indeterminate", true);
	await selectStore(page, FARM).check();
	await assertCheckedRows(page, ["珍珠芭樂", "金鑽鳳梨", "暫時缺貨的季節水果"]);
	await expect(page.getByRole("button", { name: /^移除.*已選/ })).toHaveCount(
		0,
	);
	await expect(page.locator(".cart-page__checkout-seller")).toContainText(FARM);
	await expect(
		page.locator("aside").getByText("NT$ 320", { exact: true }),
	).toBeVisible();
	await capture(page, testInfo, "store-selection-farm");

	const shipping = fixture.holdFishery();
	try {
		await seller(page, FISHERY)
			.getByRole("button", { name: "改選本店商品", exact: true })
			.click();
		await assertCheckedRows(page, ["海鮮職人精選午仔魚家庭分享組", "白蝦"]);
		await expect(page.locator(".cart-page__checkout-seller")).toContainText(
			FISHERY,
		);
		await expect
			.poll(() => fixture.validations.some((ids) => ids.includes(SKU.fish)))
			.toBe(true);
		await expect(checkout(page)).toHaveAttribute("aria-disabled", "true");
		await expect(
			page.locator("aside").getByText("NT$ 530", { exact: true }),
		).toBeVisible();
	} finally {
		shipping.release();
	}
	await expect(checkout(page)).toHaveAttribute("aria-disabled", "false");
	await expect(
		page
			.locator("aside")
			.getByRole("definition")
			.filter({ hasText: /^NT\$ 160$/ }),
	).toBeVisible();
	await capture(page, testInfo, "store-selection-fishery");
	await seller(page, FARM)
		.getByRole("button", { name: "改選本店商品", exact: true })
		.click();
	await assertCheckedRows(page, ["珍珠芭樂", "金鑽鳳梨", "暫時缺貨的季節水果"]);
	await expect(checkout(page)).toHaveAttribute("aria-disabled", "false");
	// Exercise the real click/draft writer, but do not mount the transaction page.
	await page.route(
		(url) => url.pathname === "/checkout",
		(route) =>
			route.fulfill({
				contentType: "text/html; charset=utf-8",
				body: "<!doctype html><title>測試結帳目的地</title><main>隔離的結帳目的地</main>",
			}),
	);
	await checkout(page).click();
	await expect(
		page.getByText("隔離的結帳目的地", { exact: true }),
	).toBeVisible();
	await expect
		.poll(() =>
			page.evaluate((key) => {
				const value = localStorage.getItem(key);
				return value ? JSON.parse(value) : null;
			}, LOCAL_PRIVATE_KEYS.checkoutDraft.key),
		)
		.toMatchObject({
			seller: { key: "901", name: FARM },
			selectedSkuIds: [SKU.pear, SKU.pineapple],
			shipping: { fee: 90 },
		});
	expect(
		fixture.validations.every((ids) => !ids.includes(SKU.unavailable)),
	).toBe(true);
	expect(
		fixture.validations.every(
			(ids) =>
				!ids.some((id) => id.startsWith("selection-farm-")) ||
				!ids.some((id) => id.startsWith("selection-fishery-")),
		),
	).toBe(true);
	expect(fixture.mutations).toEqual([]);
	expect(fixture.forbiddenTransactions).toEqual([]);
});

test("保留取消選取，更新中鎖住選店，移除最後已選商品不改選另一店 @ui", async ({
	page,
	baseURL,
	isMobile,
}, testInfo) => {
	test.setTimeout(60_000);
	await page.setViewportSize(
		isMobile ? { width: 320, height: 844 } : { width: 1440, height: 1000 },
	);
	const fixture = await setupCart(page, baseURL);
	await page.goto("/cart", { waitUntil: "domcontentloaded" });
	await expect(checkout(page)).toHaveAttribute("aria-disabled", "false");
	await selectStore(page, FARM).check();
	await selectStore(page, FARM).uncheck();
	await assertCheckedRows(page, []);
	await page.reload({ waitUntil: "domcontentloaded" });
	await expect(page.locator("li.cart-tile")).toHaveCount(5);
	await assertCheckedRows(page, []);
	await expect(checkout(page)).toHaveAttribute("aria-disabled", "true");
	await page
		.getByRole("checkbox", { name: "選擇 珍珠芭樂", exact: true })
		.check();
	await expect(checkout(page)).toHaveAttribute("aria-disabled", "false");
	const quantity = fixture.holdQuantity();
	try {
		await page
			.getByRole("button", { name: "增加 珍珠芭樂 數量", exact: true })
			.click();
		await expect
			.poll(
				() =>
					fixture.mutations.filter((entry) => entry.method === "PUT").length,
			)
			.toBe(1);
		await expect(selectStore(page, FARM)).toBeDisabled();
		await expect(
			seller(page, FISHERY).getByRole("button", {
				name: "改選本店商品",
				exact: true,
			}),
		).toBeDisabled();
		await expect(checkout(page)).toHaveAttribute("aria-disabled", "true");
	} finally {
		quantity.release();
	}
	await expect(
		page.getByRole("spinbutton", { name: "珍珠芭樂 數量", exact: true }),
	).toHaveValue("3");
	await expect(checkout(page)).toHaveAttribute("aria-disabled", "false");
	const refresh = fixture.holdRefresh();
	const beforeRefresh = fixture.cartReads;
	try {
		await page.evaluate(
			(event) => window.dispatchEvent(new CustomEvent(event)),
			CART_UPDATED_EVENT,
		);
		await expect.poll(() => fixture.cartReads).toBeGreaterThan(beforeRefresh);
		await expect(selectStore(page, FARM)).toBeDisabled();
		await expect(checkout(page)).toHaveAttribute("aria-disabled", "true");
	} finally {
		refresh.release();
	}
	await expect(selectStore(page, FARM)).toBeEnabled();
	await assertCheckedRows(page, ["珍珠芭樂"]);
	await expect(page.getByRole("button", { name: /^移除.*已選/ })).toHaveCount(
		0,
	);
	const remove = seller(page, FARM)
		.locator("li.cart-tile")
		.filter({
			has: page.getByRole("checkbox", { name: "選擇 珍珠芭樂", exact: true }),
		})
		.getByRole("button", { name: "移除", exact: true });
	page.once("dialog", (dialog) => dialog.dismiss());
	await remove.click();
	expect(
		fixture.mutations.filter((entry) => entry.method === "DELETE"),
	).toEqual([]);
	await expect(page.locator("li.cart-tile")).toHaveCount(5);
	const removal = fixture.holdRemoval();
	try {
		page.once("dialog", (dialog) => dialog.accept());
		await remove.click();
		await expect
			.poll(
				() =>
					fixture.mutations.filter((entry) => entry.method === "DELETE").length,
			)
			.toBe(1);
		await expect(selectStore(page, FARM)).toBeDisabled();
		await expect(
			seller(page, FISHERY).getByRole("button", {
				name: "改選本店商品",
				exact: true,
			}),
		).toBeDisabled();
		await expect(checkout(page)).toHaveAttribute("aria-disabled", "true");
	} finally {
		removal.release();
	}
	await expect(seller(page, FARM).locator("li.cart-tile")).toHaveCount(2);
	await expect(seller(page, FISHERY).locator("li.cart-tile")).toHaveCount(2);
	await expect(
		page.getByRole("checkbox", { name: "選擇 珍珠芭樂", exact: true }),
	).toHaveCount(0);
	await expect(page.locator("li.cart-tile")).toHaveCount(4);
	await assertCheckedRows(page, []);
	await page.reload({ waitUntil: "domcontentloaded" });
	await expect(page.locator("li.cart-tile")).toHaveCount(4);
	await assertCheckedRows(page, []);
	await expect(selectStore(page, FISHERY)).toBeEnabled();
	await expect(checkout(page)).toHaveAttribute("aria-disabled", "true");
	await capture(page, testInfo, "store-selection-none-after-removal");
	expect(fixture.mutations).toEqual([
		{ method: "PUT", skuId: SKU.pear, quantity: "3" },
		{ method: "DELETE", skuId: SKU.pear },
	]);
	expect(fixture.forbiddenTransactions).toEqual([]);
});
