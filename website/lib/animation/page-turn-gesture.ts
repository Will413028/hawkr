export const PAGED_TURN_MAX_ANGLE_DEG = 172;
export const PAGE_TURN_FLING_MAX_AGE_MS = 100;
export const PAGE_TURN_PROJECTION_MS = 160;

const PAGED_TURN_RIGID_FOLLOW_RATIO = 0.34;
const PAGED_TURN_END_SLOPE = 0.12;
const MAX_PROJECTED_VELOCITY_PX_PER_MS = 2.5;

export interface PageTurnPointerSample {
	x: number;
	time: number;
}

function clamp(value: number, min: number, max: number) {
	return Math.min(Math.max(value, min), max);
}

function hermiteUnitProgress(
	progress: number,
	startSlope: number,
	endSlope: number,
) {
	const safeProgress = clamp(progress, 0, 1);
	const squared = safeProgress * safeProgress;
	const cubed = squared * safeProgress;
	return clamp(
		(cubed - 2 * squared + safeProgress) * startSlope +
			(-2 * cubed + 3 * squared) +
			(cubed - squared) * endSlope,
		0,
		1,
	);
}

/**
 * Maps a horizontal drag to the rotation of a page around its hinge.
 *
 * The first third preserves the rigid-sheet cosine relationship so the page
 * leaves the finger without a jump. A flexible-sheet follow-through then
 * carries the page across its hinge while the pointer is still held, avoiding
 * the old 88-degree plateau where the finger kept moving but the paper did not.
 */
export function pageTurnProgressForDrag(
	dragDistance: number,
	gripDistanceFromHinge: number,
) {
	const gripDistance = Math.max(1, gripDistanceFromHinge);
	const dragRatio = clamp(Math.max(0, dragDistance) / gripDistance, 0, 1);
	const rigidProgress = Math.acos(1 - dragRatio) / Math.PI;
	if (dragRatio <= PAGED_TURN_RIGID_FOLLOW_RATIO) return rigidProgress;

	const followStartProgress =
		Math.acos(1 - PAGED_TURN_RIGID_FOLLOW_RATIO) / Math.PI;
	const maxProgress = PAGED_TURN_MAX_ANGLE_DEG / 180;
	const followSpan = 1 - PAGED_TURN_RIGID_FOLLOW_RATIO;
	const progressSpan = maxProgress - followStartProgress;
	const rigidSlopeAtHandoff =
		1 /
		(Math.PI *
			Math.sqrt(
				1 -
					(1 - PAGED_TURN_RIGID_FOLLOW_RATIO) *
						(1 - PAGED_TURN_RIGID_FOLLOW_RATIO),
			));
	const normalizedStartSlope =
		(rigidSlopeAtHandoff * followSpan) / progressSpan;
	const followProgress =
		(dragRatio - PAGED_TURN_RIGID_FOLLOW_RATIO) / followSpan;

	return (
		followStartProgress +
		progressSpan *
			hermiteUnitProgress(
				followProgress,
				normalizedStartSlope,
				PAGED_TURN_END_SLOPE,
			)
	);
}

/**
 * Returns the velocity of the latest meaningful movement near release.
 *
 * Browsers may emit one or more stationary pointer samples between a quick
 * flick and `pointerup`. Treating those samples as zero velocity makes a valid
 * flick decay into a cancelled turn, so only a real position change replaces
 * the candidate velocity. The age gate still prevents an old flick from
 * committing after the pointer has paused on the page.
 */
export function pageTurnReleaseVelocity(
	samples: readonly PageTurnPointerSample[],
	releaseTime: number,
	maxAgeMs = PAGE_TURN_FLING_MAX_AGE_MS,
) {
	let previous: PageTurnPointerSample | undefined;
	let latestMovementTime = Number.NEGATIVE_INFINITY;
	let latestVelocity = 0;

	for (const sample of samples) {
		if (!Number.isFinite(sample.x) || !Number.isFinite(sample.time)) continue;
		if (previous) {
			const elapsed = sample.time - previous.time;
			const distance = sample.x - previous.x;
			if (elapsed > 0 && Math.abs(distance) >= 0.5) {
				latestVelocity = distance / elapsed;
				latestMovementTime = sample.time;
			}
		}
		previous = sample;
	}

	if (!Number.isFinite(releaseTime)) return 0;
	const movementAge = Math.max(0, releaseTime - latestMovementTime);
	return movementAge <= Math.max(0, maxAgeMs) ? latestVelocity : 0;
}

/**
 * Projects the held page a short distance in the release direction. This is
 * intentionally bounded: a noisy browser sample should not throw several
 * pages, while a deliberate flick should still cross the snap threshold.
 */
export function pageTurnProjectedDistance(
	dragDistance: number,
	velocityPxPerMs: number,
	projectionMs = PAGE_TURN_PROJECTION_MS,
) {
	const safeDistance = Math.max(0, dragDistance);
	const safeVelocity = Number.isFinite(velocityPxPerMs)
		? clamp(
				velocityPxPerMs,
				-MAX_PROJECTED_VELOCITY_PX_PER_MS,
				MAX_PROJECTED_VELOCITY_PX_PER_MS,
			)
		: 0;
	return Math.max(0, safeDistance + safeVelocity * Math.max(0, projectionMs));
}

export function pagedTurnReleaseDuration(progress: number) {
	const remaining = 1 - clamp(progress, 0, 1);
	return clamp(remaining * 0.2, 0.06, 0.2);
}
