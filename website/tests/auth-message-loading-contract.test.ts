import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const sendButtonPath = new URL(
	"../features/messages/components/SendButton.tsx",
	import.meta.url,
);

test("message actions use the shared branded loading contract", async () => {
	const sendButton = await readFile(sendButtonPath, "utf8");

	assert.match(sendButton, /isLoading=\{loading\}/);
	assert.match(sendButton, /loadingLabel="傳送訊息中…"/);
	assert.match(sendButton, /isIconOnly/);
	assert.match(sendButton, /<AppIcon name="send"/);
	assert.doesNotMatch(sendButton, /send-icon\.json/);
});
