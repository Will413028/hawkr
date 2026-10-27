import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { FlutterCatalog, sharedCatalogComponents } from "./FlutterCatalog";

const meta = {
	title: "Apps/Seller",
	component: FlutterCatalog,
	args: { app: "seller", component: "buttons" },
	argTypes: {
		app: { control: false },
		component: { control: "select", options: [...sharedCatalogComponents] },
	},
	tags: ["!test"],
	parameters: {
		docs: {
			description: {
				component:
					"賣家正式 Flutter widget 預覽。Dart API 以 App 原始元件與 docs/component-library.md 為準。MCP tests 不會穿透 Canvas iframe；由 Flutter widget tests 驗證。",
			},
		},
	},
} satisfies Meta<typeof FlutterCatalog>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Buttons: Story = {};
export const Foundations: Story = { args: { component: "foundations" } };
export const Inputs: Story = { args: { component: "inputs" } };
export const Chips: Story = { args: { component: "chips" } };
export const Cards: Story = { args: { component: "cards" } };
export const States: Story = { args: { component: "states" } };
export const Commerce: Story = { args: { component: "commerce" } };
export const TemperatureTags: Story = { args: { component: "commerce" } };
export const TemperatureTagsLargeText: Story = {
	args: { component: "commerce", width: 320, textScale: "1.4" },
};
export const CompactLargeText: Story = {
	args: { width: 320, textScale: "1.4" },
};
