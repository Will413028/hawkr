import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

const source = readFileSync(
	fileURLToPath(
		new URL(
			"../features/messages/components/ConversationSidebar.tsx",
			import.meta.url,
		),
	),
	"utf8",
);

test("conversation rows use the native App Router link", () => {
	// `lib/intl/native-link.tsx` is `next/link` with the page's language in the href and nothing else.
	assert.match(source, /import Link from "@\/lib\/intl\/native-link";/);
	assert.doesNotMatch(source, /import Link from "next\/link";/);
	assert.doesNotMatch(
		source,
		/import Link from "@\/components\/shared\/Link";/,
	);
	assert.match(source, /href=\{`\$\{threadHrefBase\}\/\$\{id\}`\}/);
});
