import type { OutputBlockData, OutputData } from "@editorjs/editorjs";

export interface DescriptionEditorCleanupResult {
	data: OutputData;
	changed: boolean;
	/** Number of original blocks changed, including an emptied list removed. */
	changedBlocks: number;
}

const INLINE_TAGS = new Set([
	"a",
	"b",
	"strong",
	"i",
	"em",
	"u",
	"span",
	"small",
	"mark",
	"s",
	"strike",
	"sub",
	"sup",
	"del",
	"font",
]);
const TOKENS =
	/<(?:[^>"']|"[^"]*"|'[^']*')*>|&(?:#[xX][\da-fA-F]+|#\d+|[a-zA-Z]+);|[\s\S]/g;
const SPACE = "[ \\t\\u00a0]";
const HEADING = new RegExp(`^${SPACE}{0,3}(#{1,6})${SPACE}+`);
const BULLET = new RegExp(`^${SPACE}{0,3}([-+*•])${SPACE}+`);
const ORDERED = new RegExp(`^${SPACE}{0,3}(\\d+)[.)]${SPACE}+`);

function visibleToken(token: string): string {
	if (token.startsWith("<")) {
		if (/^<br\s*\/?\s*>$/i.test(token)) return "\n";
		const name = token.match(/^<\/?([\w-]+)/)?.[1]?.toLowerCase();
		return name && INLINE_TAGS.has(name) ? "" : "\ufffc";
	}
	if (!token.startsWith("&")) return token;
	const numeric = token.match(/^&#(x[\da-f]+|\d+);$/i)?.[1];
	if (numeric) {
		const code =
			numeric[0].toLowerCase() === "x"
				? Number.parseInt(numeric.slice(1), 16)
				: Number.parseInt(numeric, 10);
		return code > 0 && code <= 0x10ffff ? String.fromCodePoint(code) : "\ufffc";
	}
	return (
		(
			{
				"&nbsp;": "\u00a0",
				"&ensp;": " ",
				"&emsp;": " ",
				"&thinsp;": " ",
				"&amp;": "&",
				"&lt;": "<",
				"&gt;": ">",
				"&quot;": '"',
				"&apos;": "'",
				"&bull;": "•",
			} as Record<string, string>
		)[token.toLowerCase()] ?? "\ufffc"
	);
}

function visibleText(html: string): string {
	return (html.match(TOKENS) ?? []).map(visibleToken).join("");
}

/** Remove text characters only; never rewrite surrounding links/style tags. */
function removeVisiblePrefix(html: string, length: number): string {
	let remaining = length;
	return (html.match(TOKENS) ?? [])
		.map((token) => {
			const visible = visibleToken(token);
			if (!visible || remaining <= 0) return token;
			remaining -= visible.length;
			return "";
		})
		.join("");
}

function bulletMatch(text: string): RegExpMatchArray | null {
	const match = text.match(BULLET);
	// A spaced sign before a numeric value may be a temperature/measurement.
	// Existing content must not silently turn a negative value positive.
	if (
		match &&
		/[-+]/.test(match[1]) &&
		/^\d/.test(text.slice(match[0].length))
	) {
		return null;
	}
	return match;
}

function cleanListText(html: string): string {
	let next = html;
	let match = bulletMatch(visibleText(next));
	while (match) {
		next = removeVisiblePrefix(next, match[0].length);
		match = bulletMatch(visibleText(next));
	}
	return next;
}

function cleanHeadingText(html: string): string {
	let next = html;
	let match = visibleText(next).match(HEADING);
	while (match && visibleText(next).slice(match[0].length).trim()) {
		next = removeVisiblePrefix(next, match[0].length);
		match = visibleText(next).match(HEADING);
	}
	return next;
}

function cleanListItems(items: unknown[]): unknown[] {
	const result: unknown[] = [];
	for (const item of items) {
		if (typeof item === "string") {
			const content = cleanListText(item);
			if (visibleText(content).trim()) result.push(content);
			continue;
		}
		if (!item || typeof item !== "object") {
			result.push(item);
			continue;
		}
		const record = item as Record<string, unknown>;
		if (typeof record.content !== "string") {
			result.push(item);
			continue;
		}
		const content = cleanListText(record.content);
		const children = Array.isArray(record.items)
			? cleanListItems(record.items)
			: undefined;
		// Empty parent items may hold meaningful children, checklist metadata,
		// or unfamiliar fields. Preserve that structure rather than flatten it.
		const hasOtherData = Object.keys(record).some(
			(key) => !["content", "items"].includes(key),
		);
		if (visibleText(content).trim() || children?.length || hasOtherData) {
			result.push({
				...record,
				content,
				...(children ? { items: children } : {}),
			});
		}
	}
	return result;
}

/** Split soft breaks while balancing inline tags spanning multiple lines. */
function splitInlineLines(html: string): string[] | null {
	const lines: string[] = [];
	const stack: { name: string; open: string }[] = [];
	let line = "";
	for (const token of html.match(TOKENS) ?? []) {
		if (/^<br\s*\/?\s*>$/i.test(token) || token === "\n" || token === "\r") {
			if (token === "\r") continue;
			lines.push(
				line +
					[...stack]
						.reverse()
						.map(({ name }) => `</${name}>`)
						.join(""),
			);
			line = stack.map(({ open }) => open).join("");
			continue;
		}
		if (token.startsWith("<")) {
			const match = token.match(/^<(\/?)([\w-]+)(?:\s[^>]*)?>$/);
			const name = match?.[2].toLowerCase();
			if (!match || !name || !INLINE_TAGS.has(name)) return null;
			if (match[1]) {
				if (stack.at(-1)?.name !== name) return null;
				stack.pop();
			} else {
				stack.push({ name, open: token });
			}
		}
		line += token;
	}
	if (stack.length) return null;
	lines.push(line);
	return lines;
}

function structureLines(lines: string[]): OutputBlockData[] | null {
	const text = lines.map(visibleText);
	// Do not flatten indented/nested Markdown or interpret a code example.
	if (
		text.some(
			(line) =>
				/^\s*(```|~~~)/.test(line) ||
				/^( {4}|\t)\S/.test(line) ||
				/^[ \t]+(?:[-+*•]|\d+[.)])[ \t]+/.test(line),
		)
	)
		return null;
	const result: OutputBlockData[] = [];
	let recognized = false;
	let paragraph: string[] = [];
	const flushParagraph = () => {
		if (paragraph.length)
			result.push({
				type: "paragraph",
				data: { text: paragraph.join("<br>") },
			});
		paragraph = [];
	};
	for (let index = 0; index < lines.length; index++) {
		const heading = text[index].match(HEADING);
		const bullet = bulletMatch(text[index]);
		const ordered = text[index].match(ORDERED);
		if (heading && text[index].slice(heading[0].length).trim()) {
			flushParagraph();
			result.push({
				type: "header",
				data: {
					text: cleanHeadingText(lines[index]),
					level: heading[1].length <= 2 ? 2 : 3,
				},
			});
			recognized = true;
		} else if (bullet || ordered?.[1] === "1") {
			const items: string[] = [];
			let cursor = index;
			for (; cursor < lines.length; cursor++) {
				const match = bullet
					? bulletMatch(text[cursor])
					: text[cursor].match(ORDERED);
				if (!match || (!bullet && Number(match[1]) !== cursor - index + 1))
					break;
				const content = cleanListText(
					removeVisiblePrefix(lines[cursor], match[0].length),
				);
				if (visibleText(content).trim()) items.push(content);
			}
			// Editor.js v1 ordered lists cannot retain custom start values.
			// Only convert a complete sequence from 1; leave ambiguous singletons.
			if (bullet || items.length > 1) {
				flushParagraph();
				if (items.length) {
					result.push({
						type: "list",
						data: { style: bullet ? "unordered" : "ordered", items },
					});
				}
				index = cursor - 1;
				recognized = true;
			} else paragraph.push(lines[index]);
		} else if (!text[index].trim()) {
			flushParagraph();
		} else paragraph.push(lines[index]);
	}
	flushParagraph();
	return recognized ? result : null;
}

/**
 * Explicit, reversible cleanup for the editor toolbar. Never call on load or
 * save: imported text is the seller's source of truth until they choose this.
 * Unknown block types/data, links, styles, list nesting, IDs and tunes survive.
 */
export function cleanupDescriptionEditorData(
	output: OutputData,
): DescriptionEditorCleanupResult {
	const usedIds = new Set(
		output.blocks.map((block) => block.id).filter(Boolean),
	);
	let changedBlocks = 0;
	const blocks = output.blocks.flatMap((block) => {
		let next: OutputBlockData[] = [block];
		if (block.type === "header" && typeof block.data.text === "string") {
			const match = visibleText(block.data.text).match(HEADING);
			if (match && visibleText(block.data.text).slice(match[0].length).trim()) {
				next = [
					{
						...block,
						data: { ...block.data, text: cleanHeadingText(block.data.text) },
					},
				];
			}
		} else if (block.type === "list" && Array.isArray(block.data.items)) {
			const items = cleanListItems(block.data.items);
			next = items.length ? [{ ...block, data: { ...block.data, items } }] : [];
		} else if (
			block.type === "paragraph" &&
			typeof block.data.text === "string"
		) {
			const lines = splitInlineLines(block.data.text);
			const structured = lines && structureLines(lines);
			if (structured) {
				next = structured.map((part, index) => {
					const { text: _text, ...extraData } = block.data;
					const transformed = {
						...block,
						type: part.type,
						data: { ...extraData, ...part.data },
					};
					if (index && block.id) {
						let suffix = index;
						let id = `${block.id}-cleanup-${suffix}`;
						while (usedIds.has(id)) id = `${block.id}-cleanup-${++suffix}`;
						usedIds.add(id);
						transformed.id = id;
					}
					return transformed;
				});
			}
		}
		if (JSON.stringify(next) !== JSON.stringify([block])) changedBlocks++;
		return next;
	});
	return {
		data: changedBlocks ? { ...output, blocks } : output,
		changed: changedBlocks > 0,
		changedBlocks,
	};
}

/**
 * Conservative plain-text clipboard helper; returns null for normal prose,
 * nested/code Markdown, a lone bullet, or ambiguous numeric measurements.
 * Only use when the clipboard has no rich HTML and the paste replaces an
 * entire empty block. Inline syntax stays literal; never treat HTML as markup.
 */
export function parseDescriptionMarkdownPaste(
	text: string,
): OutputBlockData[] | null {
	const normalized = text.replace(/\r\n?/g, "\n");
	const lines = normalized.split("\n");
	const headings = lines.some((line) => HEADING.test(line));
	const listLines = lines.filter(
		(line) => bulletMatch(line) || ORDERED.test(line),
	).length;
	if (!headings && listLines < 2) return null;
	const escaped = lines.map((line) =>
		line.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;"),
	);
	return structureLines(escaped);
}
