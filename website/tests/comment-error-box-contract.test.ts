import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const read = (rel: string) => readFileSync(join(ROOT, rel), "utf8");

const DANGER_BOX =
	/--color-status-danger-rgb|--hkui-danger-50|--hkui-danger-text/;

test("comment and forum failure boxes share one danger style", () => {
	const shared = read("features/feed/components/CommentEntryParts.tsx");
	assert.match(shared, DANGER_BOX);
	for (const rel of [
		"features/feed/components/PostComments.tsx",
		"features/sellers/components/SellerMessageTab.tsx",
	]) {
		const source = read(rel);
		assert.doesNotMatch(
			source,
			DANGER_BOX,
			`${rel} defines its own danger box`,
		);
		assert.match(
			source,
			/<CommentErrorBox/,
			`${rel} does not use CommentErrorBox`,
		);
	}
});
