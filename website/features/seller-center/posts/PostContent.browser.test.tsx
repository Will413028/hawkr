import { type ComponentProps, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { Textarea } from "@/components/ui/Input";
import { queryRequired } from "@/tests/browser/query";
import { renderWithIntl } from "@/tests/browser/render";
import { highlightHashtags } from "./highlightHashtags";
import "@/styles/features/seller-center.css";

const SCREENSHOT_CONTENT =
	"中秋烤肉\n\n#中秋節\n#中秋烤肉\n#烤肉禮盒#中秋送禮#烤肉組2. #中秋聚餐#戶外烤肉#懶人烤肉#宅家烤肉#中秋團圓色#燒肉禮盒#和牛禮盒#海陸饗宴#烤肉必備鋪#早鳥優惠#預購中#冷凍宅配";
const LONG_CONTENT = [
	"中秋烤肉，和家人一起分享當季食材。",
	...Array.from(
		{ length: 12 },
		(_, index) =>
			`第 ${index + 1} 段：準備聚餐的食材與料理筆記。#BBQ2026_FamilyDinner #中秋烤肉禮盒 #FreshFood12345_SeasonalSpecial 今日到貨，冷藏保存並依包裝說明料理。`,
	),
].join("\n");

type Args = ComponentProps<typeof Textarea>;

function ControlledPostContent(args: Args) {
	const [content, setContent] = useState(String(args.value ?? ""));
	return (
		<div
			className="seller-shell"
			style={{ display: "block", minHeight: "auto", background: "transparent" }}
		>
			<section className="post-compose__card">
				<div className="post-compose__textarea-wrap">
					<Textarea
						{...args}
						value={content}
						onChange={(event) => {
							setContent(event.target.value);
							args.onChange?.(event);
						}}
					/>
				</div>
				<p className="post-compose__field-desc">
					輸入 # 可加標籤；買家會在動態牆看到這則內容
				</p>
			</section>
		</div>
	);
}

function renderPost(
	overrides: Partial<Args> = {},
	options: {
		theme?: "light" | "dark";
		textScale?: number;
		width?: number;
	} = {},
) {
	const { width = 880, ...renderOptions } = options;
	renderWithIntl(
		<div style={{ width, maxWidth: "100%" }}>
			<ControlledPostContent
				label="貼文內容"
				floatingLabel
				variant="bordered"
				size="lg"
				minRows={6}
				maxLength={2000}
				value={SCREENSHOT_CONTENT}
				renderHighlight={highlightHashtags}
				{...overrides}
			/>
		</div>,
		renderOptions,
	);
}

const inputLocator = () =>
	page.getByRole("textbox", { name: "貼文內容", exact: true });

async function getInput() {
	await expect.element(inputLocator()).toBeVisible();
	return inputLocator().element() as HTMLTextAreaElement;
}

function postElements(textarea: HTMLTextAreaElement) {
	const wrapper = queryRequired<HTMLElement>(
		textarea.ownerDocument.body,
		'[data-slot="input-wrapper"]',
	);
	return {
		textarea,
		backdrop: queryRequired<HTMLElement>(wrapper, '[data-slot="highlight"]'),
		label: queryRequired<HTMLElement>(wrapper, '[data-slot="floating-label"]'),
	};
}

function renderedStyle(element: HTMLElement) {
	return window.getComputedStyle(element);
}

function firstGlyphBounds(backdrop: HTMLElement) {
	const walker = document.createTreeWalker(backdrop, NodeFilter.SHOW_TEXT);
	const firstText = walker.nextNode();
	expect(firstText?.textContent, "預期有可見的貼文內容").toBeTruthy();
	const range = document.createRange();
	range.setStart(firstText as Node, 0);
	range.setEnd(firstText as Node, 1);
	return range.getBoundingClientRect();
}

async function expectAligned(textarea: HTMLTextAreaElement) {
	const { backdrop, label } = postElements(textarea);
	await vi.waitFor(() => {
		const inputBounds = textarea.getBoundingClientRect();
		const backdropBounds = backdrop.getBoundingClientRect();
		const inputStyle = renderedStyle(textarea);
		const backdropStyle = renderedStyle(backdrop);
		expect(Math.abs(inputBounds.top - backdropBounds.top)).toBeLessThanOrEqual(
			1,
		);
		expect(
			Math.abs(inputBounds.left - backdropBounds.left),
		).toBeLessThanOrEqual(1);
		expect(
			Math.abs(backdropBounds.width - textarea.clientWidth),
		).toBeLessThanOrEqual(1);
		expect(
			Math.abs(backdropBounds.height - textarea.clientHeight),
		).toBeLessThanOrEqual(1);
		for (const property of [
			"fontFamily",
			"fontSize",
			"fontWeight",
			"lineHeight",
			"letterSpacing",
			"whiteSpace",
			"overflowWrap",
			"paddingTop",
			"paddingRight",
			"paddingBottom",
			"paddingLeft",
		] as const) {
			expect(backdropStyle[property], property).toBe(inputStyle[property]);
		}
		if (textarea.value && textarea.scrollTop === 0) {
			expect(firstGlyphBounds(backdrop).top).toBeGreaterThanOrEqual(
				label.getBoundingClientRect().bottom,
			);
		}
		const tokens = backdrop.querySelectorAll<HTMLElement>("span");
		// 內容含標籤時高亮 span 必須存在，不能讓逐一比對對零個元素通過。
		if (textarea.value.includes("#")) expect(tokens.length).toBeGreaterThan(0);
		for (const token of tokens) {
			expect(renderedStyle(token).fontWeight).toBe(inputStyle.fontWeight);
		}
		// 捲動範圍相等，才能偵測實際換行的差異（含長英數標籤的粗體字形偏移）。
		expect(
			Math.abs(backdrop.scrollHeight - textarea.scrollHeight),
		).toBeLessThanOrEqual(1);
	});
}

describe("Post content", () => {
	it("aligns the highlight with the screenshot content", async () => {
		renderPost();
		const input = await getInput();
		await expect.element(inputLocator()).toHaveValue(SCREENSHOT_CONTENT);
		await expectAligned(input);
	});

	it("keeps the flat floating label aligned across focus and blur", async () => {
		renderPost({ variant: "flat" });
		const input = await getInput();
		await expectAligned(input);
		await userEvent.click(inputLocator());
		await expectAligned(input);
		await userEvent.tab();
		await expect.element(inputLocator()).not.toHaveFocus();
		await expectAligned(input);
	});

	it("stays aligned while typing and replacing a selection in an empty field", async () => {
		renderPost({ value: "" });
		const input = await getInput();
		await expect.element(inputLocator()).toHaveValue("");
		await userEvent.click(inputLocator());
		await userEvent.keyboard("中秋烤肉\n#BBQ2026 #中秋節");
		await expect
			.element(inputLocator())
			.toHaveValue("中秋烤肉\n#BBQ2026 #中秋節");
		await expectAligned(input);
		input.setSelectionRange(0, 4);
		expect(input.value.slice(input.selectionStart, input.selectionEnd)).toBe(
			"中秋烤肉",
		);
		await userEvent.keyboard("團圓晚餐");
		await expect
			.element(inputLocator())
			.toHaveValue("團圓晚餐\n#BBQ2026 #中秋節");
		await expectAligned(input);
	});

	it("stays aligned for long hashtags that wrap and scroll", async () => {
		renderPost({ value: LONG_CONTENT });
		const input = await getInput();
		await expectAligned(input);
		const { backdrop } = postElements(input);
		expect(input.scrollHeight).toBeGreaterThan(input.clientHeight);
		for (const target of [60, input.scrollHeight]) {
			input.scrollTop = target;
			input.dispatchEvent(new Event("scroll", { bubbles: true }));
			await vi.waitFor(() => {
				expect(backdrop.scrollTop).toBe(input.scrollTop);
			});
			await expectAligned(input);
		}
		input.scrollTop = 0;
		input.dispatchEvent(new Event("scroll", { bubbles: true }));
		await expectAligned(input);
		await expect.element(inputLocator()).toHaveValue(LONG_CONTENT);
	});

	it("stays aligned in the dark theme", async () => {
		renderPost({}, { theme: "dark" });
		const input = await getInput();
		expect(document.documentElement.dataset.theme).toBe("dark");
		await expectAligned(input);
	});

	it("stays aligned at 320px width and 1.4x text", async () => {
		renderPost({ value: LONG_CONTENT }, { textScale: 1.4, width: 320 });
		const input = await getInput();
		await vi.waitFor(() => {
			expect(
				Number.parseFloat(renderedStyle(input).fontSize),
			).toBeGreaterThanOrEqual(22);
			expect(input.getBoundingClientRect().width).toBeLessThan(320);
		});
		await expectAligned(input);
	});
});
