import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, userEvent, waitFor } from "storybook/test";
import { Input } from "@/components/ui/Input";

const meta = {
	title: "Web/Forms/Input",
	component: Input,
	args: {
		label: "商品名稱",
		placeholder: "例如：花蓮小農香米",
		isRequired: true,
	},
	decorators: [
		(Story) => (
			<div className="catalog-field">
				<Story />
			</div>
		),
	],
	parameters: {
		docs: {
			description: {
				component:
					"正式站共用輸入框。預設描邊、底色、尺寸與焦點狀態對齊買家／賣家 App 的 AppTheme 輸入欄位；保留 flat、faded 與 underlined 變體。",
			},
		},
	},
} satisfies Meta<typeof Input>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Default: Story = {
	play: async ({ canvas }) => {
		const input = canvas.getByRole("textbox");
		await userEvent.type(input, "花蓮小農香米");
		await expect(input).toHaveValue("花蓮小農香米");
	},
};
export const Invalid: Story = {
	args: { isInvalid: true, errorMessage: "請輸入商品名稱" },
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("textbox")).toHaveAttribute(
			"aria-invalid",
			"true",
		);
		await expect(canvas.getByText("請輸入商品名稱")).toBeVisible();
	},
};
export const Disabled: Story = {
	args: { disabled: true, defaultValue: "等待審核中" },
};
export const ReadOnly: Story = {
	args: {
		readOnly: true,
		defaultValue: "花蓮小農香米",
		description: "此欄位由商品資料帶入",
	},
};
export const FloatingLabel: Story = {
	args: { floatingLabel: true, defaultValue: "花蓮小農香米" },
};

function inputWrapper(input: HTMLElement): HTMLElement {
	const wrapper = input.closest<HTMLElement>('[data-slot="input-wrapper"]');
	if (!wrapper) throw new Error("Input must render its field wrapper");
	return wrapper;
}

function renderedStyle(element: Element, pseudo?: string): CSSStyleDeclaration {
	const view = element.ownerDocument.defaultView;
	if (!view) throw new Error("Input preview must have a browser window");
	return view.getComputedStyle(element, pseudo);
}

export const AppAppearance: Story = {
	name: "App 對齊 · 欄位狀態",
	globals: { theme: "light", textScale: "1" },
	render: () => (
		<div className="catalog-stack">
			<Input
				label="商品名稱"
				placeholder="花蓮小農香米"
				description="表單內容僅保留於此預覽。"
			/>
			<Input label="系統商品編號" placeholder="系統商品編號" disabled />
			<Input label="唯讀商品名稱" defaultValue="花蓮小農香米" readOnly />
			<Input
				label="庫存"
				placeholder="庫存"
				isInvalid
				errorMessage="庫存不可小於 0"
			/>
			<Input label="收件人姓名" floatingLabel />
		</div>
	),
	play: async ({ canvas }) => {
		const normal = canvas.getByRole("textbox", {
			name: "商品名稱",
		});
		const disabled = canvas.getByRole("textbox", { name: "系統商品編號" });
		const readOnly = canvas.getByRole("textbox", { name: "唯讀商品名稱" });
		const invalid = canvas.getByRole("textbox", { name: "庫存" });
		const floating = canvas.getByRole("textbox", { name: "收件人姓名" });
		const normalWrapper = inputWrapper(normal);
		const errorWrapper = inputWrapper(invalid);
		const floatingWrapper = inputWrapper(floating);
		const floatingLabel = canvas.getByText("收件人姓名");

		// These are the visible App field metrics, independent of utility classes.
		await waitFor(() => {
			expect(normalWrapper.getBoundingClientRect().height).toBe(48);
			expect(renderedStyle(normal).fontSize).toBe("16px");
			expect(renderedStyle(normalWrapper).borderRadius).toBe("12px");
			expect(renderedStyle(normalWrapper).backgroundColor).toBe(
				"rgb(255, 253, 247)",
			);
			expect(renderedStyle(normalWrapper).borderTopWidth).toBe("1px");
			expect(renderedStyle(normalWrapper).borderTopColor).toBe(
				"rgb(222, 216, 203)",
			);
		});
		await expect(normal).toHaveAccessibleDescription(
			"表單內容僅保留於此預覽。",
		);
		await expect(disabled).toBeDisabled();
		await expect(renderedStyle(inputWrapper(disabled)).backgroundColor).toBe(
			"rgb(255, 253, 247)",
		);

		await userEvent.click(normal);
		await userEvent.keyboard("花蓮小農香米");
		await expect(normal).toHaveValue("花蓮小農香米");
		await waitFor(() => {
			expect(normalWrapper.getBoundingClientRect().height).toBe(48);
			expect(renderedStyle(normalWrapper).boxShadow).toContain("inset");
			expect(renderedStyle(normalWrapper, "::before").backgroundImage).toBe(
				"none",
			);
			expect(renderedStyle(normalWrapper, "::after").backgroundImage).toBe(
				"none",
			);
		});
		await userEvent.tab();
		await expect(readOnly).toHaveFocus();
		await userEvent.keyboard("不可修改");
		await expect(readOnly).toHaveValue("花蓮小農香米");
		await expect(readOnly).toHaveAttribute("readonly");

		const errorColor = renderedStyle(errorWrapper).borderTopColor;
		await userEvent.tab();
		await expect(invalid).toHaveFocus();
		await expect(invalid).toHaveAttribute("aria-invalid", "true");
		await expect(invalid).toHaveAccessibleDescription("庫存不可小於 0");
		await waitFor(() => {
			expect(renderedStyle(errorWrapper).borderTopColor).toBe(errorColor);
			expect(renderedStyle(errorWrapper).boxShadow).toContain("inset");
			expect(renderedStyle(errorWrapper, "::before").backgroundImage).toBe(
				"none",
			);
			expect(renderedStyle(errorWrapper, "::after").backgroundImage).toBe(
				"none",
			);
		});

		await userEvent.tab();
		await expect(floating).toHaveFocus();
		await userEvent.keyboard("林小農");
		await userEvent.click(normal);
		await expect(floating).toHaveValue("林小農");
		await expect(floating).not.toHaveFocus();
		await expect(floatingLabel).toBeVisible();
		await waitFor(() => {
			const fieldBounds = floatingWrapper.getBoundingClientRect();
			const labelBounds = floatingLabel.getBoundingClientRect();
			expect(fieldBounds.height).toBe(48);
			expect(labelBounds.top).toBeLessThan(fieldBounds.top);
			expect(labelBounds.bottom).toBeGreaterThan(fieldBounds.top);
			expect(labelBounds.bottom).toBeLessThan(
				floating.getBoundingClientRect().top,
			);
		});
	},
};

export const CompactLargeText: Story = {
	globals: { theme: "light", textScale: "1.4" },
	parameters: {
		docs: {
			description: {
				story:
					"320px 欄位寬度與 1.4 倍字級，檢查浮動標籤、長文字與說明的可讀性。",
			},
		},
	},
	render: () => (
		<div className="catalog-stack" style={{ width: 320, maxWidth: "100%" }}>
			<Input label="收件人姓名" floatingLabel defaultValue="林小農" />
			<Input
				label="商品名稱（請填寫完整品名）"
				defaultValue="花蓮小農自然栽培香米"
				description="欄位使用較大文字時，說明仍應完整換行顯示。"
			/>
		</div>
	),
	play: async ({ canvas }) => {
		const floating = canvas.getByRole("textbox", { name: "收件人姓名" });
		const label = canvas.getByText("收件人姓名");
		await waitFor(() => {
			expect(
				Number.parseFloat(renderedStyle(floating).fontSize),
			).toBeGreaterThanOrEqual(22);
			for (const field of canvas.getAllByRole("textbox")) {
				const wrapper = inputWrapper(field);
				const fieldBounds = field.getBoundingClientRect();
				const wrapperBounds = wrapper.getBoundingClientRect();
				expect(wrapperBounds.width).toBeLessThanOrEqual(320);
				expect(fieldBounds.left).toBeGreaterThan(wrapperBounds.left);
				expect(fieldBounds.right).toBeLessThan(wrapperBounds.right);
			}
			expect(label.getBoundingClientRect().bottom).toBeLessThanOrEqual(
				floating.getBoundingClientRect().top,
			);
		});
		await expect(floating).toHaveValue("林小農");
		await expect(
			canvas.getByText("欄位使用較大文字時，說明仍應完整換行顯示。"),
		).toBeVisible();
	},
};

export const LegacyVariants: Story = {
	render: () => (
		<div className="catalog-stack">
			<Input label="Flat 商品名稱" variant="flat" defaultValue="花蓮小農香米" />
			<Input
				label="Faded 商品名稱"
				variant="faded"
				defaultValue="花蓮小農香米"
			/>
			<Input
				label="Underlined 商品名稱"
				variant="underlined"
				defaultValue="花蓮小農香米"
			/>
			<Input label="緊密尺寸商品名稱" size="sm" defaultValue="花蓮小農香米" />
		</div>
	),
};

export const Password: Story = {
	args: {
		label: "密碼",
		type: "password",
		placeholder: "請輸入密碼",
		autoComplete: "new-password",
	},
};

export const DarkInvalid: Story = {
	args: { isInvalid: true, errorMessage: "請輸入商品名稱" },
	globals: { theme: "dark" },
	play: async ({ canvasElement, canvas }) => {
		await expect(canvasElement.ownerDocument.documentElement).toHaveAttribute(
			"data-theme",
			"dark",
		);
		await expect(canvas.getByRole("textbox")).toHaveAccessibleDescription(
			"請輸入商品名稱",
		);
	},
};
