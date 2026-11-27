import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import test from "node:test";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

const FRAME =
	"../features/seller-center/preview/SellerStorefrontPreviewFrame.tsx";
const PREVIEW_APP = "../app/[locale]/(seller)/seller-center/preview";

test("seller preview reuses the buyer profile shell and reads the owner's own store", () => {
	const layout = source(`${PREVIEW_APP}/layout.tsx`);
	const frame = source(FRAME);
	const previewBanner = source(
		"../features/seller-center/preview/SellerStorefrontPreviewBanner.tsx",
	);
	const products = source(`${PREVIEW_APP}/page.tsx`);

	assert.doesNotMatch(layout, /^"use client";/m);
	assert.match(layout, /<SellerStorefrontPreviewFrame>/);
	assert.match(frame, /^"use client";/m);
	assert.match(frame, /useSellerSelf\(\)/);
	assert.match(
		frame,
		/<h1 className="sr-only">買家視角預覽<\/h1>[\s\S]*<SellerStorefrontPreviewBanner canBePublic=\{canBePublic\} \/>/,
	);
	assert.match(previewBanner, /^"use client";/m);
	assert.doesNotMatch(previewBanner, /<h1\b/);
	assert.match(frame, /<SellerProfileV2[\s\S]*previewMode/);
	assert.match(frame, /tabBasePath=\{PREVIEW_ROOT\}/);
	assert.match(frame, /PREVIEW_ROOT = "\/seller-center\/preview"/);
	assert.match(frame, /<StorefrontPreviewBoundary>/);
	assert.match(products, /<PreviewProductsTab \/>/);
	assert.doesNotMatch(frame, /SellerProfileBranding/);
});

test("seller preview exposes the same four buyer storefront tabs without a seller id segment", () => {
	const tabs = source("../features/seller/components/SellerProfileTabs.tsx");
	const posts = source(`${PREVIEW_APP}/posts/page.tsx`);
	const messages = source(`${PREVIEW_APP}/messages/page.tsx`);
	const reviews = source(`${PREVIEW_APP}/reviews/page.tsx`);

	assert.match(tabs, /basePath \?\? `\/sellers\/\$\{sellerId\}`/);
	assert.match(posts, /<PreviewPostsTab \/>/);
	assert.match(messages, /<PreviewMessagesTab \/>/);
	assert.match(reviews, /<PreviewReviewsTab \/>/);
	assert.equal(
		existsSync(new URL(`${PREVIEW_APP}/[id]`, import.meta.url)),
		false,
	);
});

test("storefront chat uses the internal store id rather than the route code", () => {
	const profile = source("../features/seller/components/SellerProfileV2.tsx");

	assert.match(profile, /const chatSellerId = resolveSellerChatTargetId\(s\);/);
	assert.match(
		profile,
		/\{chatSellerId \? \([\s\S]*?<SellerChatButton[\s\S]*?sellerId=\{chatSellerId\}/,
	);
	assert.doesNotMatch(
		profile,
		/<SellerChatButton\s+sellerId=\{sellerId\}/,
		"canonical route storeCode must not become the chat room key",
	);
});

test("seller preview loading state uses the shared truck mark", () => {
	const frame = source(FRAME);

	assert.match(frame, /import \{ TruckMark \}/);
	assert.match(frame, /<TruckMark/);
	assert.match(frame, /正在載入店面預覽…/);
	assert.match(frame, /role="status"/);
});

test("seller preview never reads through the shared public caches or the removed seller endpoints", () => {
	const dir = new URL("../features/seller-center/preview/", import.meta.url);
	const files = [
		...readdirSync(dir).map((name) => new URL(name, dir)),
		new URL("../lib/seller/storefront-preview.ts", import.meta.url),
		new URL("../lib/seller/preview-posts.ts", import.meta.url),
	];
	for (const file of files) {
		const text = readFileSync(file, "utf8");
		assert.doesNotMatch(
			text,
			/getCached\w+|unstable_cache|fetchSeller\b|fetchSellerStats|fetchPublicProduct/,
			file.pathname,
		);
	}
});

test("seller sidebar and buyer surfaces both draw the preview icon through the shared registry", () => {
	const icons = source("../components/shared/AppIcon.tsx");
	const sidebarIcons = source("../components/shared/SellerSidebarIcon.tsx");
	const shell = [
		source("../features/seller-center/components/SellerCenterShell.tsx"),
		source("../features/seller-center/components/sellerNav.tsx"),
	].join("\n");
	const buyerPreview = source(FRAME);
	const buyerPreviewBanner = source(
		"../features/seller-center/preview/SellerStorefrontPreviewBanner.tsx",
	);
	const brandingPreview = source(
		"../features/seller/components/SellerProfileBranding.tsx",
	);

	assert.match(icons, /preview: "preview"/);
	assert.match(icons, /from "@hawkr\/icons\/material-symbol"/);
	assert.match(
		shell,
		/label: "商店預覽",[\s\S]*<SellerSidebarIcon name="preview"/,
	);
	assert.doesNotMatch(sidebarIcons, /\bd="M/);
	assert.match(buyerPreview, /<SellerStorefrontPreviewBanner/);
	assert.match(buyerPreviewBanner, /<AppIcon name="preview" size=\{18\}/);
	assert.match(brandingPreview, /<AppIcon name="preview" size=\{20\}/);
	assert.doesNotMatch(shell, /function IconPreview/);
	assert.doesNotMatch(
		`${buyerPreview}\n${buyerPreviewBanner}`,
		/function PreviewEyeIcon/,
	);
	assert.doesNotMatch(brandingPreview, /function PreviewWindowIcon/);
});

test("preview mode blocks buyer write actions before handlers run", () => {
	const boundary = source(
		"../features/seller-center/preview/StorefrontPreviewBoundary.tsx",
	);
	const chat = source("../features/seller/components/SellerChatButton.tsx");
	const subscribe = source(
		"../features/seller/components/SellerSubscribeButton.tsx",
	);

	assert.match(boundary, /onClickCapture=\{blockClick\}/);
	assert.match(boundary, /onSubmitCapture=\{blockSubmit\}/);
	assert.match(chat, /if \(previewMode\) return;/);
	assert.match(subscribe, /if \(previewMode\) return;/);
});

test("product and post cards open readonly previews while write actions stay blocked", () => {
	const boundary = source(
		"../features/seller-center/preview/StorefrontPreviewBoundary.tsx",
	);
	const drawer = source(
		"../features/seller-center/preview/StorefrontProductPreviewDrawer.tsx",
	);
	const postCard = source("../features/feed/components/PostCardV2.tsx");
	const postViewer = source("../features/feed/components/PostDetailViewer.tsx");
	const actionFooterStart = postViewer.indexOf(
		'<footer className="post-viewer__actions">',
	);
	const actionFooterEnd = postViewer.indexOf("</footer>", actionFooterStart);
	assert.notEqual(actionFooterStart, -1);
	assert.notEqual(actionFooterEnd, -1);
	const writeActions = postViewer.slice(actionFooterStart, actionFooterEnd);

	assert.match(boundary, /data-storefront-product-preview-trigger="true"/);
	assert.match(boundary, /data-storefront-post-preview-trigger="true"/);
	assert.match(boundary, /data-storefront-post-preview-control="true"/);
	assert.match(boundary, /isReadonlyPostPreviewInteraction\(interactive\)/);
	assert.match(boundary, /event\.preventDefault\(\)/);
	assert.match(boundary, /event\.stopPropagation\(\)/);
	assert.match(postCard, /data-storefront-post-preview-trigger="true"/);
	assert.match(postCard, /aria-haspopup="dialog"/);
	assert.match(postCard, /if \(e\.target !== e\.currentTarget\) return;/);
	assert.match(
		postViewer,
		/data-storefront-post-preview-control="true"[\s\S]*className="post-viewer__close"/,
	);
	assert.match(
		postViewer,
		/data-storefront-post-preview-control="true"[\s\S]*className="post-viewer__nav post-viewer__nav--next"/,
	);
	assert.doesNotMatch(
		writeActions,
		/data-storefront-post-preview-control="true"/,
	);
	assert.match(boundary, /productIdFromStorefrontPreviewHref/);
	assert.match(boundary, /aria-expanded", "true"/);
	assert.match(boundary, /<StorefrontProductPreviewDrawer/);
	assert.match(drawer, /fetchSellerProductEdit\(productId\)/);
	assert.doesNotMatch(
		drawer,
		/fetchPublicProduct|fetchSeller\b|fetchSellerStats/,
	);
	assert.match(drawer, /<ProductDetailLayout[\s\S]*previewMode/);
	assert.match(drawer, /<ProductSellerIntro[\s\S]*previewMode/);
	assert.doesNotMatch(drawer, /ProductModal|ProductEditDrawer/);
});

test("seller back matches product detail while mobile CTAs keep fixed geometry", () => {
	const profile = source("../features/seller/components/SellerProfileV2.tsx");
	const productBack = source(
		"../features/product/components/ProductBackButton.tsx",
	);
	const compactBack = source("../components/shared/CompactBackControl.tsx");
	const subscribeButton = source(
		"../features/seller/components/SellerSubscribeButton.tsx",
	);
	const sellerProfileStyles = source("../styles/features/seller-profile.css");
	const mobileStart = sellerProfileStyles.indexOf("@media (max-width: 640px)");
	const mobileEnd = sellerProfileStyles.indexOf(
		"@media (max-width: 340px)",
		mobileStart,
	);
	assert.notEqual(mobileStart, -1);
	assert.notEqual(mobileEnd, -1);
	const desktop = sellerProfileStyles.slice(0, mobileStart);
	const mobile = sellerProfileStyles.slice(mobileStart, mobileEnd);

	assert.match(profile, /hkui-seller-profile__cta--chat/);
	assert.match(profile, /hkui-seller-profile__cta--card/);
	assert.match(
		profile,
		/<BackLink[\s\S]*fallbackHref="\/"[\s\S]*className=\{`\$\{COMPACT_BACK_CONTROL_CLASSNAME\} hkui-seller-profile__back`\}[\s\S]*data-hkui-button=""/,
	);
	assert.match(profile, /<CompactBackIcon \/>/);
	assert.match(
		productBack,
		/data-zoom-back=""[\s\S]*className=\{COMPACT_BACK_CONTROL_CLASSNAME\}[\s\S]*<CompactBackIcon \/>/,
	);
	assert.match(
		compactBack,
		/h-11 min-w-11 justify-center pl-2\.5 pr-3\.5 rounded-hk-pill border border-hk-border-soft bg-hk-surface-glass text-hk-body text-sm font-semibold/,
	);
	assert.match(
		compactBack,
		/width="18"[\s\S]*height="18"[\s\S]*strokeWidth="2\.2"[\s\S]*<path d="m15 18-6-6 6-6"/,
	);
	assert.match(subscribeButton, /name="notification"[\s\S]*size=\{18\}/);
	assert.match(
		mobile,
		/--hkui-seller-profile-mobile-action-width:\s*clamp\([\s\S]*60px[\s\S]*100cqw[\s\S]*80px/,
	);
	assert.match(
		mobile,
		/--hkui-seller-profile-mobile-head-cover-offset:\s*38px[\s\S]*--hkui-seller-profile-mobile-cover-action-gap:\s*var\(--hkui-space-2\)/,
	);
	assert.match(
		mobile,
		/\.hkui-seller-profile__head\s*\{[^}]*margin-top:\s*calc\([\s\S]*0px - var\(--hkui-seller-profile-mobile-head-cover-offset\)[\s\S]*\)/s,
	);
	assert.doesNotMatch(mobile, /\.hkui-seller-profile__back\s*\{/);
	assert.match(
		mobile,
		/grid-template-columns:\s*repeat\([\s\S]*var\(--hkui-seller-profile-mobile-action-width\)[\s\S]*\)/,
	);
	assert.match(
		mobile,
		/\.hkui-seller-profile__actions\s*\{[^}]*margin-block-start:\s*calc\([\s\S]*var\(--hkui-seller-profile-mobile-head-cover-offset\)[\s\S]*var\(--hkui-seller-profile-mobile-cover-action-gap\)[\s\S]*\)/s,
	);
	assert.match(
		mobile,
		/\.hkui-seller-profile__cta\s*\{[^}]*height:\s*44px[^}]*max-height:\s*44px[^}]*overflow:\s*hidden[^}]*border-radius:\s*var\(--hkui-radius-full\)/s,
	);
	assert.match(
		mobile,
		/\.hkui-seller-profile__cta:focus-visible\s*\{[^}]*outline:\s*2px solid var\(--hkui-focus\)/s,
	);
	assert.match(
		mobile,
		/--hkui-seller-profile-mobile-action-visual-inline-inset:\s*4px/,
	);
	assert.match(
		mobile,
		/\.hkui-seller-profile \.hkui-seller-profile__cta::before\s*\{[^}]*z-index:\s*-1[^}]*inset-block:\s*2px[^}]*inset-inline:\s*var\([\s\S]*--hkui-seller-profile-mobile-action-visual-inline-inset[\s\S]*\)[^}]*background:\s*var\(--hkui-seller-profile-mobile-action-background\)[^}]*pointer-events:\s*none/s,
	);
	assert.match(mobile, /\.hkui-seller-profile__cta--chat\s*\{/);
	assert.match(mobile, /\.hkui-seller-profile__cta--card\s*\{/);
	assert.match(
		mobile,
		/\.hkui-seller-profile \.hkui-seller-profile__cta\[aria-busy="true"\]\s*\{[^}]*--hkui-seller-profile-mobile-action-background:\s*var\(--hkui-default-300\)[^}]*clip-path:\s*inset\([\s\S]*2px var\(--hkui-seller-profile-mobile-action-visual-inline-inset\)[\s\S]*var\(--hkui-radius-full\)[\s\S]*\)/s,
	);
	const mobileProfileCta = mobile.match(
		/\.hkui-seller-profile \.hkui-seller-profile__cta\s*\{([^}]*)\}/s,
	)?.[1];
	assert.ok(mobileProfileCta);
	assert.match(mobileProfileCta, /transition:[\s\S]*color[\s\S]*transform/);
	assert.doesNotMatch(mobileProfileCta, /transition:[\s\S]*background/);
	assert.match(
		sellerProfileStyles,
		/@media \(max-width: 340px\)[\s\S]*--hkui-seller-profile-mobile-action-width:\s*60px[\s\S]*--hkui-seller-profile-mobile-action-visual-inline-inset:\s*1px[\s\S]*padding-inline:\s*var\(--hkui-space-2\)[\s\S]*padding-inline:\s*0/,
	);
	assert.match(desktop, /\.hkui-seller-profile__cta\s*\{[^}]*height:\s*36px/s);
	const sellerBackPosition = desktop.match(
		/\.hkui-seller-profile__back\s*\{([^}]*)\}/s,
	)?.[1];
	assert.ok(sellerBackPosition);
	assert.match(sellerBackPosition, /position:\s*absolute/);
	assert.doesNotMatch(
		sellerBackPosition,
		/(?:height|padding|background|border-radius):/,
	);
	assert.match(
		desktop,
		/\.hkui-seller-profile__subscribe \.hkui-seller-profile__cta\s*\{[^}]*min-width:\s*104px/s,
	);
});

test("storefront App banner keeps mobile copy intact with usable actions", () => {
	const uiKit = source("../styles/components/ui-kit.css");
	const bannerStart = uiKit.indexOf("14. APP DOWNLOAD BANNER V2");
	assert.notEqual(bannerStart, -1);
	const banner = uiKit.slice(bannerStart);
	const mobileStart = banner.indexOf("@media (max-width: 640px)");
	assert.notEqual(mobileStart, -1);
	const mobile = banner.slice(mobileStart);

	assert.match(
		mobile,
		/\.hkui-app-banner\s*\{[^}]*gap:\s*var\(--hkui-space-1\)[^}]*padding-inline:\s*var\(--hkui-space-1_5\)/s,
	);
	assert.match(
		mobile,
		/\.hkui-app-banner__copy\s*\{[^}]*flex-direction:\s*column[^}]*align-items:\s*flex-start[^}]*gap:\s*0/s,
	);
	assert.match(
		mobile,
		/\.hkui-app-banner__copy span\s*\{[^}]*white-space:\s*nowrap/s,
	);
	assert.match(
		mobile,
		/\.hkui-app-banner \.hkui-app-banner__cta\s*\{[^}]*position:\s*relative[^}]*min-width:\s*max-content[^}]*min-height:\s*36px[^}]*height:\s*36px[^}]*white-space:\s*nowrap/s,
	);
	assert.match(
		mobile,
		/\.hkui-app-banner__cta::before\s*\{[^}]*content:\s*""[^}]*position:\s*absolute[^}]*inset-block:\s*-4px[^}]*inset-inline:\s*0/s,
	);
	assert.match(
		mobile,
		/\.hkui-app-banner__dismiss\s*\{[^}]*width:\s*44px[^}]*height:\s*44px/s,
	);

	const narrowStart = banner.indexOf("@media (max-width: 360px)");
	assert.notEqual(narrowStart, -1);
	const narrow = banner.slice(narrowStart);
	assert.match(
		narrow,
		/\.hkui-app-banner\s*\{[^}]*display:\s*grid[^}]*grid-template-columns:\s*28px minmax\(0, 1fr\) auto[^}]*row-gap:\s*0/s,
	);
	assert.match(narrow, /\.hkui-app-banner__copy\s*\{[^}]*display:\s*contents/s);
	assert.match(
		narrow,
		/\.hkui-app-banner__copy span\s*\{[^}]*grid-column:\s*2 \/ -1[^}]*grid-row:\s*2[^}]*max-width:\s*100%/s,
	);
});
