import { describe, expect, it } from "vitest";
import { page } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import ProductReviewsSection from "./ProductReviewsSection";
import ProductReviewsSkeleton from "./ProductReviewsSkeleton";

const reviews = [0, 1].map((index) => ({
	id: index,
	author: "預覽買家",
	date: "2026/09/13",
	rating: 5,
	text: "元件預覽評價，用來檢查文字、頭像與評分在各種寬度下的位置。",
}));

describe("ProductReviewsSkeleton", () => {
	it("is busy and matches the loaded section's card width and offset", async () => {
		const { container } = renderWithIntl(
			<div className="mx-auto max-w-3xl">
				<div data-review-loading>
					<ProductReviewsSkeleton />
				</div>
				<div data-review-loaded>
					<ProductReviewsSection reviews={reviews} reviewTotal={2} />
				</div>
			</div>,
		);
		await expect
			.element(page.getByRole("status"))
			.toHaveAttribute("aria-busy", "true");
		const loading = container.querySelector(
			"[data-review-loading]",
		) as HTMLElement;
		const loaded = container.querySelector(
			"[data-review-loaded]",
		) as HTMLElement;
		const pendingCard = loading.querySelector(
			".public-section-surface",
		) as HTMLElement;
		const readyCard = loaded.querySelector(
			".public-section-surface",
		) as HTMLElement;
		expect(
			Math.abs(
				pendingCard.getBoundingClientRect().width -
					readyCard.getBoundingClientRect().width,
			),
		).toBeLessThan(1);
		expect(
			Math.abs(
				pendingCard.getBoundingClientRect().top -
					loading.getBoundingClientRect().top -
					(readyCard.getBoundingClientRect().top -
						loaded.getBoundingClientRect().top),
			),
		).toBeLessThan(1);
	});
});
