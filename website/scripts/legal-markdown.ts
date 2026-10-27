/**
 * docs/legal 正本的 Markdown 解析：只涵蓋正本用到的語法（標題、段落、編號與項目符號清單、
 * 核取清單、表格、`---` 與 HTML 註解）。
 */

import type {
	LegalBlock,
	LegalListItem,
} from "../features/legal/documents/legalDocument";

export interface ParsedLegalMarkdown {
	title: string;
	versionLine: string;
	version: string;
	blocks: LegalBlock[];
}

const VERSION_LINE = /^版本：(\d{4}-\d{2}-\d{2})（/;

type OlItem = { text: string; sub: string[] };

function tableCells(line: string): string[] {
	return line
		.replace(/^\||\|$/g, "")
		.split("|")
		.map((c) => c.trim());
}

function isSeparatorRow(cells: readonly string[]): boolean {
	return cells.every((c) => /^-+$/.test(c));
}

export function parseLegalMarkdown(markdown: string): ParsedLegalMarkdown {
	let title: string | undefined;
	const blocks: LegalBlock[] = [];
	let cur: LegalBlock | undefined;
	let olItems: OlItem[] = [];

	const closeOl = () => {
		if (cur?.type !== "ol") return;
		cur = {
			type: "ol",
			items: olItems.map(
				(item): LegalListItem => (item.sub.length > 0 ? item : item.text),
			),
		};
		blocks[blocks.length - 1] = cur;
		olItems = [];
	};
	const close = () => {
		closeOl();
		cur = undefined;
	};
	const replaceCurrent = (block: LegalBlock) => {
		cur = block;
		blocks[blocks.length - 1] = block;
	};

	for (const raw of markdown.split("\n")) {
		const line = raw.trimEnd();
		if (line === "" || line === "---" || /^<!--.*-->$/.test(line)) {
			close();
			continue;
		}
		const heading = /^(#{1,3}) (.*)$/.exec(line);
		if (heading) {
			close();
			if (heading[1].length === 1) {
				if (title !== undefined) throw new Error(`重複的標題：${line}`);
				title = heading[2];
			} else {
				blocks.push({
					type: heading[1].length === 2 ? "h2" : "h3",
					text: heading[2],
				});
			}
			continue;
		}
		if (line.startsWith("|")) {
			const cells = tableCells(line);
			if (cur?.type === "table") {
				if (!isSeparatorRow(cells)) {
					replaceCurrent({ ...cur, rows: [...cur.rows, cells] });
				}
			} else {
				close();
				cur = { type: "table", header: cells, rows: [] };
				blocks.push(cur);
			}
			continue;
		}
		const task = /^- \[ \] (.*)$/.exec(line);
		const bullet = /^- (.*)$/.exec(line);
		if (task || bullet) {
			const kind = task ? "checklist" : "ul";
			const text = (task ?? bullet)?.[1] ?? "";
			if (cur?.type === kind) {
				replaceCurrent({ type: kind, items: [...cur.items, text] });
			} else {
				close();
				cur = { type: kind, items: [text] };
				blocks.push(cur);
			}
			continue;
		}
		const numbered = /^(\d+)\. (.*)$/.exec(line);
		if (numbered) {
			if (cur?.type !== "ol") {
				close();
				cur = { type: "ol", items: [] };
				blocks.push(cur);
			}
			if (Number(numbered[1]) !== olItems.length + 1) {
				throw new Error(`編號不連續：${line}`);
			}
			olItems.push({ text: numbered[2], sub: [] });
			continue;
		}
		const sub = /^\s+- (.*)$/.exec(line);
		if (sub) {
			if (cur?.type !== "ol") throw new Error(`子項目沒有上層編號：${line}`);
			olItems[olItems.length - 1].sub.push(sub[1]);
			continue;
		}
		if (cur?.type === "p") {
			replaceCurrent({ type: "p", lines: [...cur.lines, line] });
		} else {
			close();
			cur = { type: "p", lines: [line] };
			blocks.push(cur);
		}
	}
	close();

	let versionLine: string | undefined;
	const rest: LegalBlock[] = [];
	for (const block of blocks) {
		if (block.type === "p" && versionLine === undefined) {
			const index = block.lines.findIndex((l) => l.startsWith("版本："));
			if (index >= 0) {
				versionLine = block.lines[index];
				const lines = block.lines.filter((_, i) => i !== index);
				if (lines.length > 0) rest.push({ type: "p", lines });
				continue;
			}
		}
		rest.push(block);
	}
	if (title === undefined) throw new Error("正本沒有標題");
	if (versionLine === undefined) throw new Error("正本沒有版本行");
	const match = VERSION_LINE.exec(versionLine);
	if (!match) throw new Error(`版本行格式不符：${versionLine}`);
	return { title, versionLine, version: match[1], blocks: rest };
}
