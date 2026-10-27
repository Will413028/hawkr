import assert from "node:assert/strict";
import test from "node:test";
import {
	PAGE_CURL_STRIP_COUNT,
	PAGED_TURN_CURL_HANDOFF,
	pageCurlPose,
	pageCurlReleaseDuration,
	pageCurlStripLighting,
	pagedDragProgressToCurlProgress,
} from "../lib/animation/page-curl";

test("page curl is flat at both ends and bends while the leaf is airborne", () => {
	const start = pageCurlPose(0);
	const middle = pageCurlPose(0.5);
	const end = pageCurlPose(1);

	assert.equal(start.rootRotationDegrees, 0);
	assert.equal(start.stripRotationDegrees, 0);
	assert.equal(start.shade, 0);
	assert.ok(middle.rootRotationDegrees > 90);
	assert.ok(middle.stripRotationDegrees > 0);
	assert.equal(middle.shade, 1);
	assert.ok(Math.abs(end.rootRotationDegrees - 180) < 1e-9);
	assert.ok(Math.abs(end.stripRotationDegrees) < 1e-9);
	assert.ok(Math.abs(end.shade) < 1e-9);
});

test("page curl clamps progress and creates changing light across strips", () => {
	assert.deepEqual(pageCurlPose(-1), pageCurlPose(0));
	assert.deepEqual(pageCurlPose(2), pageCurlPose(1));

	const first = pageCurlStripLighting(0.46, 0);
	const last = pageCurlStripLighting(0.46, PAGE_CURL_STRIP_COUNT - 1);
	assert.notEqual(first.nearShadow, last.nearShadow);
	assert.ok(first.nearShadow >= 0 && first.nearShadow <= 0.54);
	assert.ok(last.farLight >= 0 && last.farLight <= 1);
});

test("paged grip progress drives the full flexible paper turn", () => {
	assert.equal(pagedDragProgressToCurlProgress(0), 0);
	assert.equal(pagedDragProgressToCurlProgress(1), PAGED_TURN_CURL_HANDOFF);
	assert.equal(pagedDragProgressToCurlProgress(2), PAGED_TURN_CURL_HANDOFF);
});

test("release timing follows remaining curl distance and a forward fling", () => {
	const fullTurn = pageCurlReleaseDuration(0.2, 0, 1);
	const nearlyDone = pageCurlReleaseDuration(0.8, 0, 1);
	const flung = pageCurlReleaseDuration(0.2, 1.2, 1);
	const cancelled = pageCurlReleaseDuration(0.3, 0, 0);

	assert.ok(fullTurn > nearlyDone);
	assert.ok(flung < fullTurn);
	assert.ok(cancelled >= 0.14 && cancelled <= 0.38);
});
