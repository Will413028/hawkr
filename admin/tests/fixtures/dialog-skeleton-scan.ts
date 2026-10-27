import { createRequire } from "node:module";
import type * as TS from "typescript-compiler-api";

// TypeScript 7 沒有穩定的 JS compiler API；與 website、ui-text 一樣用 typescript-compiler-api（@typescript/typescript6 別名）。
const ts = createRequire(import.meta.url)(
	"typescript-compiler-api",
) as typeof TS;

/**
 * 對話框骨架的偵測：用 TypeScript compiler API 解析，不用正規表示式掃原始碼文字，
 * 所以字串字面值、`{"…"}`、樣板字串、`clsx("…")`、`a ? "…" : "…"`、`"modal-" + "overlay"`、
 * 物件屬性與 `setAttribute` 都看得到；註解與一般文字不會誤報。
 */

/**
 * TS／TSX 裡的對話框骨架 class：`*-modal-overlay`／`-content`／`-backdrop`、`modal-header`／`-body`／`-footer`／`-close`、
 * `adm-operation-dialog`／`-overlay`／`-header`／`-body`／`-close` 及其 `--modifier`。
 * 動作列的 `adm-operation-dialog-actions--preflight` 是 Dialog 的 `footerClassName` 修飾，呼叫端可以傳，所以不在清單內。
 */
const SKELETON_CLASS =
	/^(?:[a-z0-9]+-)*(?:modal|dialog)-(?:overlay|content|backdrop)(?:--[\w-]+)?$|^modal-(?:header|body|footer|close)(?:--[\w-]+)?$|^adm-operation-dialog(?:-(?:overlay|header|body|close))?(?:--[\w-]+)?$/u;

/**
 * CSS 裡更嚴：任何 `.modal`、`.modal-*`、`.adm-operation-dialog`、`.adm-operation-dialog-*`，以及帶前綴的
 * `*-modal-overlay`／`-content`／`-backdrop`，只能出現在 Dialog 自己的樣式表。
 * 頁面樣式表重寫 `.modal-body` 或 `.adm-operation-dialog-actions`，就會又出現「長相取決於先逛過哪一頁」。
 */
const CSS_SKELETON_CLASS =
	/^(?:[a-z0-9]+-)*(?:modal|dialog)-(?:overlay|content|backdrop)(?:--[\w-]+)?$|^modal(?:-[\w-]+)?$|^adm-operation-dialog(?:-[\w-]+)?$/u;

/** 隔離豁免屬性：只有 Toast 可以用。 */
const EXEMPT_ATTRIBUTE_NAME = "data-modal-exempt";

const DIALOG_ROLE = /^(?:alert)?dialog$/u;

/** `@/lib/a11y` 裡只該由 Dialog（與導覽抽屜、Toast）碰的 focus／隔離／堆疊 API。 */
const HOOKS = new Set([
	"useEscapeKey",
	"useFocusTrap",
	"useModalIsolation",
	"useModalStack",
	"ModalParentContext",
	"subscribeModalStack",
	"getTopModalOverlay",
]);

export interface Finding {
	kind: "role" | "aria-modal" | "class" | "hook" | "native-dialog" | "exempt";
	detail: string;
	line: number;
}

/** 目前掃描的檔案裡 `const NAME = "字串"` 的對照，讓 `role={R}` 這類間接寫法解得開。 */
let constStrings = new Map<string, TS.Expression>();

/** 表達式內所有可靜態得知的字串片段（含條件分支、`??`／`||`、`+` 相接、樣板字串的固定段）。 */
function stringsOf(node: TS.Node): string[] {
	if (ts.isStringLiteralLike(node)) return [node.text];
	if (ts.isIdentifier(node)) {
		const init = constStrings.get(node.text);
		if (!init) return [];
		// 解開一次就好，避免 `const a = b; const b = a` 無限遞迴。
		constStrings.delete(node.text);
		try {
			return stringsOf(init);
		} finally {
			constStrings.set(node.text, init);
		}
	}
	if (ts.isTemplateExpression(node)) {
		return [
			node.head.text,
			...node.templateSpans.flatMap((s) => [
				...stringsOf(s.expression),
				s.literal.text,
			]),
		];
	}
	if (ts.isBinaryExpression(node)) {
		if (node.operatorToken.kind === ts.SyntaxKind.PlusToken) {
			const left = stringsOf(node.left);
			const right = stringsOf(node.right);
			// 常數相接："modal-" + "overlay"
			if (left.length === 1 && right.length === 1) return [left[0] + right[0]];
		}
		return [...stringsOf(node.left), ...stringsOf(node.right)];
	}
	if (ts.isConditionalExpression(node)) {
		return [...stringsOf(node.whenTrue), ...stringsOf(node.whenFalse)];
	}
	if (
		ts.isParenthesizedExpression(node) ||
		ts.isAsExpression(node) ||
		ts.isNonNullExpression(node) ||
		ts.isSatisfiesExpression(node)
	) {
		return stringsOf(node.expression);
	}
	if (ts.isJsxExpression(node) && node.expression)
		return stringsOf(node.expression);
	if (ts.isCallExpression(node) || ts.isArrayLiteralExpression(node)) {
		const items = ts.isCallExpression(node) ? node.arguments : node.elements;
		return items.flatMap((item) => stringsOf(item));
	}
	if (ts.isObjectLiteralExpression(node)) {
		return node.properties.flatMap((p) => {
			if (ts.isPropertyAssignment(p)) {
				return [...stringsOf(p.name), ...stringsOf(p.initializer)];
			}
			return [];
		});
	}
	if (ts.isComputedPropertyName(node)) return stringsOf(node.expression);
	return [];
}

function propertyName(name: TS.PropertyName | TS.JsxAttributeName): string {
	if (ts.isIdentifier(name) || ts.isStringLiteralLike(name)) return name.text;
	if (ts.isJsxNamespacedName(name)) return name.getText();
	return "";
}

function classTokens(strings: string[]): string[] {
	return strings
		.flatMap((s) => s.split(/\s+/u))
		.filter((t) => SKELETON_CLASS.test(t));
}

export function scanSource(fileName: string, text: string): Finding[] {
	const source = ts.createSourceFile(
		fileName,
		text,
		ts.ScriptTarget.Latest,
		true,
		fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
	);
	constStrings = new Map();
	const collect = (node: TS.Node) => {
		if (
			ts.isVariableDeclaration(node) &&
			ts.isIdentifier(node.name) &&
			node.initializer &&
			ts.isVariableDeclarationList(node.parent) &&
			node.parent.flags & ts.NodeFlags.Const
		) {
			constStrings.set(node.name.text, node.initializer);
		}
		ts.forEachChild(node, collect);
	};
	collect(source);
	const findings: Finding[] = [];
	const add = (kind: Finding["kind"], detail: string, node: TS.Node) => {
		findings.push({
			kind,
			detail,
			line: source.getLineAndCharacterOfPosition(node.getStart()).line + 1,
		});
	};
	const checkRole = (valueNode: TS.Node, at: TS.Node) => {
		for (const s of stringsOf(valueNode)) {
			if (DIALOG_ROLE.test(s.trim())) add("role", s.trim(), at);
		}
	};

	const visit = (node: TS.Node) => {
		if (ts.isJsxAttribute(node)) {
			const name = propertyName(node.name);
			if (name === "aria-modal" || name === "ariaModal") {
				add("aria-modal", name, node);
			}
			if (name === "role" && node.initializer)
				checkRole(node.initializer, node);
		}
		if (
			ts.isPropertyAssignment(node) ||
			ts.isShorthandPropertyAssignment(node)
		) {
			const name = propertyName(node.name);
			if (name === "aria-modal" || name === "ariaModal") {
				add("aria-modal", name, node);
			}
			if (name === "role" && ts.isPropertyAssignment(node)) {
				checkRole(node.initializer, node);
			}
		}
		if (
			ts.isCallExpression(node) &&
			ts.isPropertyAccessExpression(node.expression) &&
			node.expression.name.text === "setAttribute" &&
			node.arguments.length >= 2
		) {
			const attr = stringsOf(node.arguments[0]);
			if (attr.includes("aria-modal")) add("aria-modal", "setAttribute", node);
			if (attr.includes("role")) checkRole(node.arguments[1], node);
		}
		// 直接呼叫、經命名空間（`a11y.useFocusTrap(…)`）呼叫、或 import 時改名（`useFocusTrap as trap`）都算。
		if (ts.isCallExpression(node)) {
			const callee = node.expression;
			if (ts.isIdentifier(callee) && HOOKS.has(callee.text)) {
				add("hook", callee.text, node);
			}
			if (
				ts.isPropertyAccessExpression(callee) &&
				HOOKS.has(callee.name.text)
			) {
				add("hook", callee.name.text, node);
			}
			if (
				ts.isPropertyAccessExpression(callee) &&
				callee.name.text === "showModal"
			) {
				add("native-dialog", "showModal", node);
			}
		}
		if (ts.isPropertyAccessExpression(node) && HOOKS.has(node.name.text)) {
			add("hook", node.name.text, node);
		}
		if (ts.isImportSpecifier(node)) {
			const imported = (node.propertyName ?? node.name).text;
			if (HOOKS.has(imported)) add("hook", imported, node);
			if (imported === "MODAL_EXEMPT_ATTRIBUTE") {
				add("exempt", imported, node);
			}
		}
		if (ts.isIdentifier(node) && node.text === "MODAL_EXEMPT_ATTRIBUTE") {
			add("exempt", node.text, node);
		}
		if (ts.isJsxOpeningLikeElement(node)) {
			const tag = node.tagName;
			if (ts.isIdentifier(tag) && tag.text === "dialog") {
				add("native-dialog", "<dialog>", node);
			}
		}
		if (ts.isStringLiteralLike(node) && node.text === EXEMPT_ATTRIBUTE_NAME) {
			add("exempt", EXEMPT_ATTRIBUTE_NAME, node);
		}
		if (
			ts.isJsxAttribute(node) &&
			propertyName(node.name) === EXEMPT_ATTRIBUTE_NAME
		) {
			add("exempt", EXEMPT_ATTRIBUTE_NAME, node);
		}
		if (
			ts.isStringLiteralLike(node) ||
			ts.isTemplateExpression(node) ||
			(ts.isBinaryExpression(node) &&
				node.operatorToken.kind === ts.SyntaxKind.PlusToken)
		) {
			for (const token of classTokens(stringsOf(node))) {
				add("class", token, node);
			}
		}
		ts.forEachChild(node, visit);
	};
	visit(source);
	// 巢狀的字串表達式會重複看到同一個 token，同一行同一種只留一筆。
	const seen = new Set<string>();
	return findings.filter((f) => {
		const key = `${f.kind}|${f.detail}|${f.line}`;
		if (seen.has(key)) return false;
		seen.add(key);
		return true;
	});
}

/** CSS 內以 class 選擇器定義的對話框骨架（註解不算）。 */
export function scanCss(text: string): string[] {
	const stripped = text.replace(/\/\*[\s\S]*?\*\//gu, "");
	const found = new Set<string>();
	for (const m of stripped.matchAll(/\.([A-Za-z_][\w-]*)/gu)) {
		if (CSS_SKELETON_CLASS.test(m[1])) found.add(m[1]);
	}
	return [...found].sort();
}
