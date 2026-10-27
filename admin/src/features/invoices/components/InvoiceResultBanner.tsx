"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/** 頁面級結果橫幅的三個語氣。 */
export type InvoiceBannerTone = "success" | "warning" | "danger";

export interface InvoiceBanner {
	tone: InvoiceBannerTone;
	text: string;
}

/**
 * A03／A06／A08／A13 寫入動作的頁面級結果橫幅。
 *
 * 三個 tone 各自沿用既有的 class，**不新增色值**（`tests/admin-color-ratchet.test.ts`
 * 會擋新 hex，而這三個 tone 本來就都有既有樣式）。
 *
 * `danger` 用 `role="alert"`（要打斷），`success`／`warning` 用 `role="status"`
 * ＋ `aria-live="polite"`（不打斷，但動作完成時要說話）。
 *
 * ⚠️ 三個 class 全部住在 `Invoice.css`，**不借 `DelegatedOperations.css` 的
 * `.adm-authorization-notice`**：借來的 class 在哪一天這幾頁不再 import 那張表時會變成
 * 零樣式，而**不會有任何東西轉紅**（58A 陷阱 C07）。
 *
 * 與 A04 頁內的 `ResultBanner` 是同一份東西——那一支先寫、形狀相同；這裡抽成共用是
 * 為了不讓後面四頁各自複製一份（四份各自漂移的 role/aria 是純粹的負債）。
 */
export default function InvoiceResultBanner({
	banner,
}: {
	banner: InvoiceBanner | null;
}) {
	if (!banner) return null;
	if (banner.tone === "danger") {
		return (
			<div className="invoice-danger-notice" role="alert">
				<MaterialSymbol name="error" />
				<div>{banner.text}</div>
			</div>
		);
	}
	if (banner.tone === "warning") {
		return (
			<div className="invoice-page-notice" role="status" aria-live="polite">
				<MaterialSymbol name="info" />
				<div>{banner.text}</div>
			</div>
		);
	}
	return (
		<div className="invoice-success-notice" role="status" aria-live="polite">
			<MaterialSymbol name="check_circle" />
			<div>{banner.text}</div>
		</div>
	);
}
