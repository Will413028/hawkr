import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import RouteLoadingSkeleton, {
	FeedGridSkeleton,
	ProductGridSkeleton,
} from "@/components/shared/RouteLoadingSkeleton";

const meta = {
	title: "Web/Feedback/RouteLoadingSkeleton",
	component: RouteLoadingSkeleton,
	args: { page: true, variant: "market-list" },
	parameters: {
		layout: "fullscreen",
		docs: {
			description: {
				component:
					"正式頁面與局部載入骨架。使用正式 CSS，無 API 或交易；viewport 變更時跟隨各使用端的斷點與容器。page 為一般文件流，scope 為指定區域覆蓋，columns 僅決定預留卡片數。",
			},
		},
	},
	play: async ({ canvas, canvasElement }) => {
		await expect(canvas.getByRole("status")).toHaveAttribute(
			"aria-label",
			"頁面載入中",
		);
		const root = canvasElement.querySelector(".route-loading-skeleton");
		await expect(root).not.toBeNull();
		if (root) {
			const bounds = root.getBoundingClientRect();
			for (const element of root.querySelectorAll(
				".route-loading-skeleton__purchase > span, .route-loading-skeleton__commerce-main > span, .route-loading-skeleton__workspace-head",
			)) {
				const box = element.getBoundingClientRect();
				await expect(box.right).toBeLessThanOrEqual(bounds.right + 1);
			}
		}
	},
} satisfies Meta<typeof RouteLoadingSkeleton>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Market: Story = {};
export const Search: Story = { args: { variant: "search" } };
export const Feed: Story = { args: { variant: "feed" } };
export const Home: Story = { args: { variant: "home" } };
export const ProductDetail: Story = { args: { variant: "product-detail" } };
export const Storefront: Story = { args: { variant: "seller-storefront" } };
export const Checkout: Story = { args: { variant: "checkout" } };
export const Auth: Story = { args: { variant: "auth" } };
export const Workspace: Story = { args: { variant: "workspace" } };
export const Generic: Story = { args: { variant: "generic" } };
export const FeedResults: Story = {
	render: () => <FeedGridSkeleton />,
	play: async ({ canvas, canvasElement }) => {
		await expect(canvas.getByRole("status")).toHaveAttribute(
			"aria-label",
			"動態載入中",
		);
		await expect(
			canvasElement.querySelector(".route-story-skeleton"),
		).toBeNull();
		await expect(
			canvasElement.querySelector(".route-loading-skeleton"),
		).toBeNull();
	},
};
export const SearchResults: Story = {
	render: () => <ProductGridSkeleton variant="search" announce={false} />,
	play: async ({ canvas, canvasElement }) => {
		await expect(canvas.queryByRole("status")).toBeNull();
		await expect(
			canvasElement.querySelector(".route-product-grid-skeleton"),
		).toHaveAttribute("aria-hidden", "true");
	},
};
export const ProductResults: Story = {
	render: () => (
		<div className="container">
			<ProductGridSkeleton />
		</div>
	),
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("status")).toHaveAttribute(
			"aria-label",
			"商品載入中",
		);
	},
};
