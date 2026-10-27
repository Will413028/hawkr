/**
 * 守門：目錄 enum 的使用者面文字只有一個來源（設計 enum-text-guard-design.md §1；L′ §5.2）。
 *
 * 一條規則 E：使用者面文字出現在「由受管 enum 的某個成員確定成立」的區域裡，就是命中。
 * 受管 enum＝manifest 的 `inCatalog || serverOwned`（共用文案目錄的 enum 與伺服器擁有的 enum），
 * 受管 enum 的任何命中都失敗。其他 enum（單端）的條件形式不失敗、也不回報；對照表形式寫進 `--report`（見下）。
 * 沒有範圍參數、沒有例外清單。
 *
 * - 選擇子：`E.M`／`ns.E.M`，E 由 import 解析到 `@hawkr/api/gen/…`（同名的非 proto 類別不算）。
 *   值號 0 的成員（名稱以 UNSPECIFIED 結尾）不是選擇子：目錄不給值號 0 文字。
 * - 「選出」只算正向：進入區域的每一條路徑都已確定「x 等於成員 M」才算。補集（三元式的另一支、
 *   `if (x === M) return;` 之後、`default:`）不是任何成員的說法，不算。`x === M` 為真、`x !== M` 為假、
 *   `[E.A, …].includes(x)`／`new Set([…]).has(x)` 為真（接收者可以是字面值，或指到這種初始值的 `const`）、`a && b` 為真、
 *   `a || b` 為假才確定相等；`!` 反轉。成員檢查不用在 `guard`：`if (!OPTIONS.includes(x)) return "…"; …` 之後的敘述
 *   是集合裡任何一個值共用的（選項驗證），不是某個值的說法。
 * - 區域與 form：`if`／`else`、`ternary`、`and`／`or`、`guard`（沒有 else、then 一定離開時，同區塊之後的敘述）、
 *   `case`（switch 敘述與 `switch (true)`）、`key`（computed key 的值）、`pair`（同一個物件、陣列、引數列、
 *   JSX 屬性裡與 `E.M` 並列的其他成員；不進入巢狀函式或子元素）。條件裡的識別字若是用到之前宣告的
 *   `const`，初始值是比較或 `cond ? x : null`，就代入（別名，一層一層）。
 * - 產生文字：區域裡有含漢字的字串（字串、樣板字串、JSX 文字），或引用作用域可見的 `const`，其初始值是含漢字的字串：
 *   同檔的（頂層或函式內）或 import 自同一個 client 目錄樹內另一個檔、該檔 `export const`（含 `export { U }`）的；
 *   同檔物件常數的屬性（`TEXT.key`、`TEXT["key"]`）值含漢字也算。不分是 return、賦值、JSX 子節點還是 `throw`。
 *   參數（含解構）、catch 變數、`let`／`var`、解構宣告會遮住同名的常數。
 * - R3：`userMessage(…) ?? "…"`（或 `||`、`(x && userMessage(…)) ?? "…"`），以及 `….kind === "message" ? failure.message : "…"`：錯誤的後備句寫死在呼叫點。改用 `@hawkr/api/connect/errors` 的 `errorText`
 *   （伺服器文字，否則該 code 的目錄預設文字，取消當成 internal），不各自補一句。
 *
 * `--report <file>` 寫出此 client 有「對應表」（`case`、`key`、`pair`）的 enum 全名，給跨端檢查交集
 * （scripts/check-enum-text-cross.sh）；條件形式（if、三元式…）是行為上的分流，只用在失敗判斷，不進 report。
 *
 * 不涵蓋（只做語法分析、沒有型別與資料流的極限）：包在函式、getter 裡的條件；`let`／`var` 變數；先把 enum 對應成本地的值
 * 再以本地值選文字；以 enum 值當索引的陣列或物件；re-export（`export … from`）、namespace import、跨檔的物件常數與
 * class 靜態欄位的常數；第二個 client 對只有一個 client 用的 enum，以條件形式寫出值的說法（跨端檢查只比對對照表，
 * 看不到條件形式）。這些由 code review 把關；會發佈 en 的介面另有字面值 ratchet（I-CI），admin 不在 I-CI 內，
 * 所以 admin 的這些殘餘只靠 code review。
 * 觸發（計畫 I-CI、I-app-0）：「產生文字」要加進 client 資源的查詢（next-intl 的 `t("…")`、`useTranslations`；
 * App 的 ARB）。website 的部分與 I-CI 的 website 那一刀同一刀，因為從那時起新文字一律放進 messages；App 的部分與 I-app-0 同一刀。
 *
 * 用法：tsx tools/check-enum-text.ts [--report <file>] <dir>...
 */
import {
	existsSync,
	readdirSync,
	readFileSync,
	statSync,
	writeFileSync,
} from "node:fs";
import { dirname, join, relative, resolve as resolvePath } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript-compiler-api";

export interface ManifestEnum {
	name: string;
	tsName?: string;
	tsModule?: string;
	inCatalog: boolean;
	serverOwned: boolean;
}

/** 選擇構造的種類（設計 §1.2）。 */
export type Form =
	| "if"
	| "else"
	| "ternary"
	| "and"
	| "or"
	| "guard"
	| "case"
	| "key"
	| "pair";

/** 進 `--report` 的形式：值到文字的對應表。其餘是行為上的分流，只用在失敗判斷。 */
export const REPORT_FORMS: ReadonlySet<Form> = new Set<Form>([
	"case",
	"key",
	"pair",
]);

export interface Hit {
	file: string;
	/** 選擇子（`E.M`）所在的行。 */
	line: number;
	rule: "E";
	form: Form;
	/** proto enum 的全名。 */
	enumName: string;
	member: string;
}

/**
 * 一個 import 說明符指到的檔裡，`export const` 且初始值是含漢字字串的名稱；解析不到（不在同一個 client 目錄樹內、
 * 不是相對路徑或別名）回 undefined。scanSource 不碰檔案系統，由呼叫端（scanDir）提供。
 */
export type ConstResolver = (
	spec: string,
	fromFile: string,
) => ReadonlySet<string> | undefined;

const HAN = /[㐀-䶿一-鿿豈-﫿]/;
const API_GEN_PREFIX = "@hawkr/api/gen/";
const UNSPECIFIED = /UNSPECIFIED$/;

const EXCLUDED_DIRS = new Set([
	"node_modules",
	".next",
	".open-next",
	".wrangler",
	".turbo",
	"dist",
	"build",
	"coverage",
	"gen",
	"codemeta",
	"tests",
	"__tests__",
	"e2e",
	".git",
]);

export function loadManifest(): ManifestEnum[] {
	const path = join(
		dirname(fileURLToPath(import.meta.url)),
		"../src/enum-manifest.gen.json",
	);
	return (JSON.parse(readFileSync(path, "utf8")) as { enums: ManifestEnum[] })
		.enums;
}

function isTestFile(name: string): boolean {
	return /\.(test|spec)\.[cm]?[tj]sx?$/.test(name) || name.endsWith(".gen.ts");
}

function listSources(dir: string): string[] {
	const out: string[] = [];
	for (const entry of readdirSync(dir)) {
		const full = join(dir, entry);
		const st = statSync(full);
		if (st.isDirectory()) {
			if (!EXCLUDED_DIRS.has(entry)) out.push(...listSources(full));
			continue;
		}
		if (!/\.(ts|tsx)$/.test(entry) || entry.endsWith(".d.ts")) continue;
		if (isTestFile(entry)) continue;
		out.push(full);
	}
	return out.sort();
}

function parse(fileName: string, text: string): ts.SourceFile {
	return ts.createSourceFile(
		fileName,
		text,
		ts.ScriptTarget.Latest,
		true,
		fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
	);
}

/** 節點本身是含漢字的字串片段（字串、樣板字串的各段、JSX 文字）。 */
function isHanLiteral(n: ts.Node): boolean {
	return (
		(ts.isStringLiteral(n) ||
			ts.isNoSubstitutionTemplateLiteral(n) ||
			ts.isTemplateHead(n) ||
			ts.isTemplateMiddle(n) ||
			ts.isTemplateTail(n) ||
			ts.isJsxText(n)) &&
		HAN.test(n.text)
	);
}

function containsHanLiteral(node: ts.Node): boolean {
	let found = false;
	const visit = (n: ts.Node): void => {
		if (found) return;
		if (isHanLiteral(n)) {
			found = true;
			return;
		}
		ts.forEachChild(n, visit);
	};
	visit(node);
	return found;
}

/** 初始值是含漢字的字串（字串、樣板字串，可包在括號、`as`、`satisfies` 裡）。 */
function isHanStringInit(e: ts.Expression | undefined): boolean {
	if (!e) return false;
	const cur = stripWrappers(e);
	return (
		(ts.isStringLiteral(cur) ||
			ts.isNoSubstitutionTemplateLiteral(cur) ||
			ts.isTemplateExpression(cur)) &&
		containsHanLiteral(cur)
	);
}

function isFunctionLike(n: ts.Node): n is ts.FunctionLikeDeclaration {
	return (
		ts.isArrowFunction(n) ||
		ts.isFunctionExpression(n) ||
		ts.isFunctionDeclaration(n) ||
		ts.isMethodDeclaration(n) ||
		ts.isGetAccessorDeclaration(n) ||
		ts.isSetAccessorDeclaration(n) ||
		ts.isConstructorDeclaration(n)
	);
}

/** 一個檔裡 `export const X = "含漢字的字串"` 的名稱（跨檔常數，K4）。 */
export function exportedHanConsts(
	fileName: string,
	text: string,
): ReadonlySet<string> {
	const names = new Set<string>();
	const local = new Set<string>();
	const sf = parse(fileName, text);
	for (const st of sf.statements) {
		if (
			!ts.isVariableStatement(st) ||
			!(st.declarationList.flags & ts.NodeFlags.Const)
		) {
			continue;
		}
		const exported = st.modifiers?.some(
			(m) => m.kind === ts.SyntaxKind.ExportKeyword,
		);
		for (const d of st.declarationList.declarations) {
			if (ts.isIdentifier(d.name) && isHanStringInit(d.initializer)) {
				local.add(d.name.text);
				if (exported) names.add(d.name.text);
			}
		}
	}
	// `export { U, V as W }`（沒有 from）：宣告在別處的常數。
	for (const st of sf.statements) {
		if (
			ts.isExportDeclaration(st) &&
			!st.moduleSpecifier &&
			st.exportClause &&
			ts.isNamedExports(st.exportClause)
		) {
			for (const el of st.exportClause.elements) {
				if (local.has((el.propertyName ?? el.name).text))
					names.add(el.name.text);
			}
		}
	}
	return names;
}

/** 綁定名稱（含解構）是否宣告了 name。 */
function bindsName(b: ts.BindingName, name: string): boolean {
	if (ts.isIdentifier(b)) return b.text === name;
	return b.elements.some(
		(el) => !ts.isOmittedExpression(el) && bindsName(el.name, name),
	);
}

/** 變數宣告查到的結果：`shadow` 是名稱被別的東西遮住（參數、`let`／`var`），不是可代入的常數。 */
type VarInfo = { init: ts.Expression | undefined } | "shadow";

/**
 * 掃一個檔，回「由受管或單端 enum 成員確定成立的區域裡產生使用者面文字」的位置。
 * fileName 只決定副檔名（.tsx 用 JSX 解析）與回報的路徑；consts 解析 import 進來的常數（K4），沒給就只看同檔。
 */
export function scanSource(
	fileName: string,
	text: string,
	manifest: readonly ManifestEnum[],
	consts?: ConstResolver,
): Hit[] {
	const sf = parse(fileName, text);
	const byModuleName = new Map<string, ManifestEnum>();
	for (const e of manifest) {
		if (e.tsModule && e.tsName)
			byModuleName.set(`${e.tsModule}#${e.tsName}`, e);
	}
	// 區域識別字 → enum；namespace import 的前綴 → 模組；import 進來的含漢字常數的區域名稱。
	const locals = new Map<string, ManifestEnum>();
	const namespaces = new Map<string, string>();
	const importedConsts = new Set<string>();
	for (const stmt of sf.statements) {
		if (
			!ts.isImportDeclaration(stmt) ||
			!ts.isStringLiteral(stmt.moduleSpecifier)
		)
			continue;
		const spec = stmt.moduleSpecifier.text;
		const bindings = stmt.importClause?.namedBindings;
		if (!bindings) continue;
		if (!spec.startsWith(API_GEN_PREFIX)) {
			if (!ts.isNamedImports(bindings)) continue;
			const exported = consts?.(spec, fileName);
			if (!exported) continue;
			for (const el of bindings.elements) {
				if (exported.has((el.propertyName ?? el.name).text))
					importedConsts.add(el.name.text);
			}
			continue;
		}
		if (ts.isNamespaceImport(bindings)) {
			namespaces.set(bindings.name.text, spec);
			continue;
		}
		for (const el of bindings.elements) {
			const imported = (el.propertyName ?? el.name).text;
			const e = byModuleName.get(`${spec}#${imported}`);
			if (e) locals.set(el.name.text, e);
		}
	}
	if (locals.size === 0 && namespaces.size === 0) return [];

	interface Sel {
		e: ManifestEnum;
		member: string;
		node: ts.Node;
	}

	// `E.M` 或 `ns.E.M`（可包在括號、`as`、`!`、`satisfies` 裡）；值號 0 的成員不是選擇子（K2）。
	const select = (expr: ts.Expression): Sel | undefined => {
		const x = stripWrappers(expr);
		if (!ts.isPropertyAccessExpression(x)) return undefined;
		const member = x.name.text;
		if (UNSPECIFIED.test(member)) return undefined;
		const base = x.expression;
		if (ts.isIdentifier(base)) {
			const e = locals.get(base.text);
			return e ? { e, member, node: x } : undefined;
		}
		if (
			ts.isPropertyAccessExpression(base) &&
			ts.isIdentifier(base.expression)
		) {
			const spec = namespaces.get(base.expression.text);
			const e = spec
				? byModuleName.get(`${spec}#${base.name.text}`)
				: undefined;
			return e ? { e, member, node: x } : undefined;
		}
		return undefined;
	};

	// 作用域（K4、K5 共用）：從使用處往上找 Block／函式／SourceFile／case 子句；區塊裡只看使用處之前的宣告，
	// SourceFile 看全部（函式本體在呼叫時才執行，頂層宣告的位置不影響）。參數、`let`／`var` 遮住同名的 const。
	const lookupVar = (id: ts.Identifier): VarInfo | undefined => {
		const name = id.text;
		const pos = id.getStart(sf);
		for (let n: ts.Node | undefined = id.parent; n; n = n.parent) {
			if (
				(isFunctionLike(n) &&
					n.parameters.some((p) => bindsName(p.name, name))) ||
				(ts.isCatchClause(n) &&
					n.variableDeclaration &&
					bindsName(n.variableDeclaration.name, name))
			) {
				return "shadow";
			}
			const stmts =
				ts.isBlock(n) ||
				ts.isSourceFile(n) ||
				ts.isModuleBlock(n) ||
				ts.isCaseClause(n) ||
				ts.isDefaultClause(n)
					? n.statements
					: undefined;
			if (!stmts) continue;
			for (const st of stmts) {
				if (!ts.isVariableStatement(st)) continue;
				if (!ts.isSourceFile(n) && st.end > pos) continue;
				for (const d of st.declarationList.declarations) {
					if (!bindsName(d.name, name)) continue;
					return ts.isIdentifier(d.name) &&
						st.declarationList.flags & ts.NodeFlags.Const
						? { init: d.initializer }
						: "shadow";
				}
			}
		}
		return undefined;
	};

	/** 識別字是「引用」，不是宣告、屬性名或 JSX 屬性名。 */
	const isReference = (id: ts.Identifier): boolean => {
		const p = id.parent;
		if (ts.isPropertyAccessExpression(p) && p.name === id) return false;
		if (ts.isQualifiedName(p) && p.right === id) return false;
		if (ts.isBindingElement(p) && p.propertyName === id) return false;
		if (ts.isTypeReferenceNode(p)) return false;
		if (
			(ts.isPropertyAssignment(p) ||
				ts.isPropertyDeclaration(p) ||
				ts.isPropertySignature(p) ||
				ts.isMethodDeclaration(p) ||
				ts.isEnumMember(p) ||
				ts.isJsxAttribute(p) ||
				ts.isVariableDeclaration(p) ||
				ts.isParameter(p) ||
				ts.isBindingElement(p) ||
				ts.isFunctionDeclaration(p) ||
				ts.isImportSpecifier(p) ||
				ts.isExportSpecifier(p)) &&
			p.name === id
		) {
			return false;
		}
		return true;
	};

	/** 識別字是含漢字字串的 const：同檔（作用域可見）或 import 進來的（K4）。 */
	const isHanConst = (id: ts.Identifier): boolean => {
		const v = lookupVar(id);
		if (v === "shadow") return false;
		if (v) return isHanStringInit(v.init);
		return importedConsts.has(id.text);
	};

	/** `TEXT.key`、`TEXT["key"]`：同檔的物件常數，該屬性的值含漢字字串（K4）。 */
	const isObjectConstText = (n: ts.Node): boolean => {
		let obj: ts.Expression;
		let prop: string | undefined;
		if (ts.isPropertyAccessExpression(n)) {
			obj = n.expression;
			prop = n.name.text;
		} else if (
			ts.isElementAccessExpression(n) &&
			ts.isStringLiteralLike(n.argumentExpression)
		) {
			obj = n.expression;
			prop = n.argumentExpression.text;
		} else {
			return false;
		}
		if (!ts.isIdentifier(obj)) return false;
		const v = lookupVar(obj);
		if (!v || v === "shadow" || !v.init) return false;
		const init = stripWrappers(v.init);
		if (!ts.isObjectLiteralExpression(init)) return false;
		return init.properties.some(
			(p) =>
				ts.isPropertyAssignment(p) &&
				(ts.isIdentifier(p.name) || ts.isStringLiteralLike(p.name)) &&
				p.name.text === prop &&
				containsHanLiteral(p.initializer),
		);
	};

	/** 區域裡產生文字（K4）。shallow：不進入巢狀函式本體（pair 用）。 */
	const hasText = (node: ts.Node | undefined, shallow = false): boolean => {
		if (!node) return false;
		let found = false;
		const visit = (n: ts.Node): void => {
			if (found) return;
			if (shallow && isFunctionLike(n)) return;
			if (
				isHanLiteral(n) ||
				(ts.isIdentifier(n) && isReference(n) && isHanConst(n)) ||
				isObjectConstText(n)
			) {
				found = true;
				return;
			}
			ts.forEachChild(n, visit);
		};
		visit(node);
		return found;
	};

	const isNullish = (e: ts.Expression): boolean => {
		const x = stripWrappers(e);
		return (
			x.kind === ts.SyntaxKind.NullKeyword ||
			(ts.isIdentifier(x) && x.text === "undefined")
		);
	};

	const isEq = (k: ts.SyntaxKind): boolean =>
		k === ts.SyntaxKind.EqualsEqualsEqualsToken ||
		k === ts.SyntaxKind.EqualsEqualsToken;
	const isNe = (k: ts.SyntaxKind): boolean =>
		k === ts.SyntaxKind.ExclamationEqualsEqualsToken ||
		k === ts.SyntaxKind.ExclamationEqualsToken;

	/** 成員檢查的接收者：陣列字面值、`new Set([…])`，或指到這種初始值的 const（K5 的作用域查找）。 */
	const collectionOf = (
		e: ts.Expression,
		depth = 0,
	): ts.ArrayLiteralExpression | undefined => {
		let x = stripWrappers(e);
		if (ts.isNewExpression(x) && x.arguments?.length) {
			x = stripWrappers(x.arguments[0] as ts.Expression);
		}
		if (ts.isArrayLiteralExpression(x)) return x;
		if (ts.isIdentifier(x) && depth < 4) {
			const v = lookupVar(x);
			if (v && v !== "shadow" && v.init) return collectionOf(v.init, depth + 1);
		}
		return undefined;
	};

	/**
	 * expr 取 pol（真或假）時確定「相等」的成員（K3）。`&&` 為真、`||` 為假合併兩邊；`!` 反轉；
	 * 其他回空（`&&` 為假、`||` 為真、`!==` 為真都不蘊含任何成員）。別名照 K5 代入。
	 */
	const implied = (
		expr: ts.Expression,
		pol: boolean,
		depth = 0,
		membership = true,
	): Sel[] => {
		if (depth > 8) return [];
		const c = stripWrappers(expr);
		if (
			ts.isPrefixUnaryExpression(c) &&
			c.operator === ts.SyntaxKind.ExclamationToken
		) {
			return implied(c.operand, !pol, depth + 1, membership);
		}
		if (ts.isBinaryExpression(c)) {
			const k = c.operatorToken.kind;
			if (k === ts.SyntaxKind.AmpersandAmpersandToken) {
				return pol
					? [
							...implied(c.left, true, depth + 1, membership),
							...implied(c.right, true, depth + 1, membership),
						]
					: [];
			}
			if (k === ts.SyntaxKind.BarBarToken) {
				return pol
					? []
					: [
							...implied(c.left, false, depth + 1, membership),
							...implied(c.right, false, depth + 1, membership),
						];
			}
			if (isEq(k) || isNe(k)) {
				// `alias != null`（或 `alias == null` 為假）：別名 `cond ? x : null` 有值，等於它的 cond 為真。
				const other = isNullish(c.right)
					? c.left
					: isNullish(c.left)
						? c.right
						: undefined;
				if (other) {
					const o = stripWrappers(other);
					const v = ts.isIdentifier(o) ? lookupVar(o) : undefined;
					const init =
						v && v !== "shadow" && v.init ? stripWrappers(v.init) : undefined;
					return isNe(k) === pol && init && ts.isConditionalExpression(init)
						? implied(o, true, depth + 1, membership)
						: [];
				}
				if (isEq(k) !== pol) return [];
				const s = select(c.left) ?? select(c.right);
				return s ? [s] : [];
			}
			return [];
		}
		if (
			ts.isCallExpression(c) &&
			ts.isPropertyAccessExpression(c.expression) &&
			(c.expression.name.text === "includes" ||
				c.expression.name.text === "has")
		) {
			if (!pol || !membership) return [];
			const rcv = collectionOf(c.expression.expression);
			if (!rcv) return [];
			return rcv.elements.flatMap((el) => {
				const s = select(el);
				return s ? [s] : [];
			});
		}
		if (ts.isIdentifier(c)) {
			const v = lookupVar(c);
			if (!v || v === "shadow" || !v.init) return [];
			const init = stripWrappers(v.init);
			if (ts.isConditionalExpression(init)) {
				if (!pol) return [];
				if (isNullish(init.whenFalse))
					return implied(init.condition, true, depth + 1, membership);
				if (isNullish(init.whenTrue))
					return implied(init.condition, false, depth + 1, membership);
				return [];
			}
			return implied(init, pol, depth + 1, membership);
		}
		return [];
	};

	const hits: Hit[] = [];
	const seen = new Set<string>();
	/** sels 在 regions 裡有文字就記一筆。 */
	const emit = (
		sels: readonly Sel[],
		regions: readonly (ts.Node | undefined)[],
		form: Form,
		shallow = false,
	): void => {
		if (sels.length === 0) return;
		if (!regions.some((r) => hasText(r, shallow))) return;
		for (const s of sels) {
			const line =
				sf.getLineAndCharacterOfPosition(s.node.getStart(sf)).line + 1;
			const key = `${line}:${form}:${s.e.name}.${s.member}`;
			if (seen.has(key)) continue;
			seen.add(key);
			hits.push({
				file: fileName,
				line,
				rule: "E",
				form,
				enumName: s.e.name,
				member: s.member,
			});
		}
	};

	const exits = (s: ts.Statement): boolean => {
		const last = ts.isBlock(s) ? s.statements[s.statements.length - 1] : s;
		return (
			!!last &&
			(ts.isReturnStatement(last) ||
				ts.isThrowStatement(last) ||
				ts.isBreakStatement(last) ||
				ts.isContinueStatement(last))
		);
	};

	const selectors = (
		items: readonly ts.Node[],
		valueExpr: (n: ts.Node) => ts.Expression | undefined,
	): Sel[] =>
		items.flatMap((n) => {
			const v = valueExpr(n);
			const s = v ? select(v) : undefined;
			return s ? [s] : [];
		});

	/** 並列的成員（物件屬性、陣列元素、引數）：有選擇子時，其他成員有文字就命中。 */
	const pair = (
		items: readonly ts.Node[],
		valueExpr: (n: ts.Node) => ts.Expression | undefined,
	): void => {
		const sels = selectors(items, valueExpr);
		if (sels.length === 0) return;
		const others = items.filter((n) => {
			const v = valueExpr(n);
			return !(v && select(v));
		});
		emit(sels, others, "pair", true);
	};

	const visit = (n: ts.Node): void => {
		if (ts.isIfStatement(n)) {
			emit(implied(n.expression, true), [n.thenStatement], "if");
			emit(implied(n.expression, false), [n.elseStatement], "else");
			const p = n.parent;
			if (
				!n.elseStatement &&
				exits(n.thenStatement) &&
				(ts.isBlock(p) ||
					ts.isSourceFile(p) ||
					ts.isModuleBlock(p) ||
					ts.isCaseClause(p) ||
					ts.isDefaultClause(p))
			) {
				const after = p.statements.slice(p.statements.indexOf(n) + 1);
				emit(implied(n.expression, false, 0, false), after, "guard");
			}
		}
		if (ts.isConditionalExpression(n)) {
			emit(implied(n.condition, true), [n.whenTrue], "ternary");
			emit(implied(n.condition, false), [n.whenFalse], "ternary");
		}
		if (ts.isBinaryExpression(n)) {
			if (n.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken)
				emit(implied(n.left, true), [n.right], "and");
			if (n.operatorToken.kind === ts.SyntaxKind.BarBarToken)
				emit(implied(n.left, false), [n.right], "or");
		}
		if (ts.isCaseBlock(n)) {
			const onTrue =
				stripWrappers(n.parent.expression).kind === ts.SyntaxKind.TrueKeyword;
			let pending: Sel[] = [];
			for (const clause of n.clauses) {
				if (ts.isDefaultClause(clause)) {
					// default 可由任何值到達：不是確定相等；它的敘述也不屬於前面 fall-through 進來的成員。
					pending = [];
					continue;
				}
				const direct = select(clause.expression);
				if (direct) pending.push(direct);
				else if (onTrue) pending.push(...implied(clause.expression, true));
				if (clause.statements.length === 0) continue;
				emit(pending, clause.statements, "case");
				pending = [];
			}
		}
		if (ts.isPropertyAssignment(n) && ts.isComputedPropertyName(n.name)) {
			const s = select(n.name.expression);
			if (s) emit([s], [n.initializer], "key");
		}
		if (ts.isObjectLiteralExpression(n)) {
			pair(n.properties, (p) =>
				ts.isPropertyAssignment(p) ? p.initializer : undefined,
			);
		}
		if (ts.isArrayLiteralExpression(n)) {
			pair(n.elements, (e) => (ts.isExpression(e) ? e : undefined));
		}
		if ((ts.isCallExpression(n) || ts.isNewExpression(n)) && n.arguments) {
			pair(n.arguments, (a) => (ts.isExpression(a) ? a : undefined));
		}
		if (ts.isJsxElement(n) || ts.isJsxSelfClosingElement(n)) {
			const attrs = (ts.isJsxElement(n) ? n.openingElement : n).attributes
				.properties;
			const valueExpr = (a: ts.Node): ts.Expression | undefined =>
				ts.isJsxAttribute(a) &&
				a.initializer &&
				ts.isJsxExpression(a.initializer)
					? a.initializer.expression
					: undefined;
			const sels = selectors(attrs, valueExpr);
			if (sels.length > 0) {
				const others: ts.Node[] = attrs.filter((a) => {
					const v = valueExpr(a);
					return !(v && select(v));
				});
				if (ts.isJsxElement(n)) {
					for (const c of n.children) {
						if (ts.isJsxText(c)) others.push(c);
						else if (ts.isJsxExpression(c) && c.expression) {
							const x = stripWrappers(c.expression);
							if (
								ts.isStringLiteral(x) ||
								ts.isNoSubstitutionTemplateLiteral(x) ||
								ts.isTemplateExpression(x) ||
								ts.isIdentifier(x)
							)
								others.push(c);
						}
					}
				}
				emit(sels, others, "pair", true);
			}
		}
		ts.forEachChild(n, visit);
	};
	visit(sf);
	return hits;
}

export interface FallbackHit {
	file: string;
	line: number;
}

function stripWrappers(e: ts.Expression): ts.Expression {
	let cur = e;
	while (
		ts.isParenthesizedExpression(cur) ||
		ts.isNonNullExpression(cur) ||
		ts.isAsExpression(cur) ||
		ts.isSatisfiesExpression(cur)
	) {
		cur = cur.expression;
	}
	return cur;
}

/** 右側是寫死的句子：字面值，或分支／運算元裡有字面值。呼叫（`t("x")`、`ut.…`）是目錄查詢，不算。 */
function isLiteralText(e: ts.Expression): boolean {
	const cur = stripWrappers(e);
	if (
		ts.isStringLiteral(cur) ||
		ts.isNoSubstitutionTemplateLiteral(cur) ||
		ts.isTemplateExpression(cur)
	) {
		return true;
	}
	if (ts.isConditionalExpression(cur)) {
		return isLiteralText(cur.whenTrue) || isLiteralText(cur.whenFalse);
	}
	if (ts.isBinaryExpression(cur)) {
		return isLiteralText(cur.left) || isLiteralText(cur.right);
	}
	return false;
}

function isUserMessageCall(call: ts.CallExpression): boolean {
	const callee = call.expression;
	return (
		(ts.isIdentifier(callee) && callee.text === "userMessage") ||
		(ts.isPropertyAccessExpression(callee) &&
			callee.name.text === "userMessage")
	);
}

/** 運算式本身是 `userMessage(…)`，或 `&&`／`||`／`??` 串裡有一個（`(err && userMessage(err, d)) ?? "…"`）。 */
function hasUserMessageOperand(e: ts.Expression): boolean {
	const cur = stripWrappers(e);
	if (ts.isCallExpression(cur)) return isUserMessageCall(cur);
	if (
		ts.isBinaryExpression(cur) &&
		(cur.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken ||
			cur.operatorToken.kind === ts.SyntaxKind.BarBarToken ||
			cur.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken)
	) {
		return hasUserMessageOperand(cur.left) || hasUserMessageOperand(cur.right);
	}
	return false;
}

/** 條件是 `x.kind === "message"`（或 `!==`、`==`、`?.`）。 */
function messageKindTest(e: ts.Expression): { negated: boolean } | undefined {
	const cur = stripWrappers(e);
	if (!ts.isBinaryExpression(cur)) return undefined;
	const op = cur.operatorToken.kind;
	const eq =
		op === ts.SyntaxKind.EqualsEqualsEqualsToken ||
		op === ts.SyntaxKind.EqualsEqualsToken;
	const ne =
		op === ts.SyntaxKind.ExclamationEqualsEqualsToken ||
		op === ts.SyntaxKind.ExclamationEqualsToken;
	if (!eq && !ne) return undefined;
	const isMessage = (x: ts.Expression) => {
		const v = stripWrappers(x);
		return ts.isStringLiteral(v) && v.text === "message";
	};
	const isKind = (x: ts.Expression) => {
		const v = stripWrappers(x);
		return ts.isPropertyAccessExpression(v) && v.name.text === "kind";
	};
	const matched =
		(isKind(cur.left) && isMessage(cur.right)) ||
		(isMessage(cur.left) && isKind(cur.right));
	return matched ? { negated: ne } : undefined;
}

/**
 * R3：錯誤文字的寫死後備句。
 * - `userMessage(…) ?? "…"`（`||`、括號、`(cond && userMessage(…)) ?? "…"` 都算）；
 * - `submitError(…).kind === "message" ? failure.message : "…"`（以及 `!==` 的反向）。
 * 右側是目錄查詢（呼叫、屬性）或參數時不擋。不需要 import 判斷，名稱就是這些 helper 的名稱。
 */
export function scanFallbacks(fileName: string, text: string): FallbackHit[] {
	if (!text.includes("userMessage") && !text.includes('"message"')) return [];
	const sf = ts.createSourceFile(
		fileName,
		text,
		ts.ScriptTarget.Latest,
		true,
		fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
	);
	const hits: FallbackHit[] = [];
	const hit = (n: ts.Node): void => {
		const { line } = sf.getLineAndCharacterOfPosition(n.getStart(sf));
		hits.push({ file: fileName, line: line + 1 });
	};
	const visit = (n: ts.Node): void => {
		if (
			ts.isBinaryExpression(n) &&
			(n.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken ||
				n.operatorToken.kind === ts.SyntaxKind.BarBarToken) &&
			hasUserMessageOperand(n.left) &&
			isLiteralText(n.right)
		) {
			hit(n);
		} else if (ts.isConditionalExpression(n)) {
			const test = messageKindTest(n.condition);
			if (test && isLiteralText(test.negated ? n.whenTrue : n.whenFalse)) {
				hit(n);
			}
		}
		ts.forEachChild(n, visit);
	};
	visit(sf);
	return hits;
}

/** 掃目錄的 R3 命中。 */
export function scanFallbackDir(dir: string): FallbackHit[] {
	const hits: FallbackHit[] = [];
	for (const file of listSources(dir)) {
		hits.push(
			...scanFallbacks(relative(dir, file), readFileSync(file, "utf8")),
		);
	}
	return hits;
}

/** 這個 client 目錄的 tsconfig `paths`（`"@/*": ["./src/*"]` 這類），回 [前綴, 目錄]；沒有 tsconfig 就沒有別名。 */
function pathAliases(root: string): [string, string][] {
	const cfg = join(root, "tsconfig.json");
	if (!existsSync(cfg)) return [];
	const parsed = ts.readConfigFile(cfg, ts.sys.readFile);
	const paths = (
		parsed.config as {
			compilerOptions?: { paths?: Record<string, string[]> };
		}
	)?.compilerOptions?.paths;
	const out: [string, string][] = [];
	for (const [pattern, targets] of Object.entries(paths ?? {})) {
		const target = targets[0];
		if (pattern.endsWith("/*") && target?.endsWith("/*")) {
			out.push([pattern.slice(0, -1), resolvePath(root, target.slice(0, -1))]);
		}
	}
	return out;
}

/**
 * 解析跨檔常數（K4）：相對路徑或 tsconfig 別名，而且落在掃描的目錄樹內；
 * 只追一層（被匯入檔自己的 import 不再展開）。
 */
export function makeConstResolver(root: string): ConstResolver {
	const base = resolvePath(root);
	const aliases = pathAliases(base);
	const cache = new Map<string, ReadonlySet<string> | undefined>();
	return (spec, fromFile) => {
		let target: string | undefined;
		if (spec.startsWith(".")) {
			target = resolvePath(dirname(join(base, fromFile)), spec);
		} else {
			const alias = aliases.find(([prefix]) => spec.startsWith(prefix));
			if (alias) target = join(alias[1], spec.slice(alias[0].length));
		}
		if (!target) return undefined;
		const rel = relative(base, target);
		if (rel.startsWith("..") || rel === "") return undefined;
		const file = [
			target,
			`${target}.ts`,
			`${target}.tsx`,
			join(target, "index.ts"),
			join(target, "index.tsx"),
		].find((f) => existsSync(f) && statSync(f).isFile());
		if (!file) return undefined;
		if (!cache.has(file)) {
			cache.set(file, exportedHanConsts(file, readFileSync(file, "utf8")));
		}
		return cache.get(file);
	};
}

/** 掃目錄，回所有命中。路徑相對於 baseDir。 */
export function scanDir(dir: string, manifest: readonly ManifestEnum[]): Hit[] {
	const hits: Hit[] = [];
	const consts = makeConstResolver(dir);
	for (const file of listSources(dir)) {
		hits.push(
			...scanSource(
				relative(dir, file),
				readFileSync(file, "utf8"),
				manifest,
				consts,
			),
		);
	}
	return hits;
}

export interface Report {
	/** proto enum 全名 → 對應表（case、key、pair）的位置。 */
	enums: Record<string, string[]>;
}

/** 只放對應形式（REPORT_FORMS）的命中：跨端檢查問的是「值到文字的對照表」在兩個 client 各寫一份。 */
export function buildReport(hits: readonly Hit[]): Report {
	const enums: Record<string, string[]> = {};
	for (const h of hits) {
		if (!REPORT_FORMS.has(h.form)) continue;
		const list = enums[h.enumName] ?? [];
		list.push(`${h.file}:${h.line}`);
		enums[h.enumName] = list;
	}
	for (const list of Object.values(enums)) list.sort();
	return {
		enums: Object.fromEntries(
			Object.entries(enums).sort(([a], [b]) => a.localeCompare(b)),
		),
	};
}

/** 必須擋下的命中：目錄裡的 enum 與伺服器擁有的 enum，任何 form。 */
export function forbidden(
	hits: readonly Hit[],
	manifest: readonly ManifestEnum[],
): Hit[] {
	const blocked = new Set(
		manifest.filter((e) => e.inCatalog || e.serverOwned).map((e) => e.name),
	);
	return hits.filter((h) => blocked.has(h.enumName));
}

export function main(argv: readonly string[]): number {
	const args = [...argv];
	let reportPath: string | undefined;
	const i = args.indexOf("--report");
	if (i >= 0) {
		reportPath = args[i + 1];
		args.splice(i, 2);
	}
	if (args.length === 0 || (i >= 0 && !reportPath)) {
		console.error("用法：check-enum-text [--report <file>] <dir>...");
		return 2;
	}
	const manifest = loadManifest();
	const hits: Hit[] = [];
	const fallbacks: FallbackHit[] = [];
	for (const dir of args) {
		hits.push(...scanDir(dir, manifest));
		fallbacks.push(...scanFallbackDir(dir));
	}
	if (reportPath) {
		writeFileSync(
			reportPath,
			`${JSON.stringify(buildReport(hits), null, 2)}\n`,
		);
	}
	const bad = forbidden(hits, manifest);
	for (const h of bad) {
		console.error(
			`${h.file}:${h.line}: ${h.rule}/${h.form} ${h.enumName}.${h.member} 成立時產生使用者面文字；這個 enum 的文字在共用目錄（@hawkr/ui-text）或由伺服器擁有`,
		);
	}
	for (const h of fallbacks) {
		console.error(
			`${h.file}:${h.line}: R3 userMessage(…) 後面接了寫死的後備句；改用 @hawkr/api/connect/errors 的 errorText`,
		);
	}
	if (bad.length > 0) {
		console.error(
			`\ncheck-enum-text：${bad.length} 處。文字改用 @hawkr/ui-text 的建構函式，或讀伺服器給的 LocalizedMessage；文字不是這個值的說法時，改成依契約的欄位或伺服器給的可用性分流。`,
		);
	}
	if (bad.length > 0 || fallbacks.length > 0) return 1;
	return 0;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
	try {
		process.exit(main(process.argv.slice(2)));
	} catch (error) {
		// 未捕捉的例外 node 回 1，會和「有命中」混在一起；工具壞了一律回 2。
		console.error(error);
		process.exit(2);
	}
}
