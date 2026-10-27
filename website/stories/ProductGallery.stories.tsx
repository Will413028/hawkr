import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, userEvent } from "storybook/test";
import ProductGallery from "@/features/product/components/ProductGallery";
import { __setEnvForTest, getEnv } from "@/lib/config/env";

const thumbnails = ["#b8441f", "#e4572e", "#1f2933", "#1e5fab"].map(
	(color, index) =>
		`data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="700" height="525"><rect width="700" height="525" fill="${color}"/><text x="350" y="285" text-anchor="middle" font-size="64" fill="white">${index + 1}</text></svg>`)}`,
);
const meta = {
	title: "Web/Product/ProductGallery",
	component: ProductGallery,
	tags: ["autodocs"],
	args: { thumbnails, productName: "商品圖片測試", tags: [] },
	decorators: [
		(Story) => (
			<div style={{ maxWidth: 700 }}>
				<Story />
			</div>
		),
	],
} satisfies Meta<typeof ProductGallery>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Progressive: Story = {
	play: async ({ canvas }) => {
		for (const index of [4, 2, 3, 1, 4]) {
			const button = canvas.getByRole("button", {
				name: `查看商品圖片 ${index}`,
			});
			await userEvent.click(button);
			await expect(button).toHaveAttribute("aria-pressed", "true");
		}
	},
};

/** Local gateway fixtures: no product API, external media or transactions. */
export const Responsive: Story = {
	args: {
		thumbnails: [1, 2, 3, 4].map(
			(index) => `public/product/gallery-fixture/${index}.svg`,
		),
	},
	beforeEach: () => {
		__setEnvForTest({
			...getEnv(),
			flavor: "dev",
			mediaBaseUrl: window.location.origin,
		});
		return () => __setEnvForTest(null);
	},
	play: Progressive.play,
};

export const CardMotion: Story = {
	name: "整張卡片切換",
	args: { tags: ["當季"] },
	play: async ({ canvas }) => {
		const gallery = canvas.getByRole("region", { name: "商品圖片" });
		gallery.focus();
		await userEvent.keyboard("{ArrowRight}");
		await expect(
			canvas.getByRole("button", { name: "查看商品圖片 2" }),
		).toHaveAttribute("aria-pressed", "true");
		await userEvent.keyboard("{End}");
		await expect(
			canvas.getByRole("button", { name: "查看商品圖片 4" }),
		).toHaveAttribute("aria-pressed", "true");
		await userEvent.keyboard("{Home}");
		await expect(
			canvas.getByRole("button", { name: "查看商品圖片 1" }),
		).toHaveAttribute("aria-pressed", "true");
	},
};
