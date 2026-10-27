import { expect, test } from "@playwright/test";
import type { CartItemWire } from "../../lib/cart/types";
import {
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

test("購物車缺圖與載入失敗縮圖不裁切，正常圖片及數量／移除操作維持可用 @ui", async ({
	page,
	baseURL,
	isMobile,
}, testInfo) => {
	test.setTimeout(60_000);
	await page.setViewportSize(
		isMobile ? { width: 390, height: 844 } : { width: 1440, height: 1000 },
	);
	// Next dev needs its same-origin HMR handshake before hydrating. Closing
	// localhost's ws connection prevented every session/cart request from starting.
	// Only this development connection is allowed; application sockets stay mocked.
	await page.context().routeWebSocket("**/*", (socket) => {
		const url = new URL(socket.url());
		const appUrl = new URL(baseURL ?? "http://localhost:3000");
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
	// Catch every gateway route before specific identity/cart fixtures so no
	// browser request, including the mutations below, reaches a real cart.
	await page.route("**/api/v1/**", (route) =>
		route.fulfill({ json: { code: 0, data: {} } }),
	);
	await seedStoredActiveRole(page, "buyer");
	await setupGatewayMocks(page, {
		session: "identity",
		identity: { accountId: "901", roles: ["buyer"] },
	});
	await page
		.context()
		.addCookies([
			{ name: "hk_session", value: "opaque", url: baseURL, httpOnly: true },
		]);

	const imageCases = [
		{ skuId: "cart-image-missing", name: "珍珠芭樂（未提供圖片）" },
		{ skuId: "cart-image-empty", name: "紅心芭樂（空白圖片）", imageKey: "" },
		{
			skuId: "cart-image-broken",
			name: "金鑽鳳梨（圖片載入失敗）",
			imageKey: "/images/cart-image-fallback-e2e-broken.png",
		},
		{
			skuId: "cart-image-valid",
			name: "當季鮮果（正常圖片）",
			imageKey: "/images/cart-image-fallback-e2e-valid.png",
		},
	];
	let items: CartItemWire[] = imageCases.map((item) => ({
		...item,
		price: "120",
		quantity: "2",
		specName: "KG",
		stockQty: "20",
		purchasable: true,
		deliveryTemps: ["CHILLED"],
	}));
	const mutations: { method: string; skuId: string; quantity?: string }[] = [];
	await page.route("**/images/cart-image-fallback-e2e-broken.png", (route) =>
		route.fulfill({ status: 404, body: "fixture image unavailable" }),
	);
	await page.route("**/images/cart-image-fallback-e2e-valid.png", (route) =>
		route.fulfill({
			status: 200,
			contentType: "image/png",
			body: Buffer.from(
				"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg==",
				"base64",
			),
		}),
	);
	await page.route("**/api/v1/public/carts**", (route) => {
		const request = route.request();
		const pathname = new URL(request.url()).pathname;
		const method = request.method();
		const skuId = decodeURIComponent(
			pathname.split("/items/")[1]?.split("/")[0] ?? "",
		);
		if (method === "PUT" && pathname.endsWith("/quantity")) {
			const { quantity } = request.postDataJSON() as { quantity: string };
			mutations.push({ method, skuId, quantity });
			items = items.map((item) =>
				item.skuId === skuId ? { ...item, quantity } : item,
			);
		} else if (method === "DELETE") {
			mutations.push({ method, skuId });
			items = items.filter((item) => item.skuId !== skuId);
		}
		return route.fulfill({
			json: {
				code: 0,
				data: {
					groups: [{ storeId: "901", sellerName: "鮮果測試農場", items }],
					totalItems: items.reduce(
						(sum, item) => sum + Number(item.quantity),
						0,
					),
					estimatedTotalAmount: String(
						items.reduce(
							(sum, item) => sum + Number(item.quantity) * Number(item.price),
							0,
						),
					),
				},
			},
		});
	});

	await page.goto("/cart");
	await expect(page.locator(".initial-loader")).toBeHidden();
	const rows = page.locator("li.cart-tile");
	await expect(rows).toHaveCount(4);
	for (const item of imageCases) {
		const row = rows.filter({ hasText: item.name });
		await expect(row.locator(".cart-tile__name")).toHaveText(item.name);
		await expect(
			row.getByRole("spinbutton", { name: `${item.name} 數量` }),
		).toHaveValue("2");
	}
	await expect(
		rows.nth(0).locator('.cart-tile__thumb > [role="img"]'),
	).toHaveAttribute("aria-label", "尚無圖片");
	await expect(
		rows.nth(1).locator('.cart-tile__thumb > [role="img"]'),
	).toHaveAttribute("aria-label", "尚無圖片");
	await expect(
		rows.nth(2).locator('.cart-tile__thumb > [role="img"]'),
	).toHaveAttribute("aria-label", "圖片無法顯示");
	const goodImage = rows.nth(3).locator(".cart-tile__thumb img");
	await expect(goodImage).toBeVisible();
	await expect
		.poll(() =>
			goodImage.evaluate((image) => (image as HTMLImageElement).naturalWidth),
		)
		.toBeGreaterThan(0);
	const measurements = await rows.evaluateAll((elements) =>
		elements.map((row) => {
			const thumb = row.querySelector<HTMLElement>(".cart-tile__thumb");
			if (!thumb) throw new Error("Cart item thumbnail is missing");
			const fallback = thumb.querySelector<HTMLElement>(
				':scope > [role="img"]',
			);
			const frame = thumb.getBoundingClientRect();
			return {
				name: row.querySelector(".cart-tile__name")?.textContent,
				frame: { width: frame.width, height: frame.height },
				children: fallback
					? [...fallback.children].map((child, index) => {
							const rect = child.getBoundingClientRect();
							return {
								kind: index === 0 ? "icon" : "caption",
								x: rect.x - frame.x,
								y: rect.y - frame.y,
								width: rect.width,
								height: rect.height,
								right: rect.right - frame.x,
								bottom: rect.bottom - frame.y,
							};
						})
					: [],
			};
		}),
	);
	console.log(
		`CART_IMAGE_BOUNDS ${testInfo.project.name} ${JSON.stringify(measurements)}`,
	);
	await testInfo.attach("thumbnail-bounds", {
		body: JSON.stringify(measurements, null, 2),
		contentType: "application/json",
	});
	const screenshot = testInfo.outputPath("cart-image-fallback.png");
	await page.screenshot({ path: screenshot, fullPage: true });
	await testInfo.attach("cart-image-fallback", {
		path: screenshot,
		contentType: "image/png",
	});
	const viewportScreenshot = testInfo.outputPath("cart-controls-viewport.png");
	await page.screenshot({ path: viewportScreenshot });
	await testInfo.attach("cart-controls-viewport", {
		path: viewportScreenshot,
		contentType: "image/png",
	});
	for (const measurement of measurements) {
		expect
			.soft(measurement.frame.width, measurement.name ?? "thumbnail width")
			.toBeGreaterThanOrEqual(80);
		expect
			.soft(measurement.frame.height, measurement.name ?? "thumbnail height")
			.toBeGreaterThanOrEqual(60);
		for (const child of measurement.children) {
			const context = `${measurement.name} ${child.kind}`;
			expect.soft(child.width, context).toBeGreaterThan(0);
			expect.soft(child.height, context).toBeGreaterThan(0);
			expect.soft(child.x, context).toBeGreaterThanOrEqual(-0.5);
			expect.soft(child.y, context).toBeGreaterThanOrEqual(-0.5);
			expect
				.soft(child.right, context)
				.toBeLessThanOrEqual(measurement.frame.width + 0.5);
			expect
				.soft(child.bottom, context)
				.toBeLessThanOrEqual(measurement.frame.height + 0.5);
		}
	}
	const controls = await rows.evaluateAll((elements) =>
		elements.map((row) => {
			const bounds = row.getBoundingClientRect();
			const quantity = row.querySelector<HTMLInputElement>(
				'input[type="number"]',
			);
			if (!quantity) throw new Error("Cart quantity input is missing");
			return {
				name: row.querySelector(".cart-tile__name")?.textContent,
				clientWidth: row.clientWidth,
				scrollWidth: row.scrollWidth,
				quantityFontSize: Number.parseFloat(
					getComputedStyle(quantity).fontSize,
				),
				buttons: [...row.querySelectorAll<HTMLButtonElement>("button")].map(
					(button) => {
						const rect = button.getBoundingClientRect();
						return {
							name:
								button.getAttribute("aria-label") ?? button.textContent?.trim(),
							width: rect.width,
							height: rect.height,
							left: rect.left - bounds.left,
							right: rect.right - bounds.left,
						};
					},
				),
			};
		}),
	);
	await testInfo.attach("cart-control-bounds", {
		body: JSON.stringify(controls, null, 2),
		contentType: "application/json",
	});
	console.log(
		`CART_CONTROL_BOUNDS ${testInfo.project.name} ${JSON.stringify(controls)}`,
	);
	for (const row of controls) {
		expect
			.soft(row.scrollWidth, `${row.name} stays within its row`)
			.toBeLessThanOrEqual(row.clientWidth + 1);
		expect
			.soft(row.quantityFontSize, `${row.name} quantity font`)
			.toBeGreaterThanOrEqual(16);
		for (const button of row.buttons) {
			const context = `${row.name}: ${button.name}`;
			expect.soft(button.width, `${context} width`).toBeGreaterThanOrEqual(44);
			expect
				.soft(button.height, `${context} height`)
				.toBeGreaterThanOrEqual(44);
			expect
				.soft(button.left, `${context} left edge`)
				.toBeGreaterThanOrEqual(-0.5);
			expect
				.soft(button.right, `${context} right edge`)
				.toBeLessThanOrEqual(row.clientWidth + 0.5);
		}
	}

	const missingRow = rows.filter({ hasText: imageCases[0].name });
	await missingRow
		.getByRole("button", {
			name: `增加 ${imageCases[0].name} 數量`,
			exact: true,
		})
		.click();
	await expect(missingRow.getByRole("spinbutton")).toHaveValue("3");
	expect(mutations).toContainEqual({
		method: "PUT",
		skuId: imageCases[0].skuId,
		quantity: "3",
	});
	page.once("dialog", (dialog) => dialog.accept());
	await missingRow.getByRole("button", { name: "移除", exact: true }).click();
	await expect(missingRow).toHaveCount(0);
	await expect(rows).toHaveCount(3);
	expect(mutations).toContainEqual({
		method: "DELETE",
		skuId: imageCases[0].skuId,
	});
});
