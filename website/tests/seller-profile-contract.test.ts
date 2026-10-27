import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import {
	buildSellerShareCardData,
	type SellerShareSource,
} from "../lib/seller/share-card";

// The prod flavor requires its own web origin (NEXT_PUBLIC_WEB_ORIGIN); these cases describe a deployment at this one.
process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("seller share card reads only canonical logoUrl", () => {
	const canonical = buildSellerShareCardData("42", {
		logoUrl: "https://media.example.com/i/avatar/store.png",
	});
	assert.equal(
		canonical.avatarUrl,
		"https://media.example.com/i/avatar/store.png",
	);

	assert.equal(buildSellerShareCardData("42", {}).avatarUrl, "");
	assert.equal(
		buildSellerShareCardData("42", {
			logo: "https://legacy.example/logo.png",
			logo_url: "https://legacy.example/snake.png",
			avatar: "https://legacy.example/avatar.png",
		} as unknown as SellerShareSource).avatarUrl,
		"",
	);
});

test("seller share card QR and native share target the one-page card URL", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	const card = buildSellerShareCardData(
		"42",
		{ storeName: "山漾食品" },
		"2685914682",
	);

	assert.equal(card.shareUrl, "https://example.com/sellers/2685914682/card");
	assert.match(card.shareText, /\/sellers\/2685914682\/card$/);
	assert.equal(card.description, "前往 Hawkr 查看商家介紹與商品。");
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("public seller profile and touch icon keep canonical empty-logo behavior", () => {
	const profile = source("../features/seller/components/SellerProfileV2.tsx");
	const profileEditor = source(
		"../features/seller-center/profile/SellerProfileClient.tsx",
	);
	const appleIcon = source(
		"../app/[locale]/(public)/sellers/[id]/apple-icon.tsx",
	);

	assert.match(profile, /logo_url\?: string/);
	assert.match(profile, /const avatarSrc = s\.logo_url\?\.trim\(\) \|\| ""/);
	assert.match(profile, /entity="seller"[\s\S]*?fallback="empty"/);
	assert.doesNotMatch(profile, /\bs\.logoUrl\b|\bs\.avatar\b/);

	assert.match(profileEditor, /logoUrl=\{images\.logoUrl\}/);

	assert.match(
		appleIcon,
		/resolveCdnUrl\(seller\?\.logo_url\?\.trim\(\) \|\| ""\)/,
	);
	assert.doesNotMatch(appleIcon, /LogoMark|seller\?\.logoUrl|seller\?\.avatar/);
	assert.match(appleIcon, /\{avatar \? \([\s\S]*?\) : null\}/);
});

test("seller profile editor saves and renders the official website field", () => {
	const profileEditor = source(
		"../features/seller-center/profile/SellerProfileClient.tsx",
	);

	assert.match(profileEditor, /label="官方網站"/);
	assert.match(profileEditor, /value=\{data\.websiteUrl\}/);
	assert.match(profileEditor, /update\(\{ websiteUrl: e\.target\.value \}\)/);
	assert.match(profileEditor, /normalizeWebsiteUrl/);
	assert.match(profileEditor, /isValidWebsiteUrl/);
});

test("profile save expires both public seller cache identities", () => {
	const profileEditor = source(
		"../features/seller-center/profile/SellerProfileClient.tsx",
	);
	const action = source("../features/seller-center/profile/actions.ts");

	assert.match(profileEditor, /revalidateOwnSellerProfile\(\{/);
	assert.match(profileEditor, /accountId: serverStore\.id/);
	assert.match(profileEditor, /storeCode: serverStore\.storeCode/);
	assert.match(action, /\(await auth\(\)\)\.userId/);
	assert.match(action, /revalidateTag\(sellerTag\(id\), \{ expire: 0 \}\)/);
});

/**
 * 「已認證」與細分狀態各有一個來源（`GetMyStore.capabilities` 的 `PUBLIC_LISTING`、`GetMyKYC.flow`），閘門只讀這兩個：
 * 送出的回應（最新的申請）必須落在閘門讀的那個 query，不是店家資料。
 */
test("the submitted application lands on the KYC query the gate reads", () => {
	const hooks = source("../lib/hooks/useSellerSelf.ts");
	assert.match(
		hooks,
		/useSetMyKyc[\s\S]*?setQueryData\(queryKeys\.seller\.kyc\(identity\), application\)/,
		"useSetMyKyc must write the KYC application query",
	);
	assert.doesNotMatch(
		hooks,
		/overallCertificationStatus|overall_certification_status/,
		"the retired store field is not a status source any more",
	);
	const client = source(
		"../features/seller-center/profile/certifications/CertificationsClient.tsx",
	);
	assert.match(client, /setMyKyc\(submitted\)/);
});

test("FoodRegSupplement writes only through the three food registration commands", () => {
	const supplementSource = source(
		"../features/seller-center/components/FoodRegSupplement.tsx",
	);
	for (const command of [
		"submitFoodRegistration",
		"withdrawFoodRegistration",
		"clearFoodRegistration",
	]) {
		assert.ok(
			supplementSource.includes(command),
			`FoodRegSupplement must call ${command}`,
		);
	}
	assert.ok(
		!/updateMyStore|foodBusiness/.test(supplementSource),
		"food registration is no longer a store profile field",
	);
});
