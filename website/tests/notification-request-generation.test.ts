import assert from "node:assert/strict";
import test from "node:test";
import { createNotificationRequestGeneration } from "../lib/notifications/request-generation";

test("late unread response from the previous workspace is permanently obsolete", () => {
	const requests = createNotificationRequestGeneration();
	const buyer = requests.begin();

	// Effect cleanup for buyer, followed immediately by the seller baseline.
	requests.invalidate();
	const seller = requests.begin();

	assert.equal(requests.isCurrent(buyer), false);
	assert.equal(requests.isCurrent(seller), true);
});

test("a newer recovery request also supersedes an older in-flight baseline", () => {
	const requests = createNotificationRequestGeneration();
	const baseline = requests.begin();
	const bufferMissRecovery = requests.begin();

	assert.equal(requests.isCurrent(baseline), false);
	assert.equal(requests.isCurrent(bufferMissRecovery), true);
});

test("an authoritative SSE count supersedes an in-flight REST baseline", () => {
	const requests = createNotificationRequestGeneration();
	const baseline = requests.begin();

	// notification.new / notification.unread_changed carries an absolute count.
	requests.invalidate();

	assert.equal(requests.isCurrent(baseline), false);
});

test("an optimistic local unread change supersedes an in-flight REST baseline", () => {
	const requests = createNotificationRequestGeneration();
	const baseline = requests.begin();

	// setCount / decrement / increment invalidate before updating local state.
	requests.invalidate();

	assert.equal(requests.isCurrent(baseline), false);
});
