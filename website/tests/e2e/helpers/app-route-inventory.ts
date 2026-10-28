import { readdirSync, readFileSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("../../../", import.meta.url));
const APP_ROOT = join(ROOT, "app");

function walk(directory: string): string[] {
	const files: string[] = [];
	for (const entry of readdirSync(directory, { withFileTypes: true })) {
		const path = join(directory, entry.name);
		if (entry.isDirectory()) files.push(...walk(path));
		else files.push(path);
	}
	return files;
}

function routeSegment(segment: string): string | null {
	// `[locale]` is the first URL segment of every localized page, and zh-TW (the
	// default locale) has no prefix: the routes below are the zh-TW URLs.
	if (segment === "[locale]") return null;
	if (segment.startsWith("@")) return null;
	if (/^\([^)]+\)$/.test(segment)) return null;
	const intercepted = segment.match(/^\(\.{1,3}\)(.+)$/);
	return intercepted?.[1] ?? segment;
}

function routeForPage(file: string): string | null {
	const rawSegments = relative(APP_ROOT, file).split(sep).slice(0, -1);
	// Parallel/intercepted slots are alternate renderings of a canonical page,
	// not independently addressable documents.
	if (rawSegments.some((segment) => segment.startsWith("@"))) return null;
	const segments = rawSegments
		.map(routeSegment)
		.filter((segment): segment is string => Boolean(segment));
	return `/${segments.join("/")}`.replace(/\/+$/, "") || "/";
}

export function discoverAppPageRoutes(options?: {
	includeDynamic?: boolean;
}): string[] {
	const includeDynamic = options?.includeDynamic ?? true;
	const routes = walk(APP_ROOT)
		.filter((file) => /(?:^|\/)page\.(?:ts|tsx)$/.test(file))
		.map(routeForPage)
		.filter((route): route is string => Boolean(route))
		.filter((route) => includeDynamic || !route.includes("["));
	return [...new Set(routes)].sort();
}

export interface PermanentRedirect {
	source: string;
	destination: string;
}

export function discoverPermanentRedirects(): PermanentRedirect[] {
	const config = readFileSync(join(ROOT, "next.config.ts"), "utf8");
	return [
		...config.matchAll(
			/\{\s*source:\s*"([^"]+)",\s*destination:\s*"([^"]+)",\s*permanent:\s*true\s*,?\s*\}/g,
		),
	].map((match) => ({
		source: match[1],
		destination: match[2],
	}));
}

/**
 * 只做重導、不渲染任何東西的 `page.tsx`。
 *
 * ⚠️ 這些路由在 App Router 的檔案掃描裡與一般頁面**長得一模一樣**，但它們永遠不會
 * 回 200 —— `redirect()` 在 server component 裡丟出的是 307。把它們留在
 * 「canonical public static page serves 200」的受測集合裡，那支測試就必然紅，
 * 而紅的原因與它要守的東西（頁面壞掉）無關。
 *
 * 為什麼不改成 `next.config.ts` 的 `permanent: true` redirect：那會把語意從
 * **暫時**改成**永久**（308），瀏覽器與搜尋引擎會快取它。對一個暫時的
 * 入口而言那是更強的承諾，不該為了讓測試變綠而做出去。
 *
 * 判準刻意保守：整支檔案除了 import 與 `export default function Page()` 之外
 * 只有一個 `redirect(...)` 呼叫。有任何其他渲染邏輯的頁面**不算**，因為那種頁面
 * 真的可能回 200。
 */
export function isRedirectOnlyPageSource(src: string): boolean {
	if (!/\bredirect\s*\(/.test(src)) return false;
	// 函式主體必須「永遠不渲染」：最後一句是 redirect(...)，它前面只允許
	// 單行守衛（notFound / forbidden / unauthorized / redirect）。有 return
	// JSX 或多行分支的一律排除。
	//
	// 判準看行為而非行數：redirect 前面多一行 `if (...) notFound();` 守衛，
	// 頁面照樣不渲染、照樣回 307，不該因此換類別（曾因此掉回「公開靜態頁」
	// 類、被斷言成 200 而擋掉一次 main 部署）。
	const body = src.match(
		/export\s+default\s+(?:async\s+)?function\s+\w*\s*\([^)]*\)\s*\{([\s\S]*?)\n\}/,
	)?.[1];
	if (!body) return false;
	const meaningful = body
		.split("\n")
		.map((line) => line.trim())
		.filter((line) => line && !line.startsWith("//") && !line.startsWith("*"));
	if (meaningful.length === 0) return false;
	// 最後一句是 redirect(...)：可以跨多行（`redirect({ href, locale })`），
	// 從它開頭的那一行起，括號配平的那一行必須是函式主體的最後一行。
	const start = meaningful.findIndex((line) => /^redirect\s*\(/.test(line));
	if (start === -1) return false;
	let depth = 0;
	let end = -1;
	for (let i = start; i < meaningful.length && end === -1; i++) {
		for (const ch of meaningful[i]) {
			if (ch === "(") depth++;
			else if (ch === ")") depth--;
		}
		if (depth === 0) end = i;
	}
	if (end !== meaningful.length - 1) return false;
	const GUARD_ONLY =
		/^if\s*\(.*\)\s*(?:return\s+)?(?:notFound|forbidden|unauthorized|redirect)\s*\(/;
	return meaningful.slice(0, start).every((line) => GUARD_ONLY.test(line));
}

export function discoverRedirectOnlyPages(): string[] {
	const routes = walk(APP_ROOT)
		.filter((file) => /(?:^|\/)page\.(?:ts|tsx)$/.test(file))
		.filter((file) => isRedirectOnlyPageSource(readFileSync(file, "utf8")))
		.map(routeForPage)
		.filter((route): route is string => Boolean(route));
	return [...new Set(routes)].sort();
}
