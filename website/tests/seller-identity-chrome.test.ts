import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("seller identity chrome uses one avatar presentation contract", () => {
	const avatar = source("../components/shared/SellerAvatarImage.tsx");
	const roleOwner = source("../components/shared/RoleAwareNavbar.tsx");
	const fallbackHeader = source("../components/shared/Navbar.tsx");
	const sellerChrome = source("../components/shared/SellerNavbar.tsx");
	const sidebar = source(
		"../features/seller-center/components/SellerCenterShell.tsx",
	);
	const legacyFeedStyles = source("../styles/features/feed-components.css");

	assert.match(avatar, /variant\?: "default" \| "identity"/);
	assert.match(avatar, /bg-\[var\(--hkui-primary\)\]/);
	assert.match(avatar, /fallback="empty"/);
	assert.doesNotMatch(avatar, /fallbackSrc|placeholders\.sellerAvatar/);
	assert.match(
		source("../components/shared/useSellerIdentity.ts"),
		/const avatar = storeImageUrls\(store\)\.logoUrl \?\? null/,
	);
	assert.match(
		roleOwner,
		/<SellerHeaderLoader onboarding=\{isSellerOnboardingRoute\} \/>/,
	);
	assert.match(fallbackHeader, /<SellerMenuLoader/);
	assert.equal(
		sellerChrome.match(/<SellerAvatarImage[\s\S]*?variant="identity"/g)?.length,
		2,
	);
	assert.match(sidebar, /seller-sidebar__avatar[\s\S]*?variant="identity"/);
	assert.doesNotMatch(legacyFeedStyles, /\.seller-header__avatar/);
});

test("seller identity chrome shows the public seller ID under the store name", () => {
	const identity = source("../components/shared/useSellerIdentity.ts");
	const roleOwner = source("../components/shared/RoleAwareNavbar.tsx");
	const fallbackHeader = source("../components/shared/Navbar.tsx");
	const sellerChrome = source("../components/shared/SellerNavbar.tsx");
	const sidebar = source(
		"../features/seller-center/components/SellerCenterShell.tsx",
	);

	assert.match(
		identity,
		/const sellerId = store\?\.storeCode \|\| store\?\.id \|\| ""/,
	);
	assert.match(identity, /sellerId,/);
	assert.match(
		roleOwner,
		/<SellerHeaderLoader onboarding=\{isSellerOnboardingRoute\} \/>/,
	);
	assert.match(fallbackHeader, /<SellerMenuLoader/);
	assert.equal(
		sellerChrome.match(/<span>賣家 ID：\{sellerId\}<\/span>/g)?.length,
		2,
	);
	assert.match(sidebar, /`賣家 ID：\$\{sellerId\}`/);
	assert.match(
		sidebar,
		/<small title=\{identitySubtitle\}>\{identitySubtitle\}<\/small>/,
	);
});
