"use client";

/**
 * S12｜銷項／進項兩個分頁的共用 tab 列（58A §S12）。
 *
 * 🔴 **不新開路由**：同一個心智概念（「我的發票」）分兩個資料集，用 tab 比兩條路由
 * 好找；`?tab=issued`（預設）／`?tab=received`。
 *
 * 🔴 **為什麼是分頁而不是一個列表加篩選**：兩者是**不同的法律角色**——銷項要對開立
 * 字軌與作廢期限、可作廢可折讓；進項是報稅憑證、唯讀、匯出格式不同。混在一個列表
 * 裡，「作廢」這種只對銷項合法的操作會難以分流，而使用者一眼也分不出哪些是自己開的。
 */

import { ScFilters, ScPill } from "@/components/shared/ScPrimitives";

export const INVOICE_DOCUMENTS_TABS = [
	{ value: "issued", label: "銷項（我開立的）" },
	{ value: "received", label: "進項（平台開給我的）" },
] as const;

export type InvoiceDocumentsTab =
	(typeof INVOICE_DOCUMENTS_TABS)[number]["value"];

/** URL 的 `?tab=` 只接受兩個值，其餘一律退回銷項。 */
export function normalizeInvoiceDocumentsTab(raw: string): InvoiceDocumentsTab {
	return raw === "received" ? "received" : "issued";
}

export function InvoiceDocumentsTabs({
	tab,
	onChange,
}: {
	tab: InvoiceDocumentsTab;
	onChange: (next: InvoiceDocumentsTab) => void;
}) {
	return (
		<ScFilters aria-label="發票方向">
			{INVOICE_DOCUMENTS_TABS.map((item) => (
				<ScPill
					key={item.value}
					as="button"
					active={tab === item.value}
					aria-selected={tab === item.value}
					onClick={() => onChange(item.value)}
				>
					{item.label}
				</ScPill>
			))}
		</ScFilters>
	);
}
