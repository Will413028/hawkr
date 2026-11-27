import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { isAccountMessageThreadPath } from "../lib/navigation/message-thread";

function source(relativePath: string) {
	return readFileSync(
		fileURLToPath(new URL(`../${relativePath}`, import.meta.url)),
		"utf8",
	);
}

test("navbar chat navigation defers its route prefetch to user intent", () => {
	const chatBubble = source("components/shared/ChatBubble.tsx");
	const bubbleBase = source("components/shared/NavBubbleLink.tsx");

	assert.match(chatBubble, /from "@\/components\/shared\/NavBubbleLink"/);
	assert.doesNotMatch(chatBubble, /from "next\/link"/);
	assert.match(bubbleBase, /import Link from "@\/components\/shared\/Link";/);

	assert.match(chatBubble, /prefetchOnIntent/);
	assert.match(
		bubbleBase,
		/prefetchOnIntent \? \(routePrefetchEnabled \? null : false\) : undefined/,
	);
	assert.match(bubbleBase, /onMouseEnter=\{\(event\) => \{/);
	assert.match(bubbleBase, /onFocus=\{\(event\) => \{/);
	assert.match(bubbleBase, /onPointerDown=\{\(event\) => \{/);
	assert.doesNotMatch(bubbleBase, /useRouter|router\.prefetch|setTimeout/);
	assert.doesNotMatch(chatBubble, /useRouter|router\.prefetch|setTimeout/);
});

test("thread query reuses conversations without a duplicate stream subscription", () => {
	const hook = source("lib/hooks/useChat.ts");
	const thread = source("features/account/screens/messages/ThreadClient.tsx");

	assert.match(hook, /subscribeToSocket = true/);
	assert.match(hook, /shouldFetch && subscribeToSocket/);
	assert.match(thread, /useConversations\(\{ subscribeToSocket: false \}\)/);
	assert.doesNotMatch(
		hook,
		/placeholderData/,
		"a failed list read must not be indistinguishable from an empty list",
	);
});

test("chat event handlers consume send rejections after the hook surfaces them", () => {
	const thread = source("features/account/screens/messages/ThreadClient.tsx");
	const textHandler = thread.slice(
		thread.indexOf("const handleSend ="),
		thread.indexOf("const handleChange ="),
	);

	assert.match(textHandler, /await send\(draft\.trim\(\)\);[\s\S]*catch\s*\{/);
	assert.match(
		textHandler,
		/setDraft\(""\);[\s\S]*catch\s*\{/,
		"the draft is cleared only after a successful send",
	);
});

test("chat composer keeps its focus indicator on the outer pill only", () => {
	const messages = source("styles/features/messages.css");

	assert.match(
		messages,
		/\.thread-page__field:focus-within\s*\{[^}]*box-shadow:(?!\s*none\s*;)\s*[^;]+;/,
	);
	assert.match(
		messages,
		/\.thread-page__compose textarea:focus,\s*\.thread-page__compose textarea:focus-visible\s*\{[^}]*outline:\s*none;[^}]*border-color:\s*transparent;[^}]*box-shadow:\s*none;/,
	);
});

test("only buyer message child routes are treated as full-screen threads", () => {
	assert.equal(isAccountMessageThreadPath("/account/messages"), false);
	assert.equal(isAccountMessageThreadPath("/account/messages/"), false);
	assert.equal(isAccountMessageThreadPath("/account/messages/200"), true);
	assert.equal(isAccountMessageThreadPath("/account/messages/200/"), true);
	assert.equal(
		isAccountMessageThreadPath("/account/messages/assistant/demo"),
		true,
	);
	assert.equal(isAccountMessageThreadPath("/account/messageship/200"), false);
	assert.equal(
		isAccountMessageThreadPath("/seller-center/messages/200"),
		false,
	);
});

test("buyer threads hide global bottom nav and use buyer viewport geometry", () => {
	const mobileNav = source("components/shared/MobileBottomNav.tsx");
	const layout = source("app/[locale]/(account)/account/messages/layout.tsx");
	const messages = source("styles/features/messages.css").replace(/\s+/g, " ");

	assert.match(mobileNav, /isAccountMessageThreadPath\(pathname\)/);
	assert.match(layout, /data-chat-view=\{isThreadRoute \? "thread" : "list"\}/);
	assert.match(layout, /data-chat-workspace="buyer"/);
	assert.match(
		messages,
		/body \.page-content:has\( \.chat-page\[data-chat-workspace="buyer"\]\[data-chat-view="thread"\] \) \{ padding-bottom: 0;/,
	);
	assert.match(
		messages,
		/\.chat-page\[data-chat-workspace="buyer"\]\[data-chat-view="thread"\] \.chat-shell \{ height: calc\( 100dvh - var\(--floating-nav-height, 84px\) - var\(--floating-nav-top-gap, 16px\) - var\(--hkui-space-3\) - 16px - var\(--keyboard-inset, 0px\) \); min-height: 0;/,
	);
});

test("mobile hides the conversation list by route, not by the content pane's class", () => {
	const messages = source("styles/features/messages.css").replace(/\s+/g, " ");
	const sellerLayout = source(
		"app/[locale]/(seller)/seller-center/messages/layout.tsx",
	);

	assert.doesNotMatch(messages, /\.chat-sidebar:has\(\+ \.thread-page\)/);
	assert.match(
		messages,
		/\.chat-page\[data-chat-view="thread"\] > \.chat-shell > \.chat-sidebar \{ display: none;/,
	);
	assert.match(sellerLayout, /data-chat-view="thread"/);
});
