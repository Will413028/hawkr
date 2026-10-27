import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const commentsSource = readFileSync(
	new URL("../features/feed/components/PostComments.tsx", import.meta.url),
	"utf8",
);
const metricIconsSource = readFileSync(
	new URL("../features/feed/components/PostMetricIcons.tsx", import.meta.url),
	"utf8",
);

test("comments and replies reuse the post-detail heart", () => {
	assert.match(
		commentsSource,
		/import AppIcon from "@\/components\/shared\/AppIcon";/,
	);
	assert.equal(commentsSource.match(/<AppIcon\s+name="heart"/g)?.length, 2);
	assert.match(
		commentsSource,
		/<AppIcon name="heart" fill=\{liked\} size=\{16\} \/>/,
	);
	assert.match(
		commentsSource,
		/fill=\{comment\.isLiked === true\}[\s\S]*?size=\{16\}/,
	);
	assert.doesNotMatch(commentsSource, /PostHeartIcon/);
	assert.doesNotMatch(metricIconsSource, /PostHeartIcon/);
});
