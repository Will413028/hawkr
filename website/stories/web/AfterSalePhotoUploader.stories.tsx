import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect } from "storybook/test";
import {
	AfterSalePhotoUploader,
	type StagedPhoto,
} from "@/features/after-sales/components/AfterSalePhotoUploader";

const PREVIEW =
	"data:image/svg+xml;utf8,%3Csvg xmlns='http://www.w3.org/2000/svg' width='96' height='96'%3E%3Crect width='96' height='96' fill='%23e8efe3'/%3E%3C/svg%3E";

const PHOTOS: StagedPhoto[] = [
	{ token: "token-a", previewUrl: PREVIEW },
	{ token: "token-b", previewUrl: PREVIEW },
];

const meta = {
	title: "Web/Forms/AfterSalePhotoUploader",
	component: AfterSalePhotoUploader,
	args: {
		value: PHOTOS,
		onChange: () => {},
		max: 5,
		label: "佐證圖片（選填）",
	},
	render: function Controlled(args) {
		const [value, setValue] = useState(args.value);
		return (
			<div className="catalog-field">
				<AfterSalePhotoUploader {...args} value={value} onChange={setValue} />
			</div>
		);
	},
} satisfies Meta<typeof AfterSalePhotoUploader>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Default: Story = {};
export const ItemError: Story = {
	args: { itemErrors: { "token-b": "這張照片已過期，請移除後重新新增" } },
	play: async ({ canvas }) => {
		await expect(
			canvas.getByText("這張照片已過期，請移除後重新新增"),
		).toBeVisible();
	},
};
export const FieldError: Story = {
	args: { error: "佐證圖片最多 5 張" },
	play: async ({ canvas }) => {
		await expect(canvas.getByText("佐證圖片最多 5 張")).toBeVisible();
	},
};
