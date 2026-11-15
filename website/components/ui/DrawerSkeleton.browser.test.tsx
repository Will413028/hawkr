import { describe, expect, it } from "vitest";
import { page } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { DrawerSkeleton } from "./DrawerSkeleton";

describe("DrawerSkeleton", () => {
	it("announces a busy status with the loading label", async () => {
		renderWithIntl(<DrawerSkeleton rows={2} />, { frameMaxWidth: 768 });
		const status = page.getByRole("status");
		await expect.element(status).toHaveAttribute("aria-busy", "true");
		// 子字串比對（vitest/browser 的 toHaveTextContent 字串是整段比對）。
		await expect.poll(() => status.element().textContent).toContain("載入中");
	});
});
