"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Prospect } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { type CSSProperties, type ReactNode, useState } from "react";
import { formatDateTimeShort } from "@/lib/format";
import {
	contactKindText,
	outreachStatusText,
	prospectStatusText,
	vendorTypeText,
} from "@/lib/seller-prospects";

/**
 * `MergeDialog.tsx` 的逐欄比對表（疑似重複頁與修改時的合併出口共用），欄位讀 `Prospect`。
 *
 * 對話框（overlay、a11y）由 `MergeDialog.tsx` 經共用 `<Dialog>` 畫；本檔只是對話框內容，沒有 overlay，也不該自己畫。
 */

const UNFILLED = "未填寫";

function timeText(time: Prospect["createTime"]): string {
	return time ? formatDateTimeShort(timestampDate(time)) : "-";
}

/** 聯絡方式合成一列：每筆「{類型}：{值}」，主要的加「（主要）」。 */
function contactsCell(prospect: Prospect): ReactNode {
	if (prospect.contacts.length === 0) return UNFILLED;
	return prospect.contacts.map((c) => (
		<div key={`${c.kind}:${c.value}`}>
			{`${contactKindText(c.kind)}：${c.value}${c.primary ? "（主要）" : ""}`}
		</div>
	));
}

function imagesText(prospect: Prospect): string {
	return `logo ${prospect.logoImage ? "有" : "沒有"}、封面${prospect.coverImage ? "有" : "沒有"}`;
}

interface FieldRow {
	key: string;
	label: string;
	render: (p: Prospect) => ReactNode;
	/**
	 * 自由文字欄位可能很長（後端是 textarea），預設截斷成固定行數＋可展開，不讓它撐破逐欄比對表的版面。
	 */
	truncatable?: boolean;
}

/**
 * 逐欄比對的欄位清單：只挑對「這兩筆是不是同一家、留哪一筆」決策有意義的欄位，
 * 也包含預設勝方的完整度用到的欄位（`defaultWinnerId`），管理員看得到系統為什麼選了這一邊。
 */
const FIELD_ROWS: FieldRow[] = [
	{
		key: "displayName",
		label: "業者名稱",
		render: (p) => p.displayName || "（未命名）",
	},
	{ key: "taxId", label: "統一編號", render: (p) => p.taxId || UNFILLED },
	{ key: "contacts", label: "聯絡方式", render: contactsCell },
	{ key: "category", label: "分類", render: (p) => p.category || UNFILLED },
	{ key: "region", label: "地區", render: (p) => p.region || UNFILLED },
	{
		key: "productSummary",
		label: "商品摘要",
		render: (p) => p.productSummary || UNFILLED,
		truncatable: true,
	},
	{
		key: "positioning",
		label: "市場定位",
		render: (p) => p.positioning || UNFILLED,
		truncatable: true,
	},
	{
		key: "researchSummary",
		label: "調研備註",
		render: (p) => p.researchSummary || UNFILLED,
		truncatable: true,
	},
	{
		key: "priorityLetter",
		label: "業務優先順位（字母）",
		render: (p) => p.priorityLetter || UNFILLED,
	},
	{
		key: "priorityStars",
		label: "業務優先星等",
		render: (p) => (p.priorityStars ? String(p.priorityStars) : UNFILLED),
	},
	{
		key: "vendorType",
		label: "業者型態",
		render: (p) => vendorTypeText(p.vendorType),
	},
	{ key: "images", label: "logo／封面", render: imagesText },
	{
		key: "status",
		label: "狀態",
		render: (p) => prospectStatusText(p.status),
	},
	{
		key: "outreachStatus",
		label: "招商狀態",
		render: (p) => outreachStatusText(p.outreachStatus),
	},
	{
		key: "createdAt",
		label: "建立時間",
		render: (p) => timeText(p.createTime),
	},
	{
		key: "updatedAt",
		label: "更新時間",
		render: (p) => timeText(p.updateTime),
	},
];

/**
 * 截斷到 3 行；repo 其餘頁面沒有現成的多行截斷 class 可重用，這裡用行內樣式，
 * 不新增 CSS 檔也不新增 hex（`-webkit-line-clamp` 全瀏覽器含行動版都吃）。
 *
 * 必須套在 `<td>` 內層元素（例如包一層 `<div>`），不能直接套在 `<td>` 自己身上：`display: "-webkit-box"` 會讓該節點不再是
 * `display: table-cell`，依 CSS 2.1 §17.2.1 匿名表格物件生成規則，連續的非 table-cell 兄弟節點會被包進同一個匿名 cell，
 * 左右兩欄會塌成一格。
 */
const CLAMP_STYLE: CSSProperties = {
	display: "-webkit-box",
	WebkitLineClamp: 3,
	WebkitBoxOrient: "vertical",
	overflow: "hidden",
	textOverflow: "ellipsis",
};

export function ProspectCompareTable({
	left,
	right,
}: {
	left: Prospect;
	right: Prospect;
}) {
	const [expandedRows, setExpandedRows] = useState<ReadonlySet<string>>(
		new Set(),
	);

	const toggleExpand = (key: string) => {
		setExpandedRows((prev) => {
			const next = new Set(prev);
			if (next.has(key)) next.delete(key);
			else next.add(key);
			return next;
		});
	};

	return (
		// `.table-container` 的 `overflow: hidden`（globals.css）只裁切、不提供橫向捲軸；
		// 行內覆寫 `overflow-x: auto` 才是真的讓窄螢幕能在自身容器捲動。
		<div className="table-container" style={{ overflowX: "auto" }}>
			<table className="data-table">
				<thead>
					<tr>
						<th>欄位</th>
						<th>{left.displayName || "（未命名）"}</th>
						<th>{right.displayName || "（未命名）"}</th>
					</tr>
				</thead>
				<tbody>
					{FIELD_ROWS.map((row) => {
						const expanded = expandedRows.has(row.key);
						const clipped = row.truncatable && !expanded;
						return (
							<tr key={row.key}>
								<td>
									{row.label}
									{row.truncatable ? (
										<button
											type="button"
											className="btn-reset"
											onClick={() => toggleExpand(row.key)}
											style={{
												display: "block",
												marginTop: 4,
												fontSize: 12,
												color: "var(--hk-primary-strong)",
												textDecoration: "underline",
											}}
										>
											{expanded ? "收合" : "展開全文"}
										</button>
									) : null}
								</td>
								<td>
									<div style={clipped ? CLAMP_STYLE : undefined}>
										{row.render(left)}
									</div>
								</td>
								<td>
									<div style={clipped ? CLAMP_STYLE : undefined}>
										{row.render(right)}
									</div>
								</td>
							</tr>
						);
					})}
				</tbody>
			</table>
		</div>
	);
}
