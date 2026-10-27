import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

function openingTagWithLabel(markup: string, label: string): string {
	const labelIndex = markup.indexOf(`aria-label="${label}"`);
	assert.notEqual(labelIndex, -1, `missing action labelled ${label}`);
	const start = markup.lastIndexOf("<Button", labelIndex);
	const end = markup.indexOf(">", labelIndex);
	assert.notEqual(start, -1, `missing shared Button for ${label}`);
	assert.notEqual(end, -1, `unterminated shared Button for ${label}`);
	return markup.slice(start, end + 1);
}

test("seller branding reserves upload geometry and uses the shared Upload animation", () => {
	const branding = source(
		"../features/seller/components/SellerProfileBranding.tsx",
	);
	const uploadIndicator = source(
		"../components/shared/UploadLoadingIndicator.tsx",
	);

	assert.match(
		branding,
		/import UploadLoadingIndicator from "@\/components\/shared\/UploadLoadingIndicator"/,
	);
	assert.equal(
		branding.match(/<UploadLoadingIndicator\b/g)?.length,
		5,
		"mobile/web cover, mobile/web avatar, and the edit callout must all use Upload",
	);
	assert.doesNotMatch(
		branding,
		/<Spinner\b|function Spinner|name="spinner"|animate-spin/,
	);
	assert.match(branding, /disabled=\{busy\}/);
	assert.match(branding, /aria-busy=\{busy\}/);
	assert.match(branding, /role=\{busy \? "status" : undefined\}/);
	assert.match(branding, /aria-atomic=\{busy \? "true" : undefined\}/);
	assert.match(branding, /loadingTitle="背景上傳中…"/);
	assert.match(branding, /loadingTitle="大頭貼上傳中…"/);
	assert.match(branding, /size=\{28\}[\s\S]*className="size-7 shrink-0"/);
	assert.match(
		branding,
		/size=\{22\}[\s\S]*className="size-\[22px\] shrink-0"/,
	);
	assert.match(
		branding,
		/\{busy \? \([\s\S]*<UploadLoadingIndicator[\s\S]*\) : \([\s\S]*<AppIcon name=\{icon\}/,
	);
	assert.match(branding, /className="grid overflow-hidden[^"]*"/);
	assert.match(uploadIndicator, /name="upload"/);
	assert.match(uploadIndicator, /motion-reduce:animate-none/);
});

test("seller share and download keep the initiating action identity", () => {
	const share = source("../features/seller/components/SellerShareButton.tsx");
	const shareButton = openingTagWithLabel(share, "分享名片");
	const downloadButton = openingTagWithLabel(share, "下載名片圖");

	assert.match(share, /import Button from "@\/components\/ui\/Button"/);
	assert.match(share, /type SellerSharePendingAction = "share" \| "download"/);
	assert.match(share, /const exporting = pendingAction !== null/);
	assert.match(share, /setPendingAction\("share"\)/);
	assert.match(share, /setPendingAction\("download"\)/);
	assert.equal(share.match(/setPendingAction\(null\)/g)?.length, 2);

	assert.match(shareButton, /disabled=\{exporting\}/);
	assert.match(shareButton, /isLoading=\{pendingAction === "share"\}/);
	assert.match(shareButton, /loadingLabel="分享名片中…"/);
	assert.doesNotMatch(shareButton, /pendingAction === "download"/);

	assert.match(downloadButton, /disabled=\{exporting\}/);
	assert.match(downloadButton, /isLoading=\{pendingAction === "download"\}/);
	assert.match(downloadButton, /loadingLabel="下載名片中…"/);
	assert.doesNotMatch(downloadButton, /pendingAction === "share"/);

	assert.equal(share.match(/<Button\b/g)?.length, 2);
	assert.doesNotMatch(share, /setExporting|exporting \? "產生中"/);
	assert.match(
		share,
		/<button[\s\S]*aria-label="複製商店網址"[\s\S]*onClick=\{copyUrl\}/,
	);
});

test("seller share actions use three equal, centered columns", () => {
	const styles = source("../styles/features/seller-share-card.css");
	const actions = styles.match(/\.seller-share-actions \{([^}]*)\}/);
	const item = styles.match(/\.seller-share-actions__item \{([^}]*)\}/);

	assert.ok(actions && item, "missing seller share action layout rules");
	assert.match(actions[1], /display:\s*grid/);
	assert.match(
		actions[1],
		/grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/,
	);
	assert.match(actions[1], /align-items:\s*stretch/);
	assert.match(item[1], /width:\s*100%/);
	assert.match(item[1], /min-width:\s*0/);
	assert.match(item[1], /min-height:\s*76px/);
	assert.match(item[1], /justify-content:\s*center/);
});

test("seller share card keeps comfortable spacing around its QR details", () => {
	const card = source("../features/seller/components/SellerBusinessCard.tsx");
	const styles = source("../styles/features/seller-share-card.css");
	const footer = styles.match(/\.seller-business-card__footer \{([^}]*)\}/);
	const storeId = styles.match(/\.seller-business-card__store-id \{([^}]*)\}/);

	assert.ok(footer && storeId, "missing seller share card footer layout rules");
	assert.match(footer[1], /gap:\s*var\(--hkui-space-3\)/);
	assert.match(footer[1], /padding-top:\s*var\(--hkui-space-4\)/);
	assert.match(storeId[1], /font-variant-numeric:\s*tabular-nums/);
	assert.doesNotMatch(card, /shareUrlLabel|seller-business-card__link/);
});

test("seller title clears the cover and Chinese dialog copy has no stray gap", () => {
	const share = source("../features/seller/components/SellerShareButton.tsx");
	const styles = source("../styles/features/seller-share-card.css");
	const identity = styles.match(/\.seller-business-card__identity \{([^}]*)\}/);

	assert.ok(identity, "missing seller share card identity layout rules");
	assert.match(identity[1], /padding-bottom:\s*0/);
	assert.match(share, /title=\{`\$\{data\.name\}的商家名片`\}/);
	assert.doesNotMatch(share, /title=\{`\$\{data\.name\} 的商家名片`\}/);
});

test("seller share card fits the canonical 4:1 cover in preview and PNG export", () => {
	const card = source("../features/seller/components/SellerBusinessCard.tsx");
	const client = source("../lib/seller/share-card-client.ts");
	const styles = source("../styles/features/seller-share-card.css");
	const baseCard = styles.match(/\.seller-business-card \{([^}]*)\}/);
	const cover = styles.match(/\.seller-business-card__cover \{([^}]*)\}/);
	const compactCard = styles.match(
		/\.seller-business-card--compact \{([^}]*)\}/,
	);

	assert.ok(baseCard && cover && compactCard, "missing seller card geometry");
	assert.match(
		card,
		/import \{ SELLER_COVER_ASPECT \} from "@\/lib\/media\/seller-cover"/,
	);
	assert.match(card, /style=\{\{ aspectRatio: SELLER_COVER_ASPECT \}\}/);
	assert.doesNotMatch(card, /mode="ambient-contain"/);
	assert.doesNotMatch(cover[1], /height:\s*184px/);
	assert.match(baseCard[1], /min-height:\s*460px/);
	assert.match(compactCard[1], /min-height:\s*420px/);
	assert.match(
		client,
		/import \{ SELLER_COVER_ASPECT \} from "@\/lib\/media\/seller-cover"/,
	);
	assert.match(
		client,
		/const CARD_COVER_HEIGHT = CARD_WIDTH \/ SELLER_COVER_ASPECT/,
	);
	assert.match(client, /const CARD_HEIGHT = CARD_COVER_HEIGHT \+ 1210/);
	assert.match(client, /const height = CARD_COVER_HEIGHT/);
	assert.doesNotMatch(client, /const height = 470/);
	assert.doesNotMatch(client, /drawImageContain|filter = "blur/);
	assert.match(card, /value=\{data\.shareUrl\}/);
	assert.doesNotMatch(client, /data\.shareUrl\.replace|wrapText\(ctx, url/);
	assert.match(client, /`商店 ID \$\{data\.displayId\}`/);
	assert.match(client, /掃描 QR code 開啟一頁式名片/);
});
