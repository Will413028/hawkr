import { Chip } from "@/components/ui";
import { INVOICE_FULFILLMENT_MODE_LABEL } from "@/lib/codemeta/generated";

interface InvoiceModeBadgeProps {
	/** `InvoiceFulfillmentMode` 的值（`invoiceMode`）。 */
	mode: string;
	className?: string;
}

/**
 * 「這張發票由誰、以什麼方式開立」的標籤（58A §B04 標題列）。
 *
 * 中文一律取自 CodeMeta（§0.4 ⑤）。認不得的值**不渲染**而不是把 enum 原字串印
 * 給買家看：這顆 badge 是輔助脈絡（同一件事在狀態橫幅與文件卡上都說得完整），
 * 少一顆不會讓人看不懂，印出 `AMEGO_AUTO` 才會。
 */
export function InvoiceModeBadge({ mode, className }: InvoiceModeBadgeProps) {
	const label = INVOICE_FULFILLMENT_MODE_LABEL[mode];
	if (!label) return null;
	return (
		<Chip variant="flat" color="default" size="sm" className={className}>
			{label}
		</Chip>
	);
}
