import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Stars } from "@/components/ui/Stars";

const meta = {
	title: "Web/Data Display/Stars",
	component: Stars,
	args: { count: 4 },
} satisfies Meta<typeof Stars>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Rated: Story = {};
export const Unrated: Story = { args: { count: 0 } };
export const Full: Story = { args: { count: 5 } };
