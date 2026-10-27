import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
	isStandaloneFlowPath,
	isStandaloneFlowTransition,
} from "../lib/navigation/standalone-flow";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("seller onboarding is a focused standalone flow", () => {
	assert.equal(isStandaloneFlowPath("/seller/onboarding"), true);
	assert.equal(isStandaloneFlowPath("/seller/onboarding/complete"), true);
	assert.equal(isStandaloneFlowPath("/seller"), false);
	assert.equal(isStandaloneFlowPath("/seller-center/dashboard"), false);
});

test("entering or leaving seller onboarding bypasses layered route transitions", () => {
	assert.equal(
		isStandaloneFlowTransition(
			"/seller-center/dashboard",
			"/seller/onboarding",
		),
		true,
	);
	assert.equal(isStandaloneFlowTransition("/seller/onboarding", "/"), true);
	assert.equal(
		isStandaloneFlowTransition("/seller-center/dashboard", "/account"),
		false,
	);

	const transitionController = source(
		"../components/shared/PageTransition.tsx",
	);
	assert.match(transitionController, /isStandaloneFlowTransition/);
	assert.match(transitionController, /stampViewTransitionType\("none"\)/);
});

test("seller onboarding keeps the seller navbar with logout", () => {
	const navbar = source("../components/shared/RoleAwareNavbar.tsx");
	const sellerNavbar = source("../components/shared/SellerNavbar.tsx");
	const sellerNavbarLoader = source(
		"../components/shared/SellerNavbarLoader.tsx",
	);
	assert.match(
		navbar,
		/const isSellerOnboardingRoute = isStandaloneFlowPath\(effectivePathname\)/,
	);
	assert.match(navbar, /isSellerOnboardingRoute \|\|\s*isSellerWorkspaceRoute/);
	assert.match(
		navbar,
		/<SellerHeaderLoader onboarding=\{isSellerOnboardingRoute\} \/>/,
	);
	assert.match(
		sellerNavbar,
		/data-onboarding=\{isStandaloneFlowPath\(pathname\) \|\| undefined\}/,
	);
	assert.match(sellerNavbar, /data-testid="logout-button"/);
	assert.match(sellerNavbarLoader, /data-seller-navbar-logout/);
	assert.doesNotMatch(
		navbar,
		/if \(isStandaloneFlowPath\(effectivePathname\)\) return null/,
	);

	const navigationShell = source("../styles/components/navigation-shell.css");
	assert.match(
		navigationShell,
		/@media \(max-width: 640px\)[\s\S]*seller-header\[data-onboarding="true"\][\s\S]*seller-header__nav[\s\S]*display:\s*none/,
	);
});

test("seller onboarding keeps footer and mobile chrome off", () => {
	for (const file of [
		"../components/shared/MobileBottomNav.tsx",
		"../components/shared/RoleAwareFooter.tsx",
		"../components/shared/RoleSwitcher.tsx",
	]) {
		assert.match(source(file), /isStandaloneFlowPath/);
	}
});

test("seller onboarding owns compact mobile-safe spacing", () => {
	const onboarding = source(
		"../features/seller/onboarding/SellerOnboardingClient.tsx",
	);

	assert.match(onboarding, /min-h-\[100dvh\]/);
	assert.match(onboarding, /pt-6 sm:px-5 sm:pb-16 sm:pt-12/);
	assert.match(onboarding, /size-\[88px\].*sm:size-\[112px\]/);
	assert.match(onboarding, /rounded-\[28px\].*p-5/);
	assert.match(onboarding, /SELLER · 自助開店/);
	assert.match(onboarding, /建立你的賣場/);
	assert.match(onboarding, /loadingLabel="正在建立賣場…"/);
	assert.match(onboarding, /: "建立賣場"/);
	assert.match(onboarding, /賣場資料已建立/);
});

test("seller onboarding shows the Hawkr mark above the form", () => {
	const onboarding = source(
		"../features/seller/onboarding/SellerOnboardingClient.tsx",
	);

	assert.match(
		onboarding,
		/<span\s+role="img"\s+aria-label="Hawkr"[\s\S]*<BrandLogoLoader size="fit" \/>/,
	);
	assert.doesNotMatch(onboarding, /<AppIcon name="send"/);
	assert.match(onboarding, /data-standalone-flow="seller-onboarding"/);
});

test("seller onboarding drops stale profile cache before entering the workspace", () => {
	const onboarding = source(
		"../features/seller/onboarding/SellerOnboardingClient.tsx",
	);
	const documentReplacements =
		onboarding.match(/commitDocumentNavigation\(redirect, true\)/g) ?? [];

	// One replacement admits an already-completed seller; the other exits right
	// after a successful application. Both must start a fresh QueryClient so an
	// old needs_onboarding=true/404 result cannot bounce the seller back here.
	assert.equal(documentReplacements.length, 2);
	assert.doesNotMatch(onboarding, /router\.replace\(redirect\)/);
});

test("onboarding keeps the typed address without calling address validation", () => {
	const onboarding = source(
		"../features/seller/onboarding/SellerOnboardingClient.tsx",
	);

	assert.doesNotMatch(onboarding, /validateAddress|AddressAutocompleteField/);
	assert.doesNotMatch(onboarding, /已略過地址驗證|addrVerif/);
	assert.match(
		onboarding,
		/address: townCode\s*\? \{ townCode, addressDetail: addressDetail\.trim\(\) \}\s*: undefined/,
	);
});
