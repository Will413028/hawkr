import * as React from "react";
import type { LegalBlock, LegalListItem } from "./legalDocument";

const CELL =
	"border border-[color:var(--hkui-divider)] p-[var(--space-2)_var(--space-3)] text-left align-top";

function Inline({ text }: { text: string }) {
	return text.split(/(\*\*[^*]+\*\*)/).map((part, index) =>
		part.startsWith("**") && part.endsWith("**") ? (
			// biome-ignore lint/suspicious/noArrayIndexKey: 切分結果是固定順序的靜態文字
			<strong key={index}>{part.slice(2, -2)}</strong>
		) : (
			// biome-ignore lint/suspicious/noArrayIndexKey: 切分結果是固定順序的靜態文字
			<React.Fragment key={index}>{part}</React.Fragment>
		),
	);
}

function ListItem({ item }: { item: LegalListItem }) {
	if (typeof item === "string") {
		return (
			<li>
				<Inline text={item} />
			</li>
		);
	}
	return (
		<li>
			<Inline text={item.text} />
			<ul className="mt-[var(--space-2)]">
				{item.sub.map((sub) => (
					<li key={sub}>
						<Inline text={sub} />
					</li>
				))}
			</ul>
		</li>
	);
}

function Block({ block }: { block: LegalBlock }) {
	switch (block.type) {
		case "h2":
			return null;
		case "h3":
			return (
				<h4 className="mt-[var(--space-4)] mb-[var(--space-2)] font-bold text-[color:var(--text-heading)]">
					{block.text}
				</h4>
			);
		case "p":
			return (
				<p>
					{block.lines.map((line, index) => (
						<React.Fragment key={line}>
							{index > 0 && <br />}
							<Inline text={line} />
						</React.Fragment>
					))}
				</p>
			);
		case "ol":
			return (
				<ol className="mb-[var(--space-4)] list-decimal pl-[var(--space-6)] [&_li]:mb-[var(--space-2)]">
					{block.items.map((item) => (
						<ListItem
							key={typeof item === "string" ? item : item.text}
							item={item}
						/>
					))}
				</ol>
			);
		case "ul":
		case "checklist":
			return (
				<ul>
					{block.items.map((item) => (
						<li key={item}>
							<Inline text={item} />
						</li>
					))}
				</ul>
			);
		case "table":
			return (
				<div className="mb-[var(--space-4)] overflow-x-auto">
					<table className="w-full border-collapse text-[length:var(--text-sm)]">
						<thead>
							<tr>
								{block.header.map((cell) => (
									<th key={cell} className={`${CELL} font-bold`}>
										{cell}
									</th>
								))}
							</tr>
						</thead>
						<tbody>
							{block.rows.map((row) => (
								<tr key={row.join("|")}>
									{row.map((cell) => (
										<td key={cell} className={CELL}>
											{cell}
										</td>
									))}
								</tr>
							))}
						</tbody>
					</table>
				</div>
			);
	}
}

/** 渲染投影的區塊；h2 是分節的邊界，由頁面切成 LegalPage 的 section，這裡不畫。 */
export function LegalBlocks({ blocks }: { blocks: readonly LegalBlock[] }) {
	return (
		<>
			{blocks.map((block, index) => (
				// biome-ignore lint/suspicious/noArrayIndexKey: 投影是固定順序的靜態資料
				<Block key={index} block={block} />
			))}
		</>
	);
}
