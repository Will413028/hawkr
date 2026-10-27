import assert from "node:assert/strict";
import test from "node:test";
import { notificationEventWorkspace } from "../lib/notifications/event-role";

test("only the active workspace accepts an account-wide notification item", () => {
	assert.equal(notificationEventWorkspace("seller", "seller"), "current");
	assert.equal(notificationEventWorkspace("seller", "buyer"), "other");
	assert.equal(notificationEventWorkspace(undefined, "buyer"), "unknown");
	assert.equal(notificationEventWorkspace("seller", "seller", 41, 42), "other");
	assert.equal(
		notificationEventWorkspace("seller", "seller", undefined, 42),
		"current",
	);
	assert.equal(
		notificationEventWorkspace(undefined, "seller", undefined, 42),
		"unknown",
	);
});

test("a caller without a workspace keeps the legacy single-feed behavior", () => {
	assert.equal(notificationEventWorkspace("seller", null), "current");
});
