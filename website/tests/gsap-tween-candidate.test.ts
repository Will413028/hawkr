import assert from "node:assert/strict";
import test from "node:test";
import { gsap } from "gsap";
import {
	killGsapTweenCandidate,
	pauseGsapTweenCandidate,
} from "../lib/animation/gsap";

test("GSAP tween cancellation ignores ScrollTrigger idle sentinels", () => {
	for (const candidate of [undefined, null, 0, false, {}, [], [{}]]) {
		assert.doesNotThrow(() => {
			assert.equal(killGsapTweenCandidate(candidate), false);
		});
	}
});

test("GSAP tween cancellation kills valid tween candidates once", () => {
	let calls = 0;
	const tween = {
		kill() {
			calls += 1;
		},
	};

	assert.equal(killGsapTweenCandidate(tween), true);
	assert.equal(calls, 1);

	assert.equal(killGsapTweenCandidate([0, tween, {}]), true);
	assert.equal(calls, 2);
});

test("GSAP scrub cancellation pauses reusable tween candidates", () => {
	let calls = 0;
	const tween = {
		pause() {
			calls += 1;
		},
	};

	assert.equal(pauseGsapTweenCandidate(0), false);
	assert.equal(pauseGsapTweenCandidate(tween), true);
	assert.equal(calls, 1);
});

test("GSAP tween guards accept an actual GSAP tween", () => {
	const tween = gsap.to({ x: 0 }, { x: 1, paused: true });

	tween.play();
	assert.equal(tween.paused(), false);
	assert.equal(pauseGsapTweenCandidate(tween), true);
	assert.equal(tween.paused(), true);

	assert.ok(tween.parent);
	assert.equal(killGsapTweenCandidate(tween), true);
	assert.equal(tween.parent, null);
});
