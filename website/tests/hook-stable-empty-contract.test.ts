import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const FRESH_EMPTY = /(\?\?|\|\||:)\s*(\[\]|\{\})\s*[),;]*\s*$/;

const ALLOWED = new Map<string, string[]>([
	[
		"lib/hooks/useSellerCustomers.ts",
		["queryFn: () => fetchSellerCustomers(params ?? {}),"],
	],
	[
		"lib/hooks/useSellerProducts.ts",
		["queryFn: () => scope.repos.products.fetchSellerProducts(params ?? {}),"],
	],
	["lib/hooks/useNotifications.ts", ["return next ? [next] : [];"]],
	[
		"lib/hooks/useFeedPosts.ts",
		["first?.records ?? [],", "freshPages[index]?.records ?? [],"],
	],
	[
		"lib/hooks/useMarketProducts.ts",
		[
			": {};",
			"first?.products ?? [],",
			"freshPages[0]?.products ?? [],",
			"freshPages[index]?.products ?? [],",
		],
	],
	["lib/hooks/useShippingOptions.ts", ["validationErrors: [],"]],
	[
		"lib/hooks/useCart.ts",
		["const snapshot = queryClient.getQueryData<CartItem[]>(cartKey) ?? [];"],
	],
	[
		"lib/hooks/useAccount.ts",
		[
			"mutationFn: (body: ProfileEdit) => updateProfile(body, query.data ?? {}),",
			"await updateBuyerAvatar(stagedToken, query.data ?? {});",
		],
	],
	["lib/hooks/useSocial.ts", ["records: [],"]],
	["lib/hooks/useSellerForum.ts", ["records: [],"]],
]);

function withoutMemoCallbacks(source: string): string {
	const out = source.split("");
	const open = /\buseMemo\(/g;
	for (let m = open.exec(source); m; m = open.exec(source)) {
		let depth = 1;
		let i = open.lastIndex;
		for (; i < source.length && depth > 0; i++) {
			if (source[i] === "(") depth++;
			else if (source[i] === ")") depth--;
		}
		for (let j = m.index; j < i; j++) if (out[j] !== "\n") out[j] = " ";
		open.lastIndex = i;
	}
	return out.join("");
}

function hookFiles(dir: string): string[] {
	return readdirSync(join(ROOT, dir), { withFileTypes: true }).flatMap((e) => {
		const path = join(dir, e.name);
		if (e.isDirectory()) return hookFiles(path);
		const isHook = /(^|\/)use[A-Z][^/]*\.tsx?$/.test(path);
		return isHook && !/\.(unit\.)?test\.tsx?$/.test(path) ? [path] : [];
	});
}

test("hooks never return a per-render fresh empty array or object", () => {
	const files = [
		...hookFiles("lib/hooks"),
		...hookFiles("features/delegated-operations"),
	];
	assert.ok(files.length > 20);
	const offenders: string[] = [];
	for (const file of files) {
		const rel = relative(ROOT, join(ROOT, file));
		const allowed = ALLOWED.get(rel) ?? [];
		withoutMemoCallbacks(readFileSync(join(ROOT, file), "utf8"))
			.split("\n")
			.forEach((line, i) => {
				const text = line.trim();
				if (text.includes("placeholderData")) return;
				if (FRESH_EMPTY.test(text) && !allowed.includes(text)) {
					offenders.push(`${rel}:${i + 1}: ${text}`);
				}
			});
	}
	assert.deepEqual(
		offenders,
		[],
		`A \`?? []\`, \`|| []\`, \`: []\` or \`: {}\` fallback in a hook's return path creates a new reference on every render, so an effect or memo that depends on the hook result re-runs forever (the loop fixed in c84faa6). Derive the value with useMemo keyed on the query data, e.g. \`useMemo(() => (enabled && query.data) || [], [enabled, query.data])\`; a fallback inside the useMemo callback is fine.`,
	);
});
