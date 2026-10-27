import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

function source(relativePath: string) {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

const componentPath =
	"features/seller/landing/components/SellerRecruitmentScrollWorld.tsx";
const stylePath = "styles/features/seller-recruitment-scroll-world.css";

test("seller Scroll World keeps all approved scenes and their posters", () => {
	const component = source(componentPath);
	const sceneIds = [
		"birth",
		"harvest",
		"sorting",
		"processing",
		"distribution",
		"retail",
		"table",
	];

	for (const id of sceneIds) {
		assert.match(component, new RegExp(`id: "${id}"`));
		const poster = `placeholders/journey-${id}.svg`;
		assert.match(component, new RegExp(`/${poster.replace(".", "\\.")}`));
		assert.ok(
			existsSync(new URL(`../public/${poster}`, import.meta.url)),
			`${poster} should exist`,
		);
	}

	for (const copy of [
		"人潮很多。",
		"但很少人駐足停留。",
		"你的品牌，也被略過了嗎？",
		"停留，才會認識產地。",
		"看見，才會認識價值。",
		"高曝光，不一定會有好效果。",
		"了解產品的故事，商品就不會只是一張照片。",
		"我們想把舞台還給產品本身。",
		"讓好內容和好產品，走進日常生活。",
		"讓產品有更好的舞台",
	]) {
		assert.match(component, new RegExp(copy.replace(/[。？]/g, "\\$&")));
	}

	assert.match(component, /href="\/seller\/onboarding"/);
	assert.match(component, /<BrandLogo/);
	assert.doesNotMatch(component, /data-seller-snap/);
});

test("seller Scroll World always keeps its scroll-driven scene engine active", () => {
	const component = source(componentPath);
	const styles = source(stylePath);

	assert.match(component, /prefers-reduced-motion: reduce/);
	assert.match(component, /behavior: reduce \? "auto" : "smooth"/);
	assert.doesNotMatch(component, /<video/);
	assert.match(
		component,
		/updateFromScroll\(\);\s*window\.addEventListener\("scroll", onScroll/,
	);
	assert.doesNotMatch(component, /setReducedMotion|dataset\.reducedMotion/);
	assert.match(styles, /--srw-paper:\s*#faf6ec/);
	assert.match(styles, /--srw-ink-500:\s*var\(--color-ink-500\)/);
	assert.match(styles, /--srw-gold:\s*#d4b758/);
	assert.match(styles, /\.seller-recruitment-world \.srw-scene-rail/);
	assert.match(styles, /@media \(max-width: 640px\)/);
	assert.match(
		styles,
		/@media \(prefers-reduced-motion: reduce\) \{\s*\.seller-recruitment-world \.srw-scroll-hint \{\s*animation: none;/,
	);
	assert.doesNotMatch(styles, /data-reduced-motion/);
	assert.doesNotMatch(styles, /(^|\n)\.(?:world|scene-copy|brand)\b/);
});

test("seller Scroll World gives frosted cards and the end stage a scoped dark theme", () => {
	const styles = source(stylePath);

	assert.match(
		styles,
		/\[data-theme="dark"\] \.seller-recruitment-world \.srw-copy--has-steps/,
	);
	assert.match(styles, /rgb\(var\(--color-ink-950-rgb\) \/ 0\.86\)/);
	assert.match(
		styles,
		/\[data-theme="dark"\] \.seller-recruitment-world \.srw-endcard/,
	);
	assert.match(
		styles,
		/linear-gradient\(160deg, var\(--color-ink-900\) 0%, var\(--color-ink-950\) 100%\)/,
	);
	assert.match(styles, /\.srw-endcard h2 \{\s*color: #f5e9c3;/);
	assert.match(
		styles,
		/\.srw-endcard p \{\s*color: rgb\(var\(--color-ink-100-rgb\) \/ 0\.76\);/,
	);
	assert.match(
		styles,
		/@media \(max-width: 640px\)[\s\S]*?\[data-theme="dark"\] \.seller-recruitment-world \.srw-copy/,
	);
	assert.match(styles, /rgb\(var\(--color-ink-950-rgb\) \/ 0\.88\)/);
	assert.doesNotMatch(
		styles,
		/\[data-theme="dark"\] \.seller-recruitment-world \{[\s\S]*?--srw-ink:/,
	);
});

test("seller Scroll World removes the mobile fog and teaches the first scroll", () => {
	const component = source(componentPath);
	const styles = source(stylePath);

	assert.match(component, /<AppIcon[\s\S]*?name="arrow-down"/);
	assert.match(component, /seller-scroll-world-hint-seen/);
	assert.match(component, /window\.matchMedia\("\(max-width: 640px\)"\)/);
	assert.match(component, /window\.setTimeout\(dismiss, 6000\)/);
	assert.match(component, /往下滑，繼續看/);
	assert.match(
		styles,
		/@media \(max-width: 640px\)[\s\S]*?\.seller-recruitment-world \.srw-media::after \{\s*background: none;/,
	);
	assert.match(styles, /\.seller-recruitment-world \.srw-scroll-hint/);
	assert.match(
		styles,
		/\[data-theme="dark"\] \.seller-recruitment-world \.srw-scroll-hint/,
	);
});

test("seller Scroll World matches the desktop seller Hero glass width", () => {
	const styles = source(stylePath);
	const sellerStyles = source("styles/features/seller-onboarding.css");

	for (const contract of [
		/width: calc\(100% - 40px\);/,
		/max-width: 1440px;/,
		/border-radius: 32px;/,
	]) {
		assert.match(styles, contract);
		assert.match(sellerStyles, contract);
	}

	assert.match(
		styles,
		/@media \(min-width: 881px\)[\s\S]*?\.seller-recruitment-world \.srw-sticky \{\s*border-radius: inherit;/,
	);
	assert.match(
		styles,
		/@media \(max-width: 640px\)[\s\S]*?\.seller-recruitment-world \{\s*height: 1040vh;/,
	);
});

test("seller Scroll World gutters expose the shared site canvas without replacing its scene stage", () => {
	const styles = source(stylePath);

	assert.match(
		styles,
		/\.seller-recruitment-world \{[\s\S]*?background: transparent;/,
	);
	assert.match(
		styles,
		/\.seller-recruitment-world \.srw-sticky \{[\s\S]*?background: var\(--srw-paper\);/,
	);
	assert.doesNotMatch(styles, /body:has\(\.seller-recruitment-world\)/);
	assert.doesNotMatch(
		styles,
		/page-bg-tall(?:-mobile)?\.webp/,
		"the shared site canvas should not reload an extra full-page illustration",
	);
});
