import type { Appearance } from "@stripe/stripe-js";

/**
 * Payment Element 的外觀。Stripe 的 iframe 讀不到頁面的 CSS 變數，值必須是字面色：
 * 取自 `styles/foundation/00-color-config.css` 的品牌 token（brand-primary、brand-ink）。
 */
export const PAYMENT_ELEMENT_APPEARANCE: Appearance = {
	theme: "stripe",
	variables: {
		colorPrimary: "#e4572e",
		colorText: "#1f2933",
		borderRadius: "12px",
	},
};
