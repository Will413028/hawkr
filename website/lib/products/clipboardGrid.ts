/**
 * Clipboard → table-grid parsing for the description editor's Notion-style
 * paste behaviour (issue #1246).
 *
 * Two sources produce a grid:
 *   1. `text/html` containing a `<table>` — Excel / Google Sheets / web pages
 *      all put a full HTML document on the clipboard; we take the first table
 *      and keep a sanitised inline-HTML string per cell (bold / italic /
 *      underline / colour / link survive, matching the inline-run model).
 *   2. `text/plain` TSV — tab-separated text (e.g. copied from a terminal or
 *      plain-text file). To avoid false positives on prose that happens to
 *      contain a tab, we only treat it as a table when there are ≥2 lines and
 *      every non-empty line contains a tab.
 *
 * Grids are clamped to the backend table limits (10 cols × 50 rows × 200
 * chars per cell) before insertion; callers surface a toast when clamped.
 */

/** Backend validator limits（product-service 描述表格上限）。 */
export const TABLE_MAX_COLS = 10;
export const TABLE_MAX_ROWS = 50;
export const TABLE_MAX_CELL_LEN = 200;

const ESCAPE_ENTITIES: Record<string, string> = {
	"&": "&amp;",
	"<": "&lt;",
	">": "&gt;",
	'"': "&quot;",
	"'": "&#39;",
};

function escapeHtml(text: string): string {
	return text.replace(/[&<>]/g, (c) => ESCAPE_ENTITIES[c] ?? c);
}

/** 屬性上下文必須連引號一起轉義——否則 href/color 原文可挾帶 `" onmouseover="…`
 *  之類的字串突破屬性邊界，變成活的事件屬性（self-XSS）。 */
function escapeAttr(text: string): string {
	return text.replace(/[&<>"']/g, (c) => ESCAPE_ENTITIES[c] ?? c);
}

/** escapeHtml 的反解（僅文字實體；順序 & 最後）——截斷長 cell 時先還原成
 *  純文字再量長度，避免對既轉義字串二次轉義與實體長度偏差。 */
function unescapeHtml(text: string): string {
	return text
		.replace(/&lt;/g, "<")
		.replace(/&gt;/g, ">")
		.replace(/&quot;/g, '"')
		.replace(/&#39;/g, "'")
		.replace(/&amp;/g, "&");
}

/** Inline tags the cell keeps — mirrors htmlToRuns' whitelist. */
const INLINE_KEEP = new Set([
	"B",
	"STRONG",
	"I",
	"EM",
	"U",
	"A",
	"SPAN",
	"FONT",
]);

/**
 * Reduce arbitrary cell markup to the inline subset the description model
 * understands. Unknown elements unwrap to their text; block-level structure
 * inside a cell (nested tables, divs) flattens to text with spaces.
 */
function sanitizeInline(node: Node): string {
	if (node.nodeType === Node.TEXT_NODE) {
		return escapeHtml(node.textContent ?? "");
	}
	if (node.nodeType !== Node.ELEMENT_NODE) return "";
	const el = node as HTMLElement;
	const inner = Array.from(el.childNodes).map(sanitizeInline).join("");
	if (!INLINE_KEEP.has(el.tagName)) return inner;
	switch (el.tagName) {
		case "B":
		case "STRONG":
			return inner ? `<b>${inner}</b>` : "";
		case "I":
		case "EM":
			return inner ? `<i>${inner}</i>` : "";
		case "U":
			return inner ? `<u>${inner}</u>` : "";
		case "A": {
			const href = el.getAttribute("href") ?? "";
			return /^https?:\/\//i.test(href) && inner
				? `<a href="${escapeAttr(href)}">${inner}</a>`
				: inner;
		}
		case "SPAN": {
			const color = el.style?.color ?? "";
			return color && inner
				? `<span style="color:${escapeAttr(color)}">${inner}</span>`
				: inner;
		}
		case "FONT": {
			const color = el.getAttribute("color") ?? "";
			return color && inner
				? `<span style="color:${escapeAttr(color)}">${inner}</span>`
				: inner;
		}
		default:
			return inner;
	}
}

/**
 * 真實網站表格常見的畸形結構整形（cookie886 類）：
 * - 全空列剔除（如跨欄純圖片列——img 非白名單，sanitize 後整列空字串）
 * - jagged 列補齊至最寬（colspan 標題列只解出 1 格，短列尾端補空格對齊）
 * 全部剔完為空 → null。純函式，node 可單測。
 */
export function normalizeGrid(grid: string[][]): string[][] | null {
	const rows = grid.filter((cells) => cells.some((c) => c.trim().length > 0));
	if (rows.length === 0) return null;
	const width = Math.max(...rows.map((r) => r.length));
	return rows.map((r) =>
		r.length < width ? [...r, ...Array(width - r.length).fill("")] : r,
	);
}

/** `text/html` 內含 `<table>` 時取第一個表格為格陣；否則 null。 */
export function htmlToGrid(html: string | null | undefined): string[][] | null {
	if (!html || !/<table[\s>]/i.test(html)) return null;
	if (typeof DOMParser === "undefined") return null;
	const doc = new DOMParser().parseFromString(html, "text/html");
	const table = doc.querySelector("table");
	if (!table) return null;
	const rows = Array.from(table.querySelectorAll("tr"));
	const grid = rows
		.map((tr) =>
			Array.from(tr.querySelectorAll("td, th")).map((cell) =>
				sanitizeInline(cell).trim(),
			),
		)
		.filter((cells) => cells.length > 0);
	return normalizeGrid(grid);
}

/**
 * `text/plain` TSV → 格陣。規則：≥2 行且每個非空行都含 tab 才視為表格
 * （單行含 tab 不轉，避免把含 tab 的散文 / 程式碼誤判成表格）。
 */
export function tsvToGrid(plain: string | null | undefined): string[][] | null {
	if (!plain?.includes("\t")) return null;
	const lines = plain
		.replace(/\r\n?/g, "\n")
		.split("\n")
		.filter((l) => l.trim().length > 0);
	if (lines.length < 2) return null;
	if (!lines.every((l) => l.includes("\t"))) return null;
	return lines.map((l) => l.split("\t").map((c) => escapeHtml(c.trim())));
}

/** 剪貼簿 → 格陣：HTML `<table>` 優先，否則 TSV；皆否 → null（放行原生貼上）。 */
export function parseClipboardGrid(
	html: string | null | undefined,
	plain: string | null | undefined,
): string[][] | null {
	return htmlToGrid(html) ?? tsvToGrid(plain);
}

/** 依後端上限裁切；`clamped` = 有內容被裁掉（供 caller 顯示提示）。 */
export function clampGrid(grid: string[][]): {
	grid: string[][];
	clamped: boolean;
} {
	let clamped = false;
	if (grid.length > TABLE_MAX_ROWS) clamped = true;
	const rows = grid.slice(0, TABLE_MAX_ROWS).map((row) => {
		if (row.length > TABLE_MAX_COLS) clamped = true;
		return row.slice(0, TABLE_MAX_COLS).map((cell) => {
			// 上限以純文字長度計（與 backend validator 一致），截斷時退回純文字
			// 避免砍斷 inline 標籤產生破損 HTML。cell 內文字已是轉義字串，先
			// 反解再量長度/截斷，最後重新轉義——否則二次轉義會顯示字面實體。
			const text = unescapeHtml(cell.replace(/<[^>]*>/g, ""));
			if (text.length <= TABLE_MAX_CELL_LEN) return cell;
			clamped = true;
			return escapeHtml(text.slice(0, TABLE_MAX_CELL_LEN));
		});
	});
	return { grid: rows, clamped };
}
