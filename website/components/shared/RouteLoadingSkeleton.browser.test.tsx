import { describe, expect, it } from "vitest";
import { page } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import RouteLoadingSkeleton, {
	FeedGridSkeleton,
	ProductGridSkeleton,
} from "./RouteLoadingSkeleton";

type Variant = React.ComponentProps<typeof RouteLoadingSkeleton>["variant"];

const variants: Variant[] = [
	"market-list",
	"search",
	"feed",
	"home",
	"product-detail",
	"seller-storefront",
	"checkout",
	"auth",
	"workspace",
	"generic",
];

describe("RouteLoadingSkeleton", () => {
	it.each(variants)(
		"labels the %s page skeleton and keeps its blocks inside the root",
		async (variant) => {
			const { container } = renderWithIntl(
				<RouteLoadingSkeleton page variant={variant} />,
			);
			await expect
				.element(page.getByRole("status"))
				.toHaveAttribute("aria-label", "頁面載入中");
			const root = container.querySelector(".route-loading-skeleton");
			expect(root).not.toBeNull();
			if (root) {
				const bounds = root.getBoundingClientRect();
				for (const element of root.querySelectorAll(
					".route-loading-skeleton__purchase > span, .route-loading-skeleton__commerce-main > span, .route-loading-skeleton__workspace-head",
				)) {
					const box = element.getBoundingClientRect();
					expect(box.right).toBeLessThanOrEqual(bounds.right + 1);
				}
			}
		},
	);

	it("labels the feed results skeleton without the page skeleton wrappers", async () => {
		const { container } = renderWithIntl(<FeedGridSkeleton />);
		await expect
			.element(page.getByRole("status"))
			.toHaveAttribute("aria-label", "動態載入中");
		expect(container.querySelector(".route-story-skeleton")).toBeNull();
		expect(container.querySelector(".route-loading-skeleton")).toBeNull();
	});

	it("hides the unannounced search results skeleton from assistive tech", async () => {
		const { container } = renderWithIntl(
			<ProductGridSkeleton variant="search" announce={false} />,
		);
		await expect.element(page.getByRole("status")).not.toBeInTheDocument();
		await expect
			.element(
				container.querySelector<HTMLElement>(".route-product-grid-skeleton"),
			)
			.toHaveAttribute("aria-hidden", "true");
	});

	it("labels the product results skeleton", async () => {
		renderWithIntl(
			<div className="container">
				<ProductGridSkeleton />
			</div>,
		);
		await expect
			.element(page.getByRole("status"))
			.toHaveAttribute("aria-label", "商品載入中");
	});
});
