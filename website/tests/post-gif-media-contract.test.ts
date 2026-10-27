import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import * as cdnModule from "../lib/media/cdn";
import { isGifUploadFile, normalizeGifUploadFile } from "../lib/media/gif";
import {
	cropAnimatedGifFile,
	GIF_CROP_MAX_BYTES,
	GIF_CROP_MAX_EDGE,
	GifCropError,
} from "../lib/media/gif-crop";

type GifMediaContract = {
	isGifMedia: (value: string | null | undefined) => boolean;
	postCardPresetForMedia: (
		value: string | null | undefined,
		stillPreset?: cdnModule.ImagePreset,
	) => cdnModule.ImagePreset;
};

const cdn = cdnModule as typeof cdnModule & GifMediaContract;

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

function between(text: string, startMarker: string, endMarker: string): string {
	const start = text.indexOf(startMarker);
	const end = text.indexOf(endMarker, start + startMarker.length);
	assert.notEqual(start, -1, `missing start marker: ${startMarker}`);
	assert.notEqual(end, -1, `missing end marker: ${endMarker}`);
	return text.slice(start, end);
}

test("GIF detection uses the key or URL pathname extension only", () => {
	assert.equal(typeof cdn.isGifMedia, "function");
	assert.equal(cdn.isGifMedia("public/post/hero.gif"), true);
	assert.equal(cdn.isGifMedia(" public/post/hero.GIF "), true);
	assert.equal(
		cdn.isGifMedia(
			"https://media.example.com/i/original/public/post/hero.GiF?v=2#frame",
		),
		true,
	);
	assert.equal(cdn.isGifMedia("public/post/hero.gif?rev=2#frame"), true);

	assert.equal(cdn.isGifMedia("public/post/hero.gif.webp"), false);
	assert.equal(cdn.isGifMedia("public/post/hero.jpg?download=.gif"), false);
	assert.equal(cdn.isGifMedia("public/post/hero.png#fake.gif"), false);
	assert.equal(cdn.isGifMedia("public/post/gif"), false);
	assert.equal(cdn.isGifMedia(""), false);
	assert.equal(cdn.isGifMedia(null), false);
});

// GIF 卡片會動是**刻意的產品行為**（GIF 發文與逐幀取景是 2026-08-26 的功能，
// 賣家端文案就寫著「拖曳 GIF 調整每一幀取景」）。
//
// 卡片走 `motion`：image-gateway 的動圖 preset（backend-payuni#2831，49 號設計書
// §4.4），720 長邊 scale-down、q=70、`anim: true`。對已部署 worker 實測同一顆
// 720×540 / 73 幀的貼文 GIF：
//
//   /i/original/  10,233,546 B  gif   73 幀
//   /i/motion/     2,658,380 B  webp  73 幀   ← −74%，動畫完整保留
//   /i/card/          32,815 B  jpeg   1 幀   ← 靜態
//
// ⚠️ 2026-08-27 這支測試曾被我反轉成「任何輸入都不得回 original」，理由是
// site-stability 的 12 秒 no-pop-in 斷言過不去。那是錯的順序——測試過不去時
// 該調的是那個斷言的預算，不是把別人的功能關掉。已還原，並補上缺的 derivative。
test("post cards keep GIF animation via a bounded animated derivative", () => {
	assert.equal(typeof cdn.postCardPresetForMedia, "function");
	assert.equal(
		cdn.postCardPresetForMedia("public/post/hero.GIF?rev=2"),
		"motion",
	);
	assert.equal(
		cdn.postCardPresetForMedia("public/post/hero.gif", "detail"),
		"motion",
	);
	// 靜態媒體仍走原本的 derivative —— 這一半不受上面的取捨影響。
	assert.equal(cdn.postCardPresetForMedia("public/post/hero.jpg"), "card");
	assert.equal(
		cdn.postCardPresetForMedia("public/post/hero.webp", "detail"),
		"detail",
	);
	// 鑑別力，兩個方向都要守：
	for (const key of ["public/post/a.gif", "public/post/a.GIF?v=1"]) {
		const preset = cdn.postCardPresetForMedia(key);
		// ① 有人為了讓某支測試變綠而把 GIF 改回靜態 → 這裡轉紅
		assert.notEqual(
			preset,
			"card",
			`postCardPresetForMedia(${JSON.stringify(key)}) 不得把動圖壓成靜態；` +
				"卡片動畫是產品行為，處置歸屬見 website#528",
		);
		// ② 有人改回送原檔（10 MB 進首頁）→ 這裡也轉紅
		assert.notEqual(
			preset,
			"original",
			`postCardPresetForMedia(${JSON.stringify(key)}) 不得送原檔；` +
				"卡片面要走有界的動圖 derivative（motion）",
		);
		assert.equal(preset, "motion");
	}
});

test("GIF upload normalization preserves bytes and metadata while making the read-time key stable", async () => {
	const bytes = new Uint8Array([0x47, 0x49, 0x46, 0x38, 0x39, 0x61]);
	const extensionless = new File([bytes], "animation", {
		type: "image/gif",
		lastModified: 1234,
	});
	const normalized = normalizeGifUploadFile(extensionless);

	assert.equal(isGifUploadFile(extensionless), true);
	assert.notEqual(normalized, extensionless);
	assert.equal(normalized.name, "animation.gif");
	assert.equal(normalized.type, "image/gif");
	assert.equal(normalized.lastModified, 1234);
	assert.deepEqual(new Uint8Array(await normalized.arrayBuffer()), bytes);

	const missingMime = new File([bytes], "animation.GIF", {
		type: "",
		lastModified: 5678,
	});
	const typed = normalizeGifUploadFile(missingMime);
	assert.equal(isGifUploadFile(missingMime), true);
	assert.equal(typed.name, "animation.GIF");
	assert.equal(typed.type, "image/gif");
	assert.equal(typed.lastModified, 5678);
	assert.deepEqual(new Uint8Array(await typed.arrayBuffer()), bytes);

	const png = new File([bytes], "not-a-gif.png", { type: "image/png" });
	assert.equal(isGifUploadFile(png), false);
	assert.equal(normalizeGifUploadFile(png), png);
});

test("GIF crop rejects invalid inputs before allocating a browser worker", async () => {
	const area = { x: 0, y: 0, width: 1, height: 1 };
	assert.equal(GIF_CROP_MAX_BYTES, 10 * 1024 * 1024);
	assert.equal(GIF_CROP_MAX_EDGE, 720);

	await assert.rejects(
		cropAnimatedGifFile(
			new File([new Uint8Array([1])], "still.png", { type: "image/png" }),
			area,
			{ containAspect: 4 / 3 },
		),
		(error: unknown) =>
			error instanceof GifCropError && error.code === "invalid-file",
	);

	await assert.rejects(
		cropAnimatedGifFile(
			new File([new Uint8Array(GIF_CROP_MAX_BYTES + 1)], "huge.gif", {
				type: "image/gif",
			}),
			area,
			{ containAspect: 4 / 3 },
		),
		(error: unknown) =>
			error instanceof GifCropError && error.code === "resource-limit",
	);

	await assert.rejects(
		cropAnimatedGifFile(
			new File(["GIF89a"], "invalid-area.gif", { type: "image/gif" }),
			{ ...area, width: 0 },
			{ containAspect: 4 / 3 },
		),
		(error: unknown) =>
			error instanceof GifCropError && error.code === "invalid-crop",
	);

	await assert.rejects(
		cropAnimatedGifFile(
			new File(["NOTGIF"], "spoofed.gif", { type: "image/gif" }),
			area,
			{ containAspect: 4 / 3 },
		),
		(error: unknown) =>
			error instanceof GifCropError && error.code === "invalid-file",
	);
});

test("post composer sends normalized GIF files through the same crop queue", () => {
	const compose = source("features/seller-center/posts/PostComposeClient.tsx");
	const handler = between(
		compose,
		"const handleFileChange",
		"const handleCroppedImage",
	);
	const gifBranch = handler.match(/if \(isGif\) \{([\s\S]*?)\n\t\t\}/)?.[1];

	assert.ok(gifBranch, "handleFileChange must have an explicit GIF branch");
	assert.match(gifBranch, /setPendingImage\(normalizeGifUploadFile\(file\)\)/);
	assert.match(gifBranch, /return;/);
	assert.doesNotMatch(gifBranch, /handleCroppedImage/);
	assert.match(handler, /setError\("[^"]*GIF[^"]*"\)/);
	assert.ok(
		handler.indexOf("setPendingImage(normalizeGifUploadFile(file))") <
			handler.indexOf("setPendingImage(file)"),
		"GIF must be normalized before it enters the shared crop queue",
	);
});

test("post cropper lazy-loads the animated GIF pipeline and locks dismissal while processing", () => {
	const dialog = source("components/ui/ImageCropDialog.tsx");
	const staticImports = between(dialog, '"use client";', "export interface");
	const handleConfirm = between(
		dialog,
		"const handleConfirm",
		"const handleSkip",
	);
	const cancelProcessing = between(
		dialog,
		"const handleCancelProcessing",
		"const handleSkip",
	);

	assert.ok(
		dialog.includes("isGifUploadFile"),
		"the post cropper must distinguish animated GIFs from still images",
	);
	assert.doesNotMatch(
		staticImports,
		/from\s+["']@\/lib\/media\/gif-crop["']/,
		"the animated encoder must not enter the initial crop-dialog bundle",
	);
	assert.match(
		handleConfirm,
		/import\(["']@\/lib\/media\/gif-crop["']\)/,
		"GIF processing must be loaded only after the seller confirms the crop",
	);
	assert.match(handleConfirm, /cropAnimatedGifFile/);
	assert.match(handleConfirm, /isGifUploadFile\(current\)/);
	assert.match(handleConfirm, /new AbortController\(\)/);
	assert.match(handleConfirm, /signal\s*:/);
	assert.match(handleConfirm, /POST_FEED_ASPECT/);
	assert.match(
		handleConfirm,
		/cropImageFile/,
		"still images must keep using the existing crop implementation",
	);

	assert.match(
		dialog,
		/isDismissable=\{!isPostEditor \|\| !processing\}/,
		"only the post editor is nondismissable while its cancellable job runs",
	);
	assert.match(dialog, /showCloseButton=\{!isPostEditor \|\| !processing\}/);
	assert.match(dialog, /inert=\{processing \? true : undefined\}/);
	assert.match(dialog, /disabled=\{processing[^}]*\}/);
	assert.match(cancelProcessing, /processingAbortRef\.current\?\.abort\(\)/);
	assert.match(
		dialog,
		/if\s*\(processing\)\s*\{\s*handleCancelProcessing\(\);\s*return;/,
	);
	assert.match(
		dialog,
		/\{processing\s*\?\s*["']取消處理["']\s*:\s*["']重設["']\s*\}/,
	);
});

test("animated GIF crop runs in an abortable worker and returns one upload file", () => {
	const cropper = source("lib/media/gif-crop.ts");
	const worker = source("lib/media/gif-crop.worker.ts");
	const combined = `${cropper}\n${worker}`;

	assert.match(cropper, /export class GifCropError extends Error/);
	assert.match(cropper, /export async function cropAnimatedGifFile\s*\(/);
	assert.ok(
		(cropper.match(/export const GIF_[A-Z0-9_]+/g) ?? []).length >= 2,
		"GIF resource limits must be public so upload validation and UI copy cannot drift",
	);
	assert.match(
		cropper,
		/new Worker\(\s*new URL\(\s*["']\.\/gif-crop\.worker\.ts["']\s*,\s*import\.meta\.url\s*\)\s*,\s*\{\s*type:\s*["']module["']\s*\}\s*,?\s*\)/,
	);
	assert.match(cropper, /\.terminate\(\)/);
	assert.match(cropper, /addEventListener\(\s*["']abort["']/);
	assert.match(cropper, /postMessage\([\s\S]*?,\s*\[[^\]]+\]\s*\)/);
	assert.match(cropper, /new File\(/);
	assert.match(cropper, /type:\s*["']image\/gif["']/);

	assert.match(worker, /from\s+["']gifuct-js["']/);
	assert.match(worker, /from\s+["']gifenc["']/);
	assert.match(
		worker,
		/decompressFrame\(\s*[^,]+,\s*[^,]+,\s*true\s*\)/,
		"frames must be decompressed one at a time instead of materializing the whole animation",
	);
	assert.match(worker, /for\s*\([^)]*<\s*frames\.length[^)]*\)/);
	assert.match(worker, /writeFrame\(/);
	assert.match(
		worker,
		/\.gce\.delay/,
		"raw centisecond delay must be preserved; gifuct's derived delay changes zero-delay frames",
	);
	assert.match(worker, /applyPreviousDisposal\(/);
	assert.match(worker, /repeat:\s*[^,\n]*loop/i);
	assert.match(worker, /containAspect/);
	assert.match(worker, /transparent:\s*true/);
	assert.match(worker, /transparentIndex\s*:/);
	assert.match(worker, /OffscreenCanvas/);
	assert.match(worker, /clearRect\(/);
	assert.match(worker, /drawImage\(/);
	assert.match(worker, /postMessage\([\s\S]*?,\s*\[[^\]]+\]\s*\)/);

	assert.doesNotMatch(
		combined,
		/\b(?:fetch|uploadImageFile|stageUpload)\s*\(/,
		"crop processing may return one File, but must never create remote media or orphan uploads",
	);
});

test("every post-card surface selects a GIF-safe deployed preset", () => {
	const postCard = source("features/feed/components/PostCardV2.tsx");
	const homeCard = source("features/home/components/HomeStageFeedCard.tsx");
	const sellerList = source(
		"features/seller-center/posts/SellerPostsClient.tsx",
	);
	const sellerDetail = source(
		"features/seller-center/posts/SellerPostDetailModal.tsx",
	);
	const megaMenu = source("components/shared/MegaMenuPanel.tsx");

	assert.match(
		postCard,
		/preset=\{postCardPresetForMedia\((?:imageSrc|post\.imageKeys\[0\])\)\}/,
	);
	assert.match(homeCard, /postCardPresetForMedia\(imageSrc,\s*"detail"\)/);
	assert.match(
		sellerList,
		/<AppImage[\s\S]*?src=\{p\.imageKeys\[0\]\}[\s\S]*?preset=\{postCardPresetForMedia\(p\.imageKeys\[0\]\)\}/,
	);
	assert.match(
		sellerDetail,
		/<AppImage[\s\S]*?src=\{imageKey\}[\s\S]*?preset=\{postCardPresetForMedia\(imageKey\)\}/,
	);
	assert.match(
		megaMenu,
		/preset=\{postCardPresetForMedia\(p\.imageKeys\[0\]\)\}/,
	);
	assert.match(megaMenu, /className="object-contain"/);
});

test("published post thumbnails keep the authored 4:3 frame without a second crop", () => {
	const feedCss = source("styles/features/feed-components.css");
	const sellerCss = source("styles/features/seller-center.css");
	const delegatedCss = source("styles/features/delegated-operations.css");
	const homeCard = source("features/home/components/HomeStageFeedCard.tsx");
	const megaMenu = source("components/shared/MegaMenuPanel.tsx");

	const feedImageRule = between(
		feedCss,
		".hkui-feed-card__media img {",
		"/* ===== Top-left badge",
	);
	assert.match(feedImageRule, /object-fit:\s*contain/);
	assert.doesNotMatch(feedImageRule, /scale\(/);

	const sellerImageRule = between(
		sellerCss,
		".seller-posts__cover img {",
		".seller-posts__main",
	);
	assert.match(sellerImageRule, /object-fit:\s*contain/);
	assert.match(homeCard, /aspect-\[4\/3\]/);
	assert.doesNotMatch(homeCard, /aspect-square|aspect-auto/);
	assert.match(megaMenu, /aspect-\[4\/3\]/);

	const delegatedImageRule = between(
		delegatedCss,
		".adm-post-preview__image {",
		".adm-post-preview__tags",
	);
	assert.match(delegatedImageRule, /aspect-ratio:\s*4\s*\/\s*3/);
	assert.match(delegatedImageRule, /object-fit:\s*contain/);
});

test("buyer post detail may use original for GIF while still images retain detail", () => {
	const viewer = source("features/feed/components/PostDetailViewer.tsx");
	// The GIF/still choice used to sit inline on the <img>. It moved up into
	// `mediaSrc` when the ambient backdrop landed, because that backdrop is a
	// second <img> that must resolve to the SAME url — sharing one const is
	// what keeps it free (browser cache hit) instead of a second download.
	// Anchoring here follows the decision; anchoring on `<img` would now read
	// the backdrop's tag and silently stop checking anything.
	const media = between(viewer, "const mediaSrc =", "\n\n");

	assert.match(media, /isGifMedia\(images\[activeImage\]\)/);
	assert.match(media, /"original"/);
	assert.match(media, /(?:"detail"|"productDetail")/);
	// Both the photo and the ambient backdrop must read that one const.
	assert.equal(viewer.match(/src=\{mediaSrc\}/g)?.length, 2);
});
