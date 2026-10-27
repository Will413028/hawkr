import { expect, test } from "@playwright/test";

test.setTimeout(90_000);

test("market category updates immediately and the chooser closes smoothly", {
	tag: ["@ui", "@live"],
}, async ({ page }) => {
	const insertionEffectErrors: string[] = [];
	page.on("console", (message) => {
		if (
			message.type() === "error" &&
			message.text().includes("useInsertionEffect must not schedule updates")
		) {
			insertionEffectErrors.push(message.text());
		}
	});

	await page.goto("/market/products", { waitUntil: "networkidle" });
	await expect(page.getByRole("status", { name: "Hawkr 載入中" })).toBeHidden({
		timeout: 30_000,
	});

	const isMobile = (page.viewportSize()?.width ?? 1280) <= 840;
	if (isMobile) {
		const activeNav = page.locator(
			'nav[aria-label="手機底部導覽"] a[aria-current="page"]',
		);
		await expect(activeNav).toContainText("市集", { timeout: 30_000 });
		await expect(
			page.locator('[data-mobile-nav-active-indicator="true"]'),
		).toBeVisible();
		await page.locator(".market-list-sidebar__toggle").click();
	} else {
		const activeNav = page.locator('.nav-item[aria-current="page"]');
		await expect(activeNav).toContainText("Hawkr 市集", { timeout: 30_000 });
		await expect(
			activeNav.locator('[data-nav-active-indicator="true"]'),
		).toBeVisible();
	}

	const trigger = page.locator(".market-category-popover__trigger");
	await trigger.click();
	const dialog = page.getByRole("dialog", { name: "分類篩選" });
	await expect(dialog).toBeVisible();

	const cabbage = dialog
		.locator("label.market-category-popover__checkbox")
		.filter({ hasText: "小白菜" })
		.first();
	await cabbage.click();

	await expect(page).toHaveURL(/(?:\?|&)categories=12(?:&|$)/);
	await expect(dialog).toBeVisible();
	await expect(
		page.getByRole("link", { name: /移除分類.*小白菜/ }),
	).toBeVisible();

	const done = dialog.getByRole("button", { name: "完成" });
	await expect(done).toBeVisible();
	// TanStack Query's local-only devtools badge overlaps this corner on Pixel 5.
	// Production does not render it; force the click after the visibility gate.
	await done.click({ force: true });
	await expect(dialog).toBeHidden();
	await expect(trigger).toHaveAttribute("aria-expanded", "false");
	expect(insertionEffectErrors).toEqual([]);
});

test("category chooser remains readable and keyboard-dismissible in dark mode", {
	tag: ["@ui", "@live"],
}, async ({ page }) => {
	await page.addInitScript(() => localStorage.setItem("hk:theme", "dark"));
	await page.goto("/market/products", { waitUntil: "networkidle" });
	await expect(page.getByRole("status", { name: "Hawkr 載入中" })).toBeHidden({
		timeout: 30_000,
	});
	await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

	if ((page.viewportSize()?.width ?? 1280) <= 840) {
		await page.locator(".market-list-sidebar__toggle").click();
	}
	await page.locator(".market-category-popover__trigger").click();
	const dialog = page.getByRole("dialog", { name: "分類篩選" });
	await expect(dialog).toBeVisible();
	const colors = await dialog.evaluate((element) => {
		const style = getComputedStyle(element);
		return { background: style.backgroundColor, foreground: style.color };
	});
	expect(colors.background).not.toBe("rgba(0, 0, 0, 0)");
	expect(colors.foreground).not.toBe(colors.background);

	await page.keyboard.press("Escape");
	await expect(dialog).toBeHidden();
});
