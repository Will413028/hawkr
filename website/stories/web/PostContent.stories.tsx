import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { expect, userEvent, waitFor } from "storybook/test";
import { Textarea } from "@/components/ui/Input";
import { highlightHashtags } from "@/features/seller-center/posts/highlightHashtags";
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

function ControlledPostContent(args: ComponentProps<typeof Textarea>) {
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

const meta = {
	title: "Web/Forms/Post Content",
	component: Textarea,
	args: {
		label: "貼文內容",
		floatingLabel: true,
		variant: "bordered",
		size: "lg",
		minRows: 6,
		maxLength: 2000,
		value: SCREENSHOT_CONTENT,
		renderHighlight: highlightHashtags,
	},
	render: ControlledPostContent,
	decorators: [
		(Story) => (
			<div style={{ width: 880, maxWidth: "100%" }}>
				<Story />
			</div>
		),
	],
	parameters: {
		docs: {
			description: {
				component:
					"使用正式貼文的受控 Textarea、hashtag renderer 與賣家樣式。檢查標籤、高亮文字、游標及捲動的對齊；所有內容只保留於本機預覽。",
			},
		},
	},
} satisfies Meta<typeof Textarea>;
export default meta;
type Story = StoryObj<typeof meta>;

function postElements(element: HTMLElement) {
	const textarea = element as HTMLTextAreaElement;
	const wrapper = textarea.closest<HTMLElement>('[data-slot="input-wrapper"]');
	const backdrop = wrapper?.querySelector<HTMLElement>(
		'[data-slot="highlight"]',
	);
	const label = wrapper?.querySelector<HTMLElement>(
		'[data-slot="floating-label"]',
	);
	if (!wrapper || !backdrop || !label) {
		throw new Error("Post content must expose its input, highlight and label");
	}
	return { textarea, backdrop, label };
}

function renderedStyle(element: HTMLElement) {
	const view = element.ownerDocument.defaultView;
	if (!view) throw new Error("Post content preview requires a browser window");
	return view.getComputedStyle(element);
}

function firstGlyphBounds(backdrop: HTMLElement) {
	const document = backdrop.ownerDocument;
	const walker = document.createTreeWalker(backdrop, 4);
	const firstText = walker.nextNode();
	if (!firstText?.textContent) throw new Error("Expected visible post content");
	const range = document.createRange();
	range.setStart(firstText, 0);
	range.setEnd(firstText, 1);
	return range.getBoundingClientRect();
}

async function expectAligned(element: HTMLElement) {
	const { textarea, backdrop, label } = postElements(element);
	await waitFor(() => {
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
		for (const token of backdrop.querySelectorAll<HTMLElement>("span")) {
			expect(renderedStyle(token).fontWeight).toBe(inputStyle.fontWeight);
		}
		// Equal scroll extents detect differences in actual wrapping, including
		// long Latin/numeric hashtags whose bold glyphs used to drift.
		expect(
			Math.abs(backdrop.scrollHeight - textarea.scrollHeight),
		).toBeLessThanOrEqual(1);
	});
}

export const ScreenshotContent: Story = {
	name: "貼文內容 · 截圖重現",
	globals: { theme: "light", textScale: "1" },
	play: async ({ canvas }) => {
		const input = canvas.getByRole("textbox", { name: "貼文內容" });
		await expect(input).toHaveValue(SCREENSHOT_CONTENT);
		await expectAligned(input);
	},
};

export const LegacyFloatingLabel: Story = {
	name: "原浮動標籤 · 高亮對齊",
	args: { variant: "flat" },
	globals: { theme: "light", textScale: "1" },
	play: async ({ canvas }) => {
		const input = canvas.getByRole("textbox", { name: "貼文內容" });
		await expectAligned(input);
		await userEvent.click(input);
		await expectAligned(input);
		await userEvent.tab();
		await expect(input).not.toHaveFocus();
		await expectAligned(input);
	},
};

export const EmptyThenTyping: Story = {
	name: "空白欄位 · 輸入與選取",
	args: { value: "" },
	play: async ({ canvas }) => {
		const input = canvas.getByRole("textbox", { name: "貼文內容" });
		await expect(input).toHaveValue("");
		await userEvent.click(input);
		await userEvent.keyboard("中秋烤肉\n#BBQ2026 #中秋節");
		await expect(input).toHaveValue("中秋烤肉\n#BBQ2026 #中秋節");
		await expectAligned(input);
		const { textarea } = postElements(input);
		textarea.setSelectionRange(0, 4);
		await expect(
			textarea.value.slice(textarea.selectionStart, textarea.selectionEnd),
		).toBe("中秋烤肉");
		await userEvent.keyboard("團圓晚餐");
		await expect(input).toHaveValue("團圓晚餐\n#BBQ2026 #中秋節");
		await expectAligned(input);
	},
};

export const WrappedAndScrolled: Story = {
	name: "長標籤 · 換行與捲動",
	args: { value: LONG_CONTENT },
	play: async ({ canvas }) => {
		const input = canvas.getByRole("textbox", { name: "貼文內容" });
		await expectAligned(input);
		const { textarea, backdrop } = postElements(input);
		await expect(textarea.scrollHeight).toBeGreaterThan(textarea.clientHeight);
		for (const target of [60, textarea.scrollHeight]) {
			textarea.scrollTop = target;
			textarea.dispatchEvent(new Event("scroll", { bubbles: true }));
			await waitFor(() => {
				expect(backdrop.scrollTop).toBe(textarea.scrollTop);
			});
			await expectAligned(input);
		}
		textarea.scrollTop = 0;
		textarea.dispatchEvent(new Event("scroll", { bubbles: true }));
		await expectAligned(input);
		await expect(input).toHaveValue(LONG_CONTENT);
	},
};

export const DarkContent: Story = {
	name: "深色貼文",
	globals: { theme: "dark", textScale: "1" },
	play: async ({ canvas, canvasElement }) => {
		await expect(canvasElement.ownerDocument.documentElement).toHaveAttribute(
			"data-theme",
			"dark",
		);
		await expectAligned(canvas.getByRole("textbox", { name: "貼文內容" }));
	},
};

export const NarrowLargeText: Story = {
	name: "窄螢幕 · 1.4 倍文字",
	args: { value: LONG_CONTENT },
	globals: { theme: "light", textScale: "1.4" },
	decorators: [
		(Story) => (
			<div style={{ width: 320, maxWidth: "100%" }}>
				<Story />
			</div>
		),
	],
	play: async ({ canvas }) => {
		const input = canvas.getByRole("textbox", { name: "貼文內容" });
		await waitFor(() => {
			expect(
				Number.parseFloat(renderedStyle(input).fontSize),
			).toBeGreaterThanOrEqual(22);
			expect(input.getBoundingClientRect().width).toBeLessThan(320);
		});
		await expectAligned(input);
	},
};
