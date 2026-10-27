interface KillableGsapTween {
	kill: () => unknown;
}

interface PausableGsapTween {
	pause: () => unknown;
}

function isKillableGsapTween(value: unknown): value is KillableGsapTween {
	if (!value || (typeof value !== "object" && typeof value !== "function")) {
		return false;
	}

	return typeof Reflect.get(value, "kill") === "function";
}

function isPausableGsapTween(value: unknown): value is PausableGsapTween {
	if (!value || (typeof value !== "object" && typeof value !== "function")) {
		return false;
	}

	return typeof Reflect.get(value, "pause") === "function";
}

/**
 * ScrollTrigger.getTween(true) uses `0` as its idle snap-tween sentinel at
 * runtime even though GSAP's public type currently declares a Tween return.
 * Optional chaining does not protect a non-nullish sentinel, so normalize the
 * value before trying to cancel it.
 */
export function killGsapTweenCandidate(candidate: unknown): boolean {
	const candidates = Array.isArray(candidate) ? candidate : [candidate];
	let killed = false;

	for (const value of candidates) {
		if (!isKillableGsapTween(value)) continue;
		value.kill();
		killed = true;
	}

	return killed;
}

/** Keep ScrollTrigger's reusable scrub tween alive while stopping its momentum. */
export function pauseGsapTweenCandidate(candidate: unknown): boolean {
	const candidates = Array.isArray(candidate) ? candidate : [candidate];
	let paused = false;

	for (const value of candidates) {
		if (!isPausableGsapTween(value)) continue;
		value.pause();
		paused = true;
	}

	return paused;
}
