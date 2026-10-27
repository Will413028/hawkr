import { INVOICE_DOCUMENT_TIMELINE_EVENT_LABEL } from "@/lib/codemeta/generated";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import type { InvoiceTimelineItem } from "@/lib/invoices/types";

/**
 * 事件名一律走 CodeMeta `InvoiceDocumentTimelineEvent`（5 值）。
 *
 * 🔴 **不是 `InvoiceEventType`**——那是 MQ 事件型別，兩者值域交集為空，查錯表
 * 只會整欄空白而**不報錯**。認不得的值退回原字串，至少看得出發生過一件事。
 */
function eventLabel(event: string): string {
	return INVOICE_DOCUMENT_TIMELINE_EVENT_LABEL[event] ?? event;
}

interface DocumentTimelineProps {
	items: InvoiceTimelineItem[];
}

/**
 * N2｜文件歷程（58A §B05）。
 *
 * 是**事件列表不是 stepper**：既有兩支時間軸都是固定步數（`OrderStatusTimeline`
 * 5 步、`AfterSaleProgress` 4 步），而發票歷程不定長度、可重複（作廢重開會再來
 * 一輪）、有分支（折讓）。視覺沿用 `AfterSaleArbitrationCard` 的 `<ol>`。
 */
export function DocumentTimeline({ items }: DocumentTimelineProps) {
	return (
		<ol className="list-none m-0 p-0 flex flex-col gap-3">
			{items.map((item) => (
				<li
					key={`${item.occurredAt}-${item.event}`}
					className="grid grid-cols-[16px_minmax(0,1fr)] gap-3"
				>
					<span
						className="mt-1.5 size-2 rounded-full bg-hk-primary"
						aria-hidden="true"
					/>
					<div className="flex flex-col gap-0.5">
						<strong className="text-sm font-bold text-foreground">
							{eventLabel(item.event)}
						</strong>
						<time
							className="text-xs text-foreground-soft tabular-nums"
							dateTime={item.occurredAt}
						>
							{formatTaipeiDateTime(item.occurredAt) || item.occurredAt}
						</time>
						{item.note ? (
							<p className="m-0 text-xs text-foreground-soft">{item.note}</p>
						) : null}
					</div>
				</li>
			))}
		</ol>
	);
}
