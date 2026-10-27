#!/usr/bin/env node

/**
 * JSX 中文換行 gate：JSX 文字節點裡的換行會被折成一個半形空白，中文段落因此
 * 憑空多出空格。中文沒有詞間空白，那個空格看得出來，嚴重時還會斷在詞中間
 * （「要等商品␣上架之後」）。編譯、型別、biome 都不會有任何反應——它是純粹的
 * 渲染瑕疵，只有把畫面看清楚的人才會發現，而 review 只看 diff。
 *
 * 判定完全照 Babel 的 `cleanJSXElementLiteralChild`：文字節點逐行 trim，空行
 * 丟掉，剩下的每一行（除了最後一個非空行）尾端補一個空白再串起來。所以只要
 * 節點內有「兩個相鄰的非空行」，兩行之間就一定會多一個空白。
 *
 * 只在「空白兩側都是中日韓字元」時才算違規：英文本來就要靠空白分詞，
 * `支援 Apple Pay` 這種跨行是正確寫法，誤報會讓整支 lint 恆紅。
 *
 * 修法是把整句放進字串表達式（`{"…"}`），不要用 `{" "}` 或 `&nbsp;`——那兩種
 * 是把多出來的空白「宣告成刻意的」，等於把 bug 固化。
 * 正例：features/delegated-operations/orders/RevealRecipientDialog.tsx。
 *
 * 用 TypeScript parser 而不是逐行 regex：只有真正的 JsxText 節點才判，註解、
 * 字串字面值、樣板字串裡的中文換行一律不看（它們的換行是真的換行，不會被折成
 * 空白）。regex 版本會把那些全部誤報，而誤報比漏報更糟。
 */

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import ts from "typescript-compiler-api";

const root = process.cwd();
const failures: string[] = [];

// 中日韓字元與全形標點。含 CJK 標點（、。「」）與全形形式（，：！？），因為
// 「，換行」正是本 class 最常見的形狀。刻意不含 U+3000 表意空格——它本身就是
// 刻意排版用的空白，多一個半形空白對它沒有可辨識的差別。
const CJK = /[、-〿ㄅ-ㄯ㐀-䶿一-鿿豈-﫿︰-﹏！-｠￠-￦]/;

/**
 * 回傳這個 JSX 文字節點裡「會被折成多餘空白」的位置：每個元素是該空白左側那一
 * 行在節點內的相對行號（0-based）＋前後字元，供訊息指出斷在哪個詞。
 *
 * 依 Babel：`\t` 先當空白、非首行去掉行首空白、非末行去掉行尾空白，空行丟棄，
 * 相鄰的非空行之間補一個半形空白。
 */
function foldedCjkGaps(
	raw: string,
): Array<{ line: number; before: string; after: string }> {
	const lines = raw.split(/\r\n|\n|\r/);
	const kept: Array<{ index: number; text: string }> = [];
	lines.forEach((line, index) => {
		let trimmed = line.replace(/\t/g, " ");
		if (index !== 0) trimmed = trimmed.replace(/^ +/, "");
		if (index !== lines.length - 1) trimmed = trimmed.replace(/ +$/, "");
		if (trimmed) kept.push({ index, text: trimmed });
	});

	const gaps: Array<{ line: number; before: string; after: string }> = [];
	for (let i = 0; i < kept.length - 1; i++) {
		const before = kept[i].text.slice(-1);
		const after = kept[i + 1].text.slice(0, 1);
		if (CJK.test(before) && CJK.test(after)) {
			gaps.push({ line: kept[i].index, before, after });
		}
	}
	return gaps;
}

function scan(path: string): void {
	const source = readFileSync(join(root, path), "utf8");
	const sourceFile = ts.createSourceFile(
		path,
		source,
		ts.ScriptTarget.Latest,
		/* setParentNodes */ true,
		ts.ScriptKind.TSX,
	);

	const visit = (node: ts.Node): void => {
		if (node.kind === ts.SyntaxKind.JsxText) {
			const start = node.getFullStart();
			const raw = source.slice(start, node.getEnd());
			for (const gap of foldedCjkGaps(raw)) {
				const { line } = sourceFile.getLineAndCharacterOfPosition(start);
				failures.push(
					`${path}:${line + gap.line + 1}: JSX 中文文字節點跨行 —「${gap.before}」與「${gap.after}」之間會被折出一個半形空白。整句改寫成字串表達式 {"…"}（禁用 {" "} / &nbsp;，那是把瑕疵固化）。`,
				);
			}
			return;
		}
		ts.forEachChild(node, visit);
	};
	ts.forEachChild(sourceFile, visit);
}

function walk(dir: string, match: RegExp): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(join(root, dir));
	} catch {
		return out;
	}
	for (const entry of entries) {
		if (
			entry === "node_modules" ||
			entry === ".next" ||
			entry === ".git" ||
			entry === ".worktrees"
		) {
			continue;
		}
		const rel = relative(root, join(root, dir, entry));
		// 掃描中被刪掉的檔案 / 壞掉的 symlink 要退化成「跳過」，不能讓整條 lint 崩掉。
		let isDirectory: boolean;
		try {
			isDirectory = statSync(join(root, rel)).isDirectory();
		} catch {
			continue;
		}
		if (isDirectory) out.push(...walk(rel, match));
		else if (match.test(rel)) out.push(rel);
	}
	return out;
}

// Self-check。
// 必攔：全形逗號跨行、斷在詞中間、中間夾一整行空白的兩段中文——最後那組同時證明
// 「空行先丟掉再判相鄰」這條 Babel 規則有實作到，漏掉它會讓夾空行的段落靜靜漏網。
// 必放：英文跨行（本來就需要那個空白）、單行中文、中英兩種交界、純空白節點。
const selfCaught = [
	"\n\t\t\t\t\t這是本次代行唯一會取得未遮罩收件資訊的動作，\n\t\t\t\t\t並計入結束摘要。\n\t\t\t\t",
	"\n\t\t\t\t\t要等商品\n\t\t\t\t\t上架之後才看得到。\n\t\t\t\t",
	"\n\t\t\t第一段\n\n\t\t\t第二段\n\t\t",
];
const selfIgnored = [
	"\n\t\t\t\t\tThis dialog is the only place that reveals\n\t\t\t\t\tthe unmasked recipient.\n\t\t\t\t",
	"\n\t\t\t\t\t這一整句寫在同一行，不會被折出任何空白。\n\t\t\t\t",
	"\n\t\t\t\t\t支援\n\t\t\t\t\tApple Pay 與 Google Pay。\n\t\t\t\t",
	"\n\t\t\t\t\t送出後\n\t\t\t\t\tPayUni 會回傳結果。\n\t\t\t\t",
	"\n\t\t\t\t",
];
if (
	selfCaught.some((raw) => foldedCjkGaps(raw).length === 0) ||
	selfIgnored.some((raw) => foldedCjkGaps(raw).length > 0)
) {
	console.error(
		"check-jsx-cjk-linebreak self-test failed — the gate predicate is broken.",
	);
	process.exit(1);
}

const files = ["lib", "features", "components", "app"].flatMap((d) =>
	walk(d, /\.tsx$/),
);
for (const path of files) scan(path);

if (failures.length > 0) {
	console.error(`JSX CJK line-break gate failed:\n${failures.join("\n")}`);
	process.exit(1);
}

console.log(`JSX CJK line-break gate passed (${files.length} files scanned).`);
