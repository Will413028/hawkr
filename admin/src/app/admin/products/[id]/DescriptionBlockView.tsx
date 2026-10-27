"use client";

import Image from "next/image";
import type React from "react";
import { parseListJson, parseTableJson } from "@/lib/product-description";
import type { DescriptionBlock, InlineRun } from "@/types/product";

const HEX6 = /^#[0-9a-fA-F]{6}$/;
const HTTP_URL = /^https?:\/\//i;

function renderRuns(
	runs: InlineRun[] | undefined,
	keyBase: string,
): React.ReactNode {
	if (!runs || runs.length === 0) return null;
	return runs.map((run, i) => {
		if (!run.text) return null;
		const style: React.CSSProperties = {};
		if (run.bold) style.fontWeight = 700;
		if (run.italic) style.fontStyle = "italic";
		if (run.underline) style.textDecoration = "underline";
		if (run.color && HEX6.test(run.color)) style.color = run.color;
		const key = `${keyBase}-r${i}`;
		if (run.href && HTTP_URL.test(run.href)) {
			return (
				<a
					key={key}
					href={run.href}
					target="_blank"
					rel="noopener noreferrer nofollow"
					style={{ ...style, textDecoration: "underline" }}
				>
					{run.text}
				</a>
			);
		}
		return (
			<span key={key} style={style}>
				{run.text}
			</span>
		);
	});
}

function TextBlock({ block }: { block: DescriptionBlock }) {
	if (block.runs && block.runs.length > 0) {
		return <p>{renderRuns(block.runs, block.id || "text")}</p>;
	}
	return <p>{block.content || ""}</p>;
}

function TableBlock({ block }: { block: DescriptionBlock }) {
	const model = parseTableJson(block.tableJson);
	if (!model) return null;
	const keyBase = block.id || "table";
	return (
		<div style={{ overflowX: "auto" }}>
			<table className="data-table">
				<tbody>
					{model.rows.map((row, ri) => {
						const isHead = model.withHeadings && ri === 0;
						return (
							// biome-ignore lint/suspicious/noArrayIndexKey: table rows are positional, no stable id from backend
							<tr key={`${keyBase}-row${ri}`}>
								{row.map((cell, ci) => {
									const cellKey = `${keyBase}-c${ri}-${ci}`;
									const style: React.CSSProperties = cell.textAlign
										? { textAlign: cell.textAlign }
										: {};
									return isHead ? (
										<th key={cellKey} style={style}>
											{renderRuns(cell.runs, cellKey)}
										</th>
									) : (
										<td key={cellKey} style={style}>
											{renderRuns(cell.runs, cellKey)}
										</td>
									);
								})}
							</tr>
						);
					})}
				</tbody>
			</table>
		</div>
	);
}

function ListBlock({ block }: { block: DescriptionBlock }) {
	const model = parseListJson(block.listJson);
	if (!model) return null;
	const keyBase = block.id || "list";
	const items = model.items.map((item, i) => (
		// biome-ignore lint/suspicious/noArrayIndexKey: list items are positional, no stable id from backend
		<li key={`${keyBase}-li${i}`}>
			{renderRuns(item.runs, `${keyBase}-li${i}`)}
		</li>
	));
	return model.ordered ? (
		<ol className="description-list">{items}</ol>
	) : (
		<ul className="description-list">{items}</ul>
	);
}

function ImageBlock({ block }: { block: DescriptionBlock }) {
	if (!block.imageUrl) return null;
	const width = Number(block.width) || 800;
	const height = Number(block.height) || 600;
	return (
		<Image
			src={block.imageUrl}
			alt={block.alt || ""}
			width={width}
			height={height}
			unoptimized
			style={{ maxWidth: "100%", height: "auto", borderRadius: 10 }}
		/>
	);
}

export default function DescriptionBlockView({
	block,
}: {
	block: DescriptionBlock;
}) {
	return (
		<div className="description-block">
			{block.type === "table" ? (
				<TableBlock block={block} />
			) : block.type === "list" ? (
				<ListBlock block={block} />
			) : block.type === "image" ? (
				<ImageBlock block={block} />
			) : (
				<TextBlock block={block} />
			)}
		</div>
	);
}
