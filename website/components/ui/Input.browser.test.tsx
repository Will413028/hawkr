import { afterEach, describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { Input } from "./Input";

function inputWrapper(input: HTMLElement): HTMLElement {
	const wrapper = input.closest<HTMLElement>('[data-slot="input-wrapper"]');
	if (!wrapper) throw new Error("Input must render its field wrapper");
	return wrapper;
}

function renderedStyle(element: Element, pseudo?: string): CSSStyleDeclaration {
	return getComputedStyle(element, pseudo);
}

const textbox = (name: string) =>
	page.getByRole("textbox", { name, exact: true });

afterEach(() => {
	document.documentElement.style.removeProperty("font-size");
	document.documentElement.style.removeProperty("--hk-text-scale");
});

describe("Input", () => {
	it("accepts typed text", async () => {
		renderWithIntl(
			<Input label="商品名稱" placeholder="例如：花蓮小農香米" isRequired />,
		);
		const input = page.getByRole("textbox");
		await userEvent.type(input, "花蓮小農香米");
		await expect.element(input).toHaveValue("花蓮小農香米");
	});

	it("marks itself invalid and shows the error message", async () => {
		renderWithIntl(
			<Input
				label="商品名稱"
				placeholder="例如：花蓮小農香米"
				isRequired
				isInvalid
				errorMessage="請輸入商品名稱"
			/>,
		);
		await expect
			.element(page.getByRole("textbox"))
			.toHaveAttribute("aria-invalid", "true");
		await expect
			.element(page.getByText("請輸入商品名稱", { exact: true }))
			.toBeVisible();
	});

	it("matches the App field appearance across states", async () => {
		renderWithIntl(
			<div style={{ display: "grid", gap: 24, maxWidth: 720 }}>
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
			</div>,
		);
		const normalLocator = textbox("商品名稱");
		const normal = normalLocator.element() as HTMLElement;
		const disabledLocator = textbox("系統商品編號");
		const disabled = disabledLocator.element() as HTMLElement;
		const readOnlyLocator = textbox("唯讀商品名稱");
		const invalidLocator = textbox("庫存");
		const invalid = invalidLocator.element() as HTMLElement;
		const floatingLocator = textbox("收件人姓名");
		const floating = floatingLocator.element() as HTMLElement;
		const normalWrapper = inputWrapper(normal);
		const errorWrapper = inputWrapper(invalid);
		const floatingWrapper = inputWrapper(floating);
		const floatingLabel = page
			.getByText("收件人姓名", { exact: true })
			.element();

		// These are the visible App field metrics, independent of utility classes.
		await vi.waitFor(() => {
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
		await expect
			.element(normalLocator)
			.toHaveAccessibleDescription("表單內容僅保留於此預覽。");
		await expect.element(disabledLocator).toBeDisabled();
		expect(renderedStyle(inputWrapper(disabled)).backgroundColor).toBe(
			"rgb(255, 253, 247)",
		);

		await userEvent.click(normalLocator);
		await userEvent.keyboard("花蓮小農香米");
		await expect.element(normalLocator).toHaveValue("花蓮小農香米");
		await vi.waitFor(() => {
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
		await expect.element(readOnlyLocator).toHaveFocus();
		await userEvent.keyboard("不可修改");
		await expect.element(readOnlyLocator).toHaveValue("花蓮小農香米");
		await expect.element(readOnlyLocator).toHaveAttribute("readonly");

		const errorColor = renderedStyle(errorWrapper).borderTopColor;
		await userEvent.tab();
		await expect.element(invalidLocator).toHaveFocus();
		await expect
			.element(invalidLocator)
			.toHaveAttribute("aria-invalid", "true");
		await expect
			.element(invalidLocator)
			.toHaveAccessibleDescription("庫存不可小於 0");
		await vi.waitFor(() => {
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
		await expect.element(floatingLocator).toHaveFocus();
		await userEvent.keyboard("林小農");
		await userEvent.click(normalLocator);
		await expect.element(floatingLocator).toHaveValue("林小農");
		await expect.element(floatingLocator).not.toHaveFocus();
		await expect.element(floatingLabel).toBeVisible();
		await vi.waitFor(() => {
			const fieldBounds = floatingWrapper.getBoundingClientRect();
			const labelBounds = floatingLabel.getBoundingClientRect();
			expect(fieldBounds.height).toBe(48);
			expect(labelBounds.top).toBeLessThan(fieldBounds.top);
			expect(labelBounds.bottom).toBeGreaterThan(fieldBounds.top);
			expect(labelBounds.bottom).toBeLessThan(
				floating.getBoundingClientRect().top,
			);
		});
	});

	it("keeps floating label, long text and description readable at 320px and 1.4x text", async () => {
		const root = document.documentElement;
		renderWithIntl(
			<div
				style={{
					display: "grid",
					gap: 24,
					maxWidth: 720,
					width: 320,
				}}
			>
				<Input label="收件人姓名" floatingLabel defaultValue="林小農" />
				<Input
					label="商品名稱（請填寫完整品名）"
					defaultValue="花蓮小農自然栽培香米"
					description="欄位使用較大文字時，說明仍應完整換行顯示。"
				/>
			</div>,
		);
		root.style.fontSize = "140%";
		root.style.setProperty("--hk-text-scale", "1.4");
		const floatingLocator = textbox("收件人姓名");
		const floating = floatingLocator.element() as HTMLElement;
		const label = page.getByText("收件人姓名", { exact: true }).element();
		await vi.waitFor(() => {
			expect(
				Number.parseFloat(renderedStyle(floating).fontSize),
			).toBeGreaterThanOrEqual(22);
			for (const field of page.getByRole("textbox").elements()) {
				const wrapper = inputWrapper(field as HTMLElement);
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
		await expect.element(floatingLocator).toHaveValue("林小農");
		await expect
			.element(
				page.getByText("欄位使用較大文字時，說明仍應完整換行顯示。", {
					exact: true,
				}),
			)
			.toBeVisible();
	});

	it("describes the input with the error message under the dark theme", async () => {
		renderWithIntl(
			<Input
				label="商品名稱"
				placeholder="例如：花蓮小農香米"
				isRequired
				isInvalid
				errorMessage="請輸入商品名稱"
			/>,
			{ theme: "dark" },
		);
		await expect
			.element(document.documentElement)
			.toHaveAttribute("data-theme", "dark");
		await expect
			.element(page.getByRole("textbox"))
			.toHaveAccessibleDescription("請輸入商品名稱");
	});
});
