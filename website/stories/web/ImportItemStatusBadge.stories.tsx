import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import { ImportItemStatusBadge } from "@/features/seller-center/products-import/components/ImportItemStatusBadge";
import { IMPORT_ITEM_STATUS } from "@/features/seller-center/products-import/types";

const meta = {
	title: "Web/Seller Center/ImportItemStatusBadge",
	component: ImportItemStatusBadge,
	args: { status: IMPORT_ITEM_STATUS.READY },
	parameters: {
		docs: {
			description: {
				component:
					"賣家中心商品匯入的正式狀態元件，沿用既有狀態碼與繁中文案。READY 但 needsFix 時必須顯示「需修正」，不能把可預覽誤當完成上架。此展示不呼叫後端。",
			},
		},
	},
} satisfies Meta<typeof ImportItemStatusBadge>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Ready: Story = {};
export const NeedsCorrection: Story = {
	args: { needsFix: true },
	play: async ({ canvas }) => {
		await expect(canvas.getByText("需修正")).toBeVisible();
		await expect(canvas.queryByText("可上架")).not.toBeInTheDocument();
	},
};
export const Failed: Story = {
	args: { status: IMPORT_ITEM_STATUS.COMMIT_FAILED },
};
export const AllImportStates: Story = {
	render: () => (
		<div className="catalog-row">
			{Object.values(IMPORT_ITEM_STATUS).map((status) => (
				<ImportItemStatusBadge key={status} status={status} />
			))}
		</div>
	),
};
