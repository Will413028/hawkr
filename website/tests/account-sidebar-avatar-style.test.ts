import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const sidebar = readFileSync(
	new URL("../features/account/components/AccountSidebar.tsx", import.meta.url),
	"utf8",
);

test("account sidebar keeps the buyer avatar circular and cover-cropped", () => {
	const avatarStart = sidebar.indexOf("<Avatar");
	const avatarEnd = sidebar.indexOf("/>", avatarStart);
	assert.ok(avatarStart >= 0 && avatarEnd > avatarStart);

	const avatar = sidebar.slice(avatarStart, avatarEnd);
	for (const className of [
		"size-11!",
		"shrink-0",
		"rounded-[50%]",
		"object-cover",
	]) {
		assert.ok(
			avatar.includes(className),
			`account sidebar avatar must include ${className}`,
		);
	}
});
