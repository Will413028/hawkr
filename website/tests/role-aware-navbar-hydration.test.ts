import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const navbarPath = new URL(
	"../components/shared/RoleAwareNavbar.tsx",
	import.meta.url,
);

test("seller navbar keeps SSR and the first client render deterministic", async () => {
	const source = await readFile(navbarPath, "utf8");

	assert.match(
		source,
		/if \(\(isSellerOnboardingRoute \|\| isSellerWorkspaceRoute\) && !mounted\)/,
	);
	assert.match(
		source,
		/<div className="root-nav-layout-reservation" aria-hidden="true" \/>/,
	);
	assert.match(
		source,
		/const isSellerWorkspaceRoute =\s*effectivePathname\.startsWith\("\/seller-center"\)/,
	);
	assert.match(source, /const effectivePathname = pathname \?\? ""/);
	assert.doesNotMatch(source, /window\.location\.pathname/);
});
