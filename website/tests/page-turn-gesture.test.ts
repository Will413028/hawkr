import assert from "node:assert/strict";
import test from "node:test";
import {
	PAGED_TURN_MAX_ANGLE_DEG,
	pagedTurnReleaseDuration,
	pageTurnProgressForDrag,
	pageTurnProjectedDistance,
	pageTurnReleaseVelocity,
} from "../lib/animation/page-turn-gesture";

test("page turn keeps the grabbed point aligned before flexible follow-through", () => {
	const gripDistance = 300;
	const earlyDragDistance = gripDistance * 0.2;
	const expectedRigidProgress = Math.acos(0.8) / Math.PI;

	assert.equal(pageTurnProgressForDrag(0, gripDistance), 0);
	assert.ok(
		Math.abs(
			pageTurnProgressForDrag(earlyDragDistance, gripDistance) -
				expectedRigidProgress,
		) < 1e-9,
	);
	assert.ok(pageTurnProgressForDrag(gripDistance * 0.75, gripDistance) > 0.5);
	assert.ok(
		Math.abs(
			pageTurnProgressForDrag(gripDistance, gripDistance) -
				PAGED_TURN_MAX_ANGLE_DEG / 180,
		) < 1e-9,
	);
	assert.ok(
		Math.abs(
			pageTurnProgressForDrag(gripDistance * 2, gripDistance) -
				PAGED_TURN_MAX_ANGLE_DEG / 180,
		) < 1e-9,
	);
	assert.equal(pageTurnProgressForDrag(-20, gripDistance), 0);
});

test("page turn progress reverses continuously when the finger moves back", () => {
	const outward = pageTurnProgressForDrag(120, 280);
	const returned = pageTurnProgressForDrag(40, 280);

	assert.ok(outward > returned);
	assert.ok(returned > 0);
});

test("release velocity keeps a recent flick through stationary pointer samples", () => {
	const samples = [
		{ x: 275, time: 0 },
		{ x: 240, time: 20 },
		{ x: 240, time: 60 },
		{ x: 240, time: 70 },
	];

	assert.equal(pageTurnReleaseVelocity(samples, 70), -1.75);
	assert.equal(pageTurnReleaseVelocity(samples, 121), 0);
});

test("release velocity follows the latest real reversal", () => {
	const samples = [
		{ x: 275, time: 0 },
		{ x: 240, time: 20 },
		{ x: 260, time: 40 },
		{ x: 260, time: 60 },
	];

	assert.equal(pageTurnReleaseVelocity(samples, 60), 1);
});

test("release projection advances a flick and retreats a reversal", () => {
	assert.equal(pageTurnProjectedDistance(30, 0.5), 110);
	assert.equal(pageTurnProjectedDistance(80, -0.25), 40);
	assert.equal(pageTurnProjectedDistance(20, -2), 0);
	assert.equal(pageTurnProjectedDistance(20, Number.NaN), 20);
});

test("legacy paged release duration remains bounded", () => {
	assert.equal(pagedTurnReleaseDuration(0), 0.2);
	assert.equal(pagedTurnReleaseDuration(0.5), 0.1);
	assert.equal(pagedTurnReleaseDuration(1), 0.06);
	assert.equal(pagedTurnReleaseDuration(2), 0.06);
});
