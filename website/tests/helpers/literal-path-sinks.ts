import ts from "typescript-compiler-api";
import { isLocalizablePagePath } from "../../lib/intl/paths";

/**
 * Finds the places where a source file hands a LITERAL root-relative page path to something that is not one
 * of the navigation layer's exits (lib/intl/navigation.ts): a plain `<a href>`, a `<form action>`,
 * `window.open`, `history.pushState/replaceState`. Such a path reaches the browser as written, so an
 * English reader lands on the zh-TW page.
 *
 * Read from the syntax tree, not grepped, because the same path is spelled six ways: `"/x"`, `'/x'`,
 * `{"/x"}`, `` {`/x`} ``, `` {`/x/${id}`} `` (its head is the literal part), `{a ? "/x" : "/y"}`.
 * A path that is not a localizable page (`/api/…`, a `public/` file, a non-localized route) is not a sink:
 * those are meant to go out as written.
 */
export interface LiteralPathSink {
	line: number;
	kind: string;
	path: string;
}

const LOGICAL_OPERATORS = new Set([
	ts.SyntaxKind.BarBarToken,
	ts.SyntaxKind.QuestionQuestionToken,
	ts.SyntaxKind.AmpersandAmpersandToken,
]);

/** The statically known leading text of each way `node` can evaluate. */
function staticHeads(node: ts.Expression): string[] {
	if (
		ts.isParenthesizedExpression(node) ||
		ts.isAsExpression(node) ||
		ts.isNonNullExpression(node) ||
		ts.isSatisfiesExpression(node)
	) {
		return staticHeads(node.expression);
	}
	if (ts.isStringLiteralLike(node)) return [node.text];
	if (ts.isTemplateExpression(node)) return [node.head.text];
	if (ts.isConditionalExpression(node)) {
		return [...staticHeads(node.whenTrue), ...staticHeads(node.whenFalse)];
	}
	if (ts.isBinaryExpression(node)) {
		const operator = node.operatorToken.kind;
		if (operator === ts.SyntaxKind.PlusToken) return staticHeads(node.left);
		if (LOGICAL_OPERATORS.has(operator)) {
			return [...staticHeads(node.left), ...staticHeads(node.right)];
		}
	}
	return [];
}

function literalPage(node: ts.Expression | undefined): string | null {
	if (!node) return null;
	for (const head of staticHeads(node)) {
		if (!head.startsWith("/") || head.startsWith("//")) continue;
		const pathname = head.split(/[?#]/)[0] || "/";
		if (isLocalizablePagePath(pathname)) return head;
	}
	return null;
}

function tagName(element: ts.JsxOpeningLikeElement): string {
	return ts.isIdentifier(element.tagName) ? element.tagName.text : "";
}

function calleeText(call: ts.CallExpression): string {
	return call.expression.getText().replace(/\s+/g, "");
}

export function findLiteralPathSinks(
	file: string,
	code: string,
): LiteralPathSink[] {
	const sf = ts.createSourceFile(
		file,
		code,
		ts.ScriptTarget.Latest,
		true,
		/\.tsx$/.test(file) ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
	);
	const found: LiteralPathSink[] = [];
	const report = (node: ts.Node, kind: string, value: string | null) => {
		if (value === null) return;
		found.push({
			line: sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1,
			kind,
			path: value,
		});
	};
	const visit = (node: ts.Node) => {
		if (ts.isJsxAttribute(node) && ts.isIdentifier(node.name)) {
			const element = node.parent.parent;
			if (
				ts.isJsxOpeningElement(element) ||
				ts.isJsxSelfClosingElement(element)
			) {
				const tag = tagName(element);
				const watched =
					(tag === "a" && node.name.text === "href") ||
					(tag === "form" && node.name.text === "action");
				if (watched) {
					const init = node.initializer;
					const expression =
						init && ts.isJsxExpression(init) ? init.expression : init;
					report(
						node,
						`<${tag} ${node.name.text}>`,
						literalPage(expression as ts.Expression | undefined),
					);
				}
			}
		} else if (ts.isCallExpression(node)) {
			const callee = calleeText(node);
			if (/^(?:window\.|globalThis\.|self\.)?open$/.test(callee)) {
				report(node, "window.open", literalPage(node.arguments[0]));
			} else if (
				/^(?:window\.)?history\.(?:pushState|replaceState)$/.test(callee)
			) {
				report(node, callee, literalPage(node.arguments[2]));
			}
		}
		ts.forEachChild(node, visit);
	};
	visit(sf);
	return found;
}
