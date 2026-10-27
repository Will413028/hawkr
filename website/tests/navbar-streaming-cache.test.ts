import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const rootLayoutPath = new URL(
	"../components/shared/RootDocument.tsx",
	import.meta.url,
);
const navbarPath = new URL("../components/shared/Navbar.tsx", import.meta.url);
const sellerMessagesPath = new URL(
	"../features/sellers/messages/SellerMessagesPage.tsx",
	import.meta.url,
);

test("navbar stays out of request-time root streaming boundaries", async () => {
	const [layout, navbar, panel] = await Promise.all([
		readFile(rootLayoutPath, "utf8"),
		readFile(navbarPath, "utf8"),
		readFile(
			new URL("../components/shared/MegaMenuPanel.tsx", import.meta.url),
			"utf8",
		),
	]);

	assert.match(layout, /<RoleAwareNavbar \/>/);
	assert.doesNotMatch(
		layout,
		/NavbarWithSessionHint|NavbarWithDynamicLinks|await cookies\(\)/,
	);
	assert.doesNotMatch(layout, /getCachedCategoryTree|getCachedPopularHashtags/);
	assert.doesNotMatch(navbar, /useQuery|fetchMegaMenuDynamicLinks/);
	assert.match(panel, /queryKeys\.navbar\.megaMenu\(\)/);
	assert.match(panel, /queryFn: fetchMegaMenuDynamicLinks/);
	assert.match(navbar, /dynamicLinks=\{megaMenuDynamicLinks\}/);
});

test("seller message metadata remains prerender-safe without a dynamic root", async () => {
	const source = await readFile(sellerMessagesPath, "utf8");

	assert.match(source, /getCachedSeller\(id\)/);
	assert.doesNotMatch(source, /fetchSeller\(id\)/);
});
