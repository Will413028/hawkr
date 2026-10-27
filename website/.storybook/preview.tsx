import type { Preview } from "@storybook/nextjs-vite";
import { NextIntlClientProvider } from "next-intl";
import { useEffect } from "react";
import { sb } from "storybook/test";
import { defaultLocale, messagesFor } from "../lib/intl/config";
import "../styles/globals.css";
import "../styles/components/product-certification.css";
import "../styles/components/product-modal.css";
import "../styles/components/ui-kit.css";
import "../styles/components/input-field-fx.css";
import "./preview.css";

const preview: Preview = {
	tags: ["autodocs", "ai-generated"],
	globalTypes: {
		theme: {
			description: "Web 色彩模式（App 預覽使用 App 自身主題）",
			toolbar: {
				icon: "paintbrush",
				items: [
					{ value: "light", title: "淺色" },
					{ value: "dark", title: "深色" },
				],
				dynamicTitle: true,
			},
		},
		textScale: {
			description: "Web 字級倍率；不放大間距與觸控尺寸",
			toolbar: {
				icon: "paragraph",
				items: [
					{ value: "0.875", title: "小字級" },
					{ value: "1", title: "標準字級" },
					{ value: "1.125", title: "大字級" },
					{ value: "1.4", title: "1.4 倍壓力檢查" },
				],
				dynamicTitle: true,
			},
		},
	},
	initialGlobals: { theme: "light", textScale: "1" },
	parameters: {
		nextjs: { appDirectory: true },
		layout: "padded",
		a11y: { test: "error" },
		controls: { expanded: true },
		options: {
			storySort: {
				order: [
					"Documentation",
					"開始使用",
					"Foundations",
					"Web",
					"Apps",
					"Cross Platform",
				],
			},
		},
	},
	decorators: [
		function ThemeDecorator(Story, context) {
			useEffect(() => {
				const root = document.documentElement;
				root.lang = "zh-Hant";
				root.classList.add("hk-font-rounded");
				root.dataset.theme = context.globals.theme;
				root.style.fontSize = `${Number(context.globals.textScale) * 100}%`;
				root.style.setProperty(
					"--hk-text-scale",
					String(context.globals.textScale),
				);
			}, [context.globals.theme, context.globals.textScale]);
			return <Story />;
		},
		// 路徑類的 hook（lib/intl/navigation.ts 的 usePathname）是 next-intl 的，要有語系 provider。
		function IntlDecorator(Story) {
			return (
				<NextIntlClientProvider
					locale={defaultLocale}
					messages={messagesFor(defaultLocale)}
					timeZone="Asia/Taipei"
				>
					<Story />
				</NextIntlClientProvider>
			);
		},
	],
};

export default preview;

// Seller cart stories override identity without mounting an authenticated app.
sb.mock("../lib/hooks/useQueryIdentity.ts", { spy: true });

// Navigation stories keep the real state machine and intercept document I/O.
sb.mock("../lib/navigation/document-location.ts", { spy: true });
sb.mock("../lib/navigation/recovery-state.ts", { spy: true });
sb.mock("../lib/analytics/gtag.ts", { spy: true });
