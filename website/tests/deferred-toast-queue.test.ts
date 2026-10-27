import assert from "node:assert/strict";
import test from "node:test";
import { createDeferredQueue } from "../lib/feedback/toast";

test("toast calls made before the viewport is ready flush in order", () => {
	const deliveries: string[][] = [];
	const queue = createDeferredQueue<string>((items) => deliveries.push(items));

	queue.push("first");
	queue.push("second");
	assert.deepEqual(deliveries, []);

	queue.markReady();
	assert.deepEqual(deliveries, [["first", "second"]]);

	queue.push("third");
	assert.deepEqual(deliveries, [["first", "second"], ["third"]]);
});

test("marking the toast viewport ready twice never replays deliveries", () => {
	const deliveries: string[][] = [];
	const queue = createDeferredQueue<string>((items) => deliveries.push(items));

	queue.push("only");
	queue.markReady();
	queue.markReady();

	assert.deepEqual(deliveries, [["only"]]);
});
