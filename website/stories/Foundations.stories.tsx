import type { Meta, StoryObj } from "@storybook/nextjs-vite";

const palette = [
	"primary",
	"primary-strong",
	"primary-deep",
	"secondary",
	"ink",
	"secondary-soft",
];
/** @import import { Foundations } from "@/stories/Foundations.stories"; */
export function Foundations() {
	return (
		<main className="catalog-stack">
			<h1>Hawkr · 視覺基礎</h1>
			<p>
				三端共用既有品牌色、4px 間距節奏、基礎圓角與 200ms
				標準動效。實際值由正式樣式表讀取。
			</p>
			<div className="catalog-grid">
				{palette.map((color) => (
					<div className="catalog-swatch" key={color}>
						<div style={{ background: `var(--color-brand-${color})` }} />
						<p>
							{color}
							<br />
							<code>--color-brand-{color}</code>
						</p>
					</div>
				))}
			</div>
			<h2>字體與層級</h2>
			<p>
				內文與控制項使用正式網站字體。品牌標題沿用 jf-openhuninn；App
				使用各自正式 ThemeData。
			</p>
			<div style={{ display: "grid", gap: 16 }}>
				{[12, 14, 16, 18, 20, 24].map((size) => (
					<span style={{ fontSize: `${size / 16}rem` }} key={size}>
						{size} · 新鮮，是日常的好選擇 Fresh daily
					</span>
				))}
			</div>
			<h2>間距與圓角</h2>
			<div className="catalog-row">
				{[8, 12, 16, 24, 32].map((radius) => (
					<div
						key={radius}
						style={{
							borderRadius: radius,
							border: "1px solid var(--border-soft)",
							padding: 24,
							background: "var(--surface-card)",
						}}
					>
						{radius}px
					</div>
				))}
			</div>
			<p className="catalog-note">
				Web md 按鈕 44px；主要 CTA 用 lg 48px 對齊
				App。密度、字體平台差異與尚待統一項目詳見元件庫文件。
			</p>
		</main>
	);
}
const meta = {
	title: "Foundations/Brand",
	excludeStories: ["Foundations"],
	component: Foundations,
	parameters: { layout: "padded" },
} satisfies Meta<typeof Foundations>;
export default meta;
export const Overview: StoryObj<typeof meta> = {};
