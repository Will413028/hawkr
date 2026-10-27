/**
 * Bridge between Editor.js (the seller-facing block editor) and the backend's
 * `{ version: "1.0", blocks: ProductDescriptionBlock[] }` shape (02 §3.6
 * inline-run 模型) that the mobile apps also read.
 *
 *   Editor.js                       ⇄  backend block
 *   ──────────────────────────────────────────────────────────────
 *   paragraph (inline HTML)         ⇄  { type:"text", runs:[{text,bold,…,href}], style? }
 *   header (level 2 / 3)            ⇄  { type:"text", runs, style:{ fontSize, fontWeight:700 } }
 *   table  (withHeadings, content)  ⇄  { type:"table", tableJson }
 *   image (file.url)                ⇄  { type:"image", imageKey, stagedToken }
 *
 * Inline runs preserve per-segment bold / italic / underline / colour / link.
 * Paragraph-level style keeps only fontSize (heading) + textAlign. Legacy
 * `content` (no runs) is read as a single run for backward compatibility.
 */

import type { OutputBlockData, OutputData } from "@editorjs/editorjs";
import { resolveCdnUrl, toObjectKey } from "@/lib/media/cdn";
import {
	type BlockTextStyle,
	type InlineRun,
	type ProductDescription,
	type ProductDescriptionBlock,
	sanitizeBlockStyle,
} from "@/lib/products/seller-repo";

export const TABLE_CELL_ALIGN_ATTRIBUTE = "data-hk-cell-align";

/** Heading level ⇄ backend fontSize (px string, matches BLOCK_FONT_SIZES). */
const HEADER_SIZE_BY_LEVEL: Record<number, string> = { 2: "26px", 3: "20px" };

function headerLevelForFontSize(fontSize?: string): number | null {
	if (fontSize === "26px") return 2;
	if (fontSize === "20px") return 3;
	return null;
}

function escapeHtml(s: string): string {
	return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function escapeAttr(s: string): string {
	return escapeHtml(s).replace(/"/g, "&quot;");
}

/** Plain-text fallback for non-DOM contexts (e.g. an accidental SSR call). */
function stripTags(html: string): string {
	return html
		.replace(/<br\s*\/?>/gi, "\n")
		.replace(/<[^>]*>/g, "")
		.replace(/&nbsp;/g, " ")
		.replace(/&amp;/g, "&")
		.replace(/&lt;/g, "<")
		.replace(/&gt;/g, ">")
		.replace(/&quot;/g, '"')
		.replace(/&#39;/g, "'");
}

/** Normalise a CSS colour (hex or `rgb(...)`) to `#rrggbb`, or undefined. */
export function toHexColor(raw: string | null | undefined): string | undefined {
	if (!raw) return undefined;
	const s = raw.trim().toLowerCase();
	const short = s.match(/^#([0-9a-f])([0-9a-f])([0-9a-f])$/);
	if (short)
		return `#${short[1]}${short[1]}${short[2]}${short[2]}${short[3]}${short[3]}`;
	if (/^#[0-9a-f]{6}$/.test(s)) return s;
	const rgb = s.match(/^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/);
	if (rgb) {
		const hex = [rgb[1], rgb[2], rgb[3]]
			.map((n) => Number(n).toString(16).padStart(2, "0"))
			.join("");
		return `#${hex}`;
	}
	return undefined;
}

/* ============================================================
 *            Inline runs  ⇄  Editor.js inline HTML
 * ============================================================ */

/** A single run → the inline HTML Editor.js understands (link outermost so a
 *  linked bold word round-trips). */
function runToHtml(run: InlineRun): string {
	let html = escapeHtml(run.text ?? "");
	if (run.color) html = `<span style="color:${run.color}">${html}</span>`;
	if (run.underline) html = `<u>${html}</u>`;
	if (run.italic) html = `<i>${html}</i>`;
	if (run.bold) html = `<b>${html}</b>`;
	if (run.href) html = `<a href="${escapeAttr(run.href)}">${html}</a>`;
	return html;
}

function runsToHtml(runs: InlineRun[] | undefined): string {
	if (!runs || runs.length === 0) return "";
	return runs.map(runToHtml).join("");
}

function styleKey(r: InlineRun): string {
	return [
		r.bold ? "b" : "",
		r.italic ? "i" : "",
		r.underline ? "u" : "",
		r.color ?? "",
		r.href ?? "",
	].join("|");
}

/** Merge adjacent runs with identical style, drop empty — the canonical form
 *  that keeps save → reload → save idempotent (02 §3.6.1). */
function normalizeRuns(runs: InlineRun[]): InlineRun[] {
	const out: InlineRun[] = [];
	for (const r of runs) {
		if (!r.text) continue;
		const last = out[out.length - 1];
		if (last && styleKey(last) === styleKey(r)) {
			last.text += r.text;
		} else {
			// Browser commands can wrap the same formatting in different orders
			// (color > bold versus bold > color). Keep the serialized field order
			// canonical too: history compares JSON signatures after re-rendering.
			out.push({
				text: r.text,
				...(r.bold ? { bold: true } : {}),
				...(r.italic ? { italic: true } : {}),
				...(r.underline ? { underline: true } : {}),
				...(r.color ? { color: r.color } : {}),
				...(r.href ? { href: r.href } : {}),
			});
		}
	}
	return out;
}

function walkInline(node: Node, style: InlineRun, runs: InlineRun[]): void {
	for (const child of Array.from(node.childNodes)) {
		if (child.nodeType === 3 /* TEXT_NODE */) {
			const text = child.textContent ?? "";
			if (text) runs.push({ ...style, text });
			continue;
		}
		if (child.nodeType !== 1 /* ELEMENT_NODE */) continue;
		const el = child as HTMLElement;
		const tag = el.tagName.toLowerCase();
		if (tag === "br") {
			runs.push({ ...style, text: " " });
			continue;
		}
		const next: InlineRun = { ...style, text: "" };
		if (tag === "b" || tag === "strong") next.bold = true;
		else if (tag === "i" || tag === "em") next.italic = true;
		else if (tag === "u") next.underline = true;
		else if (tag === "a") {
			const href = el.getAttribute("href");
			if (href && /^https?:\/\//i.test(href)) next.href = href.trim();
		}
		// Browser editing commands and rich-text paste can express formatting as
		// CSS spans after foreColor enables styleWithCSS. Persist the same runs as
		// semantic b/i/u tags, including explicit removal inside a formatted span.
		const weight = el.style.fontWeight;
		if (weight === "bold" || weight === "bolder" || Number(weight) >= 600)
			next.bold = true;
		else if (
			weight === "normal" ||
			(Number(weight) > 0 && Number(weight) < 600)
		)
			delete next.bold;
		const fontStyle = el.style.fontStyle;
		if (fontStyle === "italic" || fontStyle.startsWith("oblique"))
			next.italic = true;
		else if (fontStyle === "normal") delete next.italic;
		const decoration = el.style.textDecorationLine || el.style.textDecoration;
		if (decoration.includes("underline")) next.underline = true;
		else if (decoration === "none") delete next.underline;
		const color = toHexColor(el.style?.color || el.getAttribute("color"));
		if (color) next.color = color;
		walkInline(el, next, runs);
	}
}

/** Editor.js inline HTML → normalized runs. */
function htmlToRuns(html: string): InlineRun[] {
	if (!html) return [];
	if (typeof document === "undefined") {
		const text = stripTags(html);
		return text.trim() ? [{ text }] : [];
	}
	const host = document.createElement("div");
	host.innerHTML = html;
	const runs: InlineRun[] = [];
	walkInline(host, { text: "" }, runs);
	// Native deletion may leave <br> until a tool's keyup cleanup. Treat it
	// like the eventual empty value so saving between input and keyup cannot
	// create a phantom history step. Preserve all spacing around real content.
	return runs.some((run) => run.text.trim()) ? normalizeRuns(runs) : [];
}

/** Re-wrap legacy plain text (no runs) in the inline tags for whole-paragraph
 *  style captured on a previous save, so old rows still render before backfill. */
function wrapLegacyInline(escapedText: string, style: BlockTextStyle): string {
	let out = escapedText;
	if (style.color) out = `<span style="color:${style.color}">${out}</span>`;
	if (style.textDecoration === "underline") out = `<u>${out}</u>`;
	if (style.fontStyle === "italic") out = `<i>${out}</i>`;
	if (style.fontWeight === 700) out = `<b>${out}</b>`;
	return out;
}

function isRenderableImage(url: string | undefined): url is string {
	return !!url && url !== "__pending__" && url !== "__error__";
}

/** Name of the block tune that carries paragraph alignment in the editor. */
const ALIGN_TUNE = "align";

function tunesForStyle(
	style: BlockTextStyle,
): OutputBlockData["tunes"] | undefined {
	if (style.textAlign === "center" || style.textAlign === "right") {
		return { [ALIGN_TUNE]: { alignment: style.textAlign } };
	}
	return undefined;
}

function alignmentFromTunes(
	tunes: OutputBlockData["tunes"],
): "center" | "right" | undefined {
	const raw = tunes?.[ALIGN_TUNE] as { alignment?: unknown } | undefined;
	const a = raw?.alignment;
	return a === "center" || a === "right" ? a : undefined;
}

/* ============================================================
 *                    Table  ⇄  Editor.js table
 * ============================================================ */

interface TableCell {
	runs?: InlineRun[];
	textAlign?: "center" | "right";
}
interface TableModel {
	withHeadings?: boolean;
	rows?: TableCell[][];
}

function tableCellAlignment(value: unknown): "center" | "right" | undefined {
	return value === "center" || value === "right" ? value : undefined;
}

function editorHtmlForTableCell(cell: TableCell | undefined): string {
	const html = runsToHtml(cell?.runs);
	const textAlign = tableCellAlignment(cell?.textAlign);
	return textAlign
		? `<div ${TABLE_CELL_ALIGN_ATTRIBUTE}="${textAlign}" style="text-align:${textAlign}">${html}</div>`
		: html;
}

function tableCellFromEditorHtml(cellHtml: unknown): TableCell {
	const raw = String(cellHtml ?? "");
	const wrapper = raw
		.trim()
		.match(
			/^<div\b(?=[^>]*\bdata-hk-cell-align=(['"])(center|right)\1)[^>]*>([\s\S]*)<\/div>$/i,
		);
	const textAlign = tableCellAlignment(wrapper?.[2]?.toLowerCase());
	const html = wrapper?.[3] ?? raw;
	return {
		runs: htmlToRuns(html),
		...(textAlign ? { textAlign } : {}),
	};
}

/** backend tableJson string → Editor.js table tool data. */
function tableJsonToEditorData(tableJson: string | undefined): {
	withHeadings: boolean;
	content: string[][];
} | null {
	if (!tableJson) return null;
	let model: TableModel;
	try {
		model = JSON.parse(tableJson) as TableModel;
	} catch {
		return null;
	}
	const rows = Array.isArray(model.rows) ? model.rows : [];
	const content = rows.map((row) =>
		(Array.isArray(row) ? row : []).map(editorHtmlForTableCell),
	);
	if (content.length === 0) return null;
	return { withHeadings: !!model.withHeadings, content };
}

/** Editor.js table tool data → backend tableJson string. */
function editorDataToTableJson(data: unknown): string | null {
	const d = data as { withHeadings?: boolean; content?: unknown };
	const content = Array.isArray(d?.content) ? (d.content as unknown[]) : [];
	const rows: TableCell[][] = content
		.map((row) =>
			(Array.isArray(row) ? (row as unknown[]) : []).map(
				tableCellFromEditorHtml,
			),
		)
		.filter((row) => row.length > 0);
	if (rows.length === 0) return null;
	return JSON.stringify({ withHeadings: !!d?.withHeadings, rows });
}

/* ============================================================
 *                     List  ⇄  Editor.js list
 * ============================================================ */

interface ListItem {
	runs?: InlineRun[];
}
interface ListModel {
	ordered?: boolean;
	items?: ListItem[];
	textAlign?: "center" | "right";
}

/** `@editorjs/list` item may be a plain HTML string (flat list) or a nested
 *  `{ content, items }` object — extract the item's own HTML either way. */
function listItemHtml(item: unknown): string {
	if (typeof item === "string") return item;
	const o = item as { content?: unknown } | null;
	return typeof o?.content === "string" ? o.content : "";
}

/** backend listJson string → Editor.js list tool data. */
function listJsonToEditorData(listJson: string | undefined): {
	style: "ordered" | "unordered";
	items: string[];
	textAlign?: "center" | "right";
} | null {
	if (!listJson) return null;
	let model: ListModel;
	try {
		model = JSON.parse(listJson) as ListModel;
	} catch {
		return null;
	}
	const items = Array.isArray(model.items) ? model.items : [];
	const htmlItems = items.map((it) => runsToHtml(it?.runs));
	if (htmlItems.length === 0) return null;
	const textAlign = tableCellAlignment(model.textAlign);
	return {
		style: model.ordered ? "ordered" : "unordered",
		items: htmlItems,
		...(textAlign ? { textAlign } : {}),
	};
}

/** Editor.js list tool data → backend listJson string. */
function editorDataToListJson(
	data: unknown,
	textAlign?: "center" | "right",
): string | null {
	const d = data as { style?: unknown; items?: unknown };
	const rawItems = Array.isArray(d?.items) ? (d.items as unknown[]) : [];
	const items: ListItem[] = rawItems.map((it) => ({
		runs: htmlToRuns(listItemHtml(it)),
	}));
	if (items.length === 0) return null;
	return JSON.stringify({
		ordered: d?.style === "ordered",
		items,
		...(textAlign ? { textAlign } : {}),
	});
}

/* ============================================================
 *                 backend  ⇄  Editor.js data
 * ============================================================ */

/** backend description → Editor.js initial/render data. */
export function toEditorData(
	desc: ProductDescription | null | undefined,
): OutputData {
	const blocks: OutputBlockData[] = [];
	for (const blk of desc?.blocks ?? []) {
		if (blk.type === "image") {
			// API responses carry a display URL, but local undo/redo snapshots use
			// fromEditorData's write-safe shape and only retain the object key.
			// Resolve that key through the shared media gateway instead of dropping
			// the image or giving <img> a relative path. Keep an explicit URL intact
			// (it may be signed); pending/error images must still stay excluded.
			const displayUrl =
				blk.imageUrl ??
				(isRenderableImage(blk.imageKey)
					? resolveCdnUrl(toObjectKey(blk.imageKey))
					: undefined);
			if (!isRenderableImage(displayUrl)) continue;
			blocks.push({
				id: blk.id,
				type: "image",
				data: {
					file: {
						url: displayUrl,
						imageKey: blk.imageKey,
						assetId: blk.assetId,
						stagedToken: blk.stagedToken,
					},
					caption: blk.alt ?? "",
					withBorder: false,
					withBackground: false,
					stretched: false,
				},
			});
			continue;
		}

		if (blk.type === "table") {
			const table = tableJsonToEditorData(blk.tableJson);
			if (!table) continue;
			blocks.push({
				id: blk.id,
				type: "table",
				data: {
					withHeadings: table.withHeadings,
					content: table.content,
				},
			});
			continue;
		}

		if (blk.type === "list") {
			const list = listJsonToEditorData(blk.listJson);
			if (!list) continue;
			const tunes = tunesForStyle({ textAlign: list.textAlign });
			blocks.push({
				id: blk.id,
				type: "list",
				data: { style: list.style, items: list.items },
				...(tunes ? { tunes } : {}),
			});
			continue;
		}

		// text block
		const style = sanitizeBlockStyle(blk.style);
		const tunes = tunesForStyle(style);
		const level = headerLevelForFontSize(style.fontSize);
		// New rows carry `runs`; legacy rows only `content`.
		const html =
			blk.runs && blk.runs.length > 0
				? runsToHtml(blk.runs)
				: wrapLegacyInline(escapeHtml(blk.content ?? ""), style);
		if (level) {
			blocks.push({
				id: blk.id,
				type: "header",
				data: { text: html, level },
				...(tunes ? { tunes } : {}),
			});
		} else {
			blocks.push({
				id: blk.id,
				type: "paragraph",
				data: { text: html },
				...(tunes ? { tunes } : {}),
			});
		}
	}
	if (blocks.length === 0) {
		blocks.push({ type: "paragraph", data: { text: "" } });
	}
	return { blocks } as OutputData;
}

interface EditorParagraphData {
	text?: string;
}
interface EditorHeaderData {
	text?: string;
	level?: number;
}
interface EditorImageData {
	file?: {
		url?: string;
		imageKey?: string;
		assetId?: string;
		stagedToken?: string;
	};
	url?: string;
	caption?: string;
}

/** Editor.js saved data → backend description (drops empty blocks). */
export function fromEditorData(
	data: OutputData | null | undefined,
): ProductDescription {
	const blocks: ProductDescriptionBlock[] = [];
	for (const b of data?.blocks ?? []) {
		if (b.type === "image") {
			const d = b.data as EditorImageData;
			const displayUrl = d.file?.url ?? d.url;
			if (!isRenderableImage(displayUrl)) continue;
			// 10 檔 §25 / 49 §6.1：只有裸 objectKey 可以送去落庫。Gateway 現在會走
			// resolveInputObjectKey 剝殼＋拒外來域名（seller_product.go:244-260），
			// 但前端仍是第一道防線——新上傳的圖只帶 `file.url`（uploadImageFile 回的是
			// 完整 CDN URL）+ stagedToken，toObjectKey 把它剝回 key；同時也把既有的
			// 髒值在下次存檔時修回。
			blocks.push({
				type: "image",
				id: b.id,
				assetId: d.file?.assetId,
				imageKey: toObjectKey(d.file?.imageKey ?? displayUrl) || undefined,
				stagedToken: d.file?.stagedToken,
				alt: d.caption ?? "",
			});
			continue;
		}

		if (b.type === "table") {
			const tableJson = editorDataToTableJson(b.data);
			if (!tableJson) continue;
			blocks.push({ type: "table", id: b.id, tableJson });
			continue;
		}

		if (b.type === "list") {
			const listJson = editorDataToListJson(
				b.data,
				alignmentFromTunes(b.tunes),
			);
			if (!listJson) continue;
			blocks.push({ type: "list", id: b.id, listJson });
			continue;
		}

		const textAlign = alignmentFromTunes(b.tunes);
		if (b.type === "header") {
			const d = b.data as EditorHeaderData;
			const runs = htmlToRuns(String(d.text ?? ""));
			if (runs.length === 0) continue;
			const level = Number(d.level) || 2;
			blocks.push({
				type: "text",
				id: b.id,
				runs,
				style: {
					fontSize: HEADER_SIZE_BY_LEVEL[level] ?? "20px",
					fontWeight: 700,
					...(textAlign ? { textAlign } : {}),
				},
			});
			continue;
		}

		// paragraph (and any unknown tool) → text block with runs
		const d = b.data as EditorParagraphData;
		const runs = htmlToRuns(String(d.text ?? ""));
		if (runs.length === 0) continue;
		blocks.push({
			type: "text",
			id: b.id,
			runs,
			style: textAlign ? { textAlign } : undefined,
		});
	}
	return { version: "1.0", blocks };
}
