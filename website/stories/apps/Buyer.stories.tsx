import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { FlutterCatalog, sharedCatalogComponents } from "./FlutterCatalog";

const meta = {
	title: "Apps/Buyer",
	component: FlutterCatalog,
	args: { app: "buyer", component: "buttons" },
	argTypes: {
		app: { control: false },
		component: {
			control: "select",
			options: [...sharedCatalogComponents, "cart"],
		},
	},
	tags: ["!test"],
	parameters: {
		docs: {
			description: {
				component:
					"買家正式 Flutter widget 預覽。Dart constructor 與狀態請看 App docs/component-library.md；MCP 的 React props 是 iframe 控制項，不是 Flutter API。Flutter 互動與版面由 App widget tests 驗證。",
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
export const Cart: Story = { args: { component: "cart" } };
export const TemperatureTags: Story = { args: { component: "chips" } };
export const TemperatureTagsLargeText: Story = {
	args: { component: "chips", width: 320, textScale: "1.4" },
};
export const CompactLargeText: Story = {
	args: { width: 320, textScale: "1.4" },
};
