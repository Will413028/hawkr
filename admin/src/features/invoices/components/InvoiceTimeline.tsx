import type React from "react";

export interface InvoiceTimelineEntry {
	/** React key。用後端的 id（noteId／auditLogId／responseId），沒有才退回索引。 */
	id: string;
	/** ISO 8601 UTC，由呼叫端先過 `formatDateTime()`。 */
	time: string;
	/** 這一列發生了什麼（一行，已是可讀文字或碼值）。 */
	title: React.ReactNode;
	/** 誰做的。留空則不渲染那一行。 */
	actor?: React.ReactNode;
	/** 補充內容（理由、備註、結果）。 */
	body?: React.ReactNode;
}

/**
 * 發票模組的共用時間軸（A03 審核備註／A06 操作稽核／A08 往來紀錄）。
 *
 * 為什麼元件化：既有的時間軸（`disputes/[id]/page.tsx` 的往來紀錄）是整段 inline style
 * 手刻的，而這個形狀在發票模組有三頁要用。複製三份的話，三份會各自漂。
 *
 * ⚠️ **`<ol>` 不是 `<ul>`**：這些紀錄有時間順序，而且「追加事件不覆蓋」是稽核的前提
 * ——UI 不提供編輯任何一筆。
 *
 * ⚠️ 顏色全部走 class（`Invoice.css` 內的 `--admin-*` token），這裡不寫任何 inline
 * style：色值棘輪只掃 `.css`，tsx 的 inline style 裸 hex 沒有任何機械保證（§0.6 C06）。
 */
export default function InvoiceTimeline({
	entries,
	emptyText,
}: {
	entries: InvoiceTimelineEntry[];
	emptyText: string;
}) {
	if (entries.length === 0) {
		return <p className="invoice-timeline__empty">{emptyText}</p>;
	}
	return (
		<ol className="invoice-timeline">
			{entries.map((entry) => (
				<li key={entry.id} className="invoice-timeline__item">
					<div className="invoice-timeline__time">{entry.time}</div>
					<div className="invoice-timeline__body">
						<div className="invoice-timeline__title">{entry.title}</div>
						{entry.actor ? (
							<div className="invoice-timeline__actor">{entry.actor}</div>
						) : null}
						{entry.body ? (
							<div className="invoice-timeline__note">{entry.body}</div>
						) : null}
					</div>
				</li>
			))}
		</ol>
	);
}
