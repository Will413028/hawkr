import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const appToasterPath = new URL(
	"../components/shared/AppToaster.tsx",
	import.meta.url,
);

test("Sonner cannot mutate document head before root hydration", async () => {
	const source = await readFile(appToasterPath, "utf8");

	assert.doesNotMatch(
		source,
		/import\s+\{\s*Toaster\s*\}\s+from\s+["']sonner["']/,
	);
	assert.match(source, /React\.useEffect\(\(\) => \{/);
	assert.match(source, /whenDocumentSettled\(\)/);
	assert.match(source, /\.then\(\(\) => import\("sonner"\)\)/);
	assert.match(source, /requestAnimationFrame\(markToastViewportReady\)/);
	assert.match(source, /if \(!Toaster\) return null/);
});

test("route chunks cannot statically evaluate Sonner during hydration", async () => {
	const sourceRoots = ["app", "components", "features", "lib"];
	const staticImports: string[] = [];

	const visit = async (directory: URL): Promise<void> => {
		for (const entry of await readdir(directory, { withFileTypes: true })) {
			const target = new URL(
				`${entry.name}${entry.isDirectory() ? "/" : ""}`,
				directory,
			);
			if (entry.isDirectory()) {
				await visit(target);
				continue;
			}
			if (!/\.[cm]?[jt]sx?$/.test(entry.name)) continue;

			const source = await readFile(target, "utf8");
			for (const [index, line] of source.split("\n").entries()) {
				if (
					/\bfrom\s+["']sonner["']/.test(line) &&
					!/\bimport\s+type\b/.test(line)
				) {
					staticImports.push(`${target.pathname}:${index + 1}`);
				}
			}
		}
	};

	for (const root of sourceRoots) {
		await visit(new URL(`../${root}/`, import.meta.url));
	}

	assert.deepEqual(staticImports, []);
});
