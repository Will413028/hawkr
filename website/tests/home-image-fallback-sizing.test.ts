import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const stage = readFileSync(
	new URL(
		"../features/home/components/HomeCompactMarketStage.tsx",
		import.meta.url,
	),
	"utf8",
);
const feedCard = readFileSync(
	new URL("../features/home/components/HomeStageFeedCard.tsx", import.meta.url),
	"utf8",
);

test("home editor picks pass their rendered thumbnail size to the image fallback", () => {
	assert.match(
		stage,
		/<AppImage[\s\S]*?src=\{productImage\(product\)\}[\s\S]*?width=\{64\}[\s\S]*?height=\{64\}[\s\S]*?sizes="64px"[\s\S]*?style=\{\{ width: "100%", height: "100%" \}\}/,
	);
	assert.doesNotMatch(
		stage,
		/src=\{productImage\(product\)\}[\s\S]*?width=\{128\}[\s\S]*?height=\{128\}/,
	);
});

test("home feed LCP priority hint stays stable while the carousel rotates", () => {
	assert.match(
		stage,
		/\{feedItems\.map\(\(feed, index\) => \([\s\S]*?imagePriority=\{index === 0\}/,
	);
	assert.doesNotMatch(stage, /imagePriority=\{feed\.id === activeFeed\.id\}/);
	assert.match(
		feedCard,
		/loading=\{imagePriority \? "eager" : imageLoading\}[\s\S]*?fetchPriority=\{imagePriority \? "high" : "low"\}/,
	);
	assert.doesNotMatch(feedCard, /\b(?:priority|preload)=/);
});
