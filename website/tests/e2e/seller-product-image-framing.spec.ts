import { expect, type Locator, type Page, test } from "@playwright/test";
import {
	mockMyKyc,
	mockMyStore,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const PORTRAIT_PRODUCT_SVG = `
<svg xmlns="http://www.w3.org/2000/svg" width="600" height="1000" viewBox="0 0 600 1000">
  <rect width="600" height="1000" fill="#fffdf7"/>
  <rect x="18" y="18" width="564" height="964" rx="20" fill="#e1dbce" stroke="#334c0f" stroke-width="18"/>
  <rect x="90" y="100" width="420" height="800" rx="36" fill="#ffffff" stroke="#cab15c" stroke-width="12"/>
  <text x="300" y="180" text-anchor="middle" font-family="sans-serif" font-size="44" fill="#334c0f">TOP EDGE</text>
  <circle cx="300" cy="500" r="150" fill="#719244"/>
  <text x="300" y="850" text-anchor="middle" font-family="sans-serif" font-size="44" fill="#334c0f">BOTTOM EDGE</text>
</svg>`;

const PRODUCT_CROP_LABEL = "商品圖片取景區，可拖曳、使用中鍵或方向鍵調整位置";

async function openProductImageEditor(page: Page) {
	await page
		.locator('.img-uploader input[type="file"][multiple]')
		.setInputFiles({
			name: "portrait-product.svg",
			mimeType: "image/svg+xml",
			buffer: Buffer.from(PORTRAIT_PRODUCT_SVG),
		});

	const dialog = page.getByRole("dialog", { name: "調整商品圖片" });
	const viewport = dialog.locator('[data-product-image-viewport="true"]');
	const cropRegion = dialog.getByLabel(PRODUCT_CROP_LABEL, { exact: true });
	const scaleSlider = dialog.getByRole("slider", {
		name: "商品圖片縮放",
	});

	await expect(dialog).toBeVisible();
	await expect(viewport).toBeVisible();
	await expect(cropRegion).toBeVisible();
	await expect(scaleSlider).toBeVisible();

	return { cropRegion, dialog, scaleSlider, viewport };
}

async function expectDefaultProductFraming(dialog: Locator, viewport: Locator) {
	await expect(
		dialog.getByText("完整原圖 · 未裁切", { exact: true }),
	).toBeVisible();
	await expect(viewport).toHaveAttribute("data-product-image-scale", "1.0000");
	await expect(viewport).toHaveAttribute(
		"data-product-image-offset-y",
		"0.000000",
	);
	await expect(viewport).toHaveAttribute(
		"data-product-image-will-crop",
		"false",
	);
}

function numericAttribute(locator: Locator, name: string) {
	return locator.getAttribute(name).then((value) => Number(value));
}

function touchPoint(id: number, x: number, y: number) {
	return {
		id,
		x,
		y,
		radiusX: 2,
		radiusY: 2,
		force: 1,
	};
}

test.describe("seller product image framing", { tag: ["@ui"] }, () => {
	test.beforeEach(async ({ page, baseURL }) => {
		if (!baseURL) throw new Error("Playwright baseURL is required");

		await page.route("**/api/v1/**", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({ code: 0, data: {} }),
			}),
		);
		await setupGatewayMocks(page, {
			identity: {
				accountId: "1",
				roles: ["seller"],
			},
			session: "identity",
		});
		await mockMyStore(page, { id: "2683638500", storeName: "商品圖測試商店" });
		await mockMyKyc(page, { status: "APPROVED" });
		await page
			.context()
			.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
		await seedStoredActiveRole(page, "seller");
		await page.addInitScript(() => {
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		await page.goto("/seller-center/products/new", {
			waitUntil: "domcontentloaded",
		});
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();
		await expect(page.getByRole("heading", { name: "新增商品" })).toBeVisible();
	});

	test("portrait uploads default to the complete uncropped image", async ({
		page,
	}) => {
		const { dialog, scaleSlider, viewport } =
			await openProductImageEditor(page);

		await expectDefaultProductFraming(dialog, viewport);
		await expect(scaleSlider).toHaveValue("1");
		await expect(dialog).toContainText("600 × 1000 px / 輸出 1000 × 750 px");
	});

	test("desktop wheel zoom, middle-button pan, and reset update framing", async ({
		isMobile,
		page,
	}) => {
		test.skip(Boolean(isMobile), "Desktop mouse gestures only");

		const { cropRegion, dialog, scaleSlider, viewport } =
			await openProductImageEditor(page);
		await expectDefaultProductFraming(dialog, viewport);
		await cropRegion.scrollIntoViewIfNeeded();
		const cropBox = await cropRegion.boundingBox();
		if (!cropBox)
			throw new Error("Product image crop region has no layout box");
		const centerX = cropBox.x + cropBox.width / 2;
		const centerY = cropBox.y + cropBox.height / 2;

		await page.mouse.move(centerX, centerY);
		await page.mouse.wheel(0, -120);
		await expect
			.poll(() => numericAttribute(viewport, "data-product-image-scale"))
			.toBeGreaterThan(1.05);
		await expect(viewport).toHaveAttribute(
			"data-product-image-will-crop",
			"true",
		);
		await expect(
			dialog.getByText("自訂取景 · 框外會裁切", { exact: true }),
		).toBeVisible();
		await expect
			.poll(async () => Number(await scaleSlider.inputValue()))
			.toBeGreaterThan(1.05);

		const offsetBeforeDrag = await numericAttribute(
			viewport,
			"data-product-image-offset-y",
		);
		await page.mouse.move(centerX, centerY);
		await page.mouse.down({ button: "middle" });
		try {
			await page.mouse.move(
				centerX,
				centerY + Math.min(72, cropBox.height * 0.25),
				{ steps: 8 },
			);
		} finally {
			await page.mouse.up({ button: "middle" });
		}
		await expect
			.poll(async () =>
				Math.abs(
					(await numericAttribute(viewport, "data-product-image-offset-y")) -
						offsetBeforeDrag,
				),
			)
			.toBeGreaterThan(0.01);

		await dialog.getByRole("button", { name: "重設", exact: true }).click();
		await expectDefaultProductFraming(dialog, viewport);
		await expect(scaleSlider).toHaveValue("1");
	});

	test("mobile pinch zoom, one-finger pan, and reset update framing", async ({
		isMobile,
		page,
	}) => {
		test.skip(!isMobile, "Mobile touch gestures only");

		const { dialog, scaleSlider, viewport } =
			await openProductImageEditor(page);
		await expectDefaultProductFraming(dialog, viewport);
		await viewport.scrollIntoViewIfNeeded();
		const viewportBox = await viewport.boundingBox();
		if (!viewportBox) {
			throw new Error("Product image viewport has no layout box");
		}
		const centerX = viewportBox.x + viewportBox.width / 2;
		const centerY = viewportBox.y + viewportBox.height / 2;
		const cdp = await page.context().newCDPSession(page);

		try {
			const initialHalfDistance = Math.min(28, viewportBox.width * 0.12);
			const finalHalfDistance = Math.min(76, viewportBox.width * 0.3);
			await cdp.send("Input.dispatchTouchEvent", {
				type: "touchStart",
				touchPoints: [
					touchPoint(1, centerX - initialHalfDistance, centerY),
					touchPoint(2, centerX + initialHalfDistance, centerY),
				],
			});
			for (let step = 1; step <= 8; step += 1) {
				const progress = step / 8;
				const halfDistance =
					initialHalfDistance +
					(finalHalfDistance - initialHalfDistance) * progress;
				await cdp.send("Input.dispatchTouchEvent", {
					type: "touchMove",
					touchPoints: [
						touchPoint(1, centerX - halfDistance, centerY),
						touchPoint(2, centerX + halfDistance, centerY),
					],
				});
				await page.waitForTimeout(16);
			}
			await cdp.send("Input.dispatchTouchEvent", {
				type: "touchEnd",
				touchPoints: [],
			});

			await expect
				.poll(() => numericAttribute(viewport, "data-product-image-scale"))
				.toBeGreaterThan(1.4);
			await expect(viewport).toHaveAttribute(
				"data-product-image-will-crop",
				"true",
			);
			await expect
				.poll(async () => Number(await scaleSlider.inputValue()))
				.toBeGreaterThan(1.4);

			const offsetBeforeDrag = await numericAttribute(
				viewport,
				"data-product-image-offset-y",
			);
			await cdp.send("Input.dispatchTouchEvent", {
				type: "touchStart",
				touchPoints: [touchPoint(3, centerX, centerY)],
			});
			const dragDistance = Math.min(64, viewportBox.height * 0.24);
			for (let step = 1; step <= 8; step += 1) {
				await cdp.send("Input.dispatchTouchEvent", {
					type: "touchMove",
					touchPoints: [
						touchPoint(3, centerX, centerY + dragDistance * (step / 8)),
					],
				});
				await page.waitForTimeout(16);
			}
			await cdp.send("Input.dispatchTouchEvent", {
				type: "touchEnd",
				touchPoints: [],
			});
			await expect
				.poll(async () =>
					Math.abs(
						(await numericAttribute(viewport, "data-product-image-offset-y")) -
							offsetBeforeDrag,
					),
				)
				.toBeGreaterThan(0.01);
		} finally {
			await cdp
				.send("Input.dispatchTouchEvent", {
					type: "touchEnd",
					touchPoints: [],
				})
				.catch(() => undefined);
			await cdp.detach();
		}

		await dialog.getByRole("button", { name: "重設", exact: true }).click();
		await expectDefaultProductFraming(dialog, viewport);
		await expect(scaleSlider).toHaveValue("1");
	});
});
