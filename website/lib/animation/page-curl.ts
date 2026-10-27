const RAD_TO_DEG = 180 / Math.PI;

export const PAGE_CURL_STRIP_COUNT = 14;
export const PAGE_CURL_MAX_BEND_RADIANS = 0.52;
export const PAGED_TURN_CURL_HANDOFF = 1;

function clamp(value: number, min: number, max: number) {
	return Math.min(Math.max(value, min), max);
}

export interface PageCurlPose {
	progress: number;
	rootRotationDegrees: number;
	stripRotationDegrees: number;
	shade: number;
}

/**
 * Describes one curved paper leaf. The root performs the page turn while the
 * nested strips counter-rotate into an arc. The bend is zero at rest and at
 * the destination, and strongest while the sheet is in the air.
 */
export function pageCurlPose(
	progress: number,
	stripCount = PAGE_CURL_STRIP_COUNT,
): PageCurlPose {
	const safeProgress = clamp(progress, 0, 1);
	const safeStripCount = Math.max(1, Math.round(stripCount));
	const turnRadians = Math.PI * safeProgress;
	const bendRadians =
		PAGE_CURL_MAX_BEND_RADIANS * Math.sin(Math.PI * safeProgress);

	return {
		progress: safeProgress,
		rootRotationDegrees: (turnRadians + bendRadians) * RAD_TO_DEG,
		stripRotationDegrees: ((2 * bendRadians) / safeStripCount) * RAD_TO_DEG,
		shade: Math.sin(Math.PI * safeProgress),
	};
}

export function pageCurlStripLighting(
	progress: number,
	stripIndex: number,
	stripCount = PAGE_CURL_STRIP_COUNT,
) {
	const pose = pageCurlPose(progress, stripCount);
	const rootRadians = (pose.rootRotationDegrees * Math.PI) / 180;
	const stripRadians = (pose.stripRotationDegrees * Math.PI) / 180;
	const safeIndex = clamp(
		Math.round(stripIndex),
		0,
		Math.max(0, stripCount - 1),
	);
	const nearFacing = Math.abs(Math.cos(rootRadians - safeIndex * stripRadians));
	const farFacing = Math.abs(
		Math.cos(rootRadians - (safeIndex + 1) * stripRadians),
	);

	return {
		nearLight: nearFacing,
		farLight: farFacing,
		nearShadow: (1 - nearFacing) * 0.54,
		farShadow: (1 - farFacing) * 0.54,
	};
}

/** The drag helper already returns full curl progress, including follow-through. */
export function pagedDragProgressToCurlProgress(progress: number) {
	return clamp(progress, 0, 1);
}

export function pageCurlReleaseDuration(
	progress: number,
	velocity = 0,
	target: 0 | 1 = 1,
) {
	const safeProgress = clamp(progress, 0, 1);
	const distance = Math.abs(target - safeProgress);
	const velocityTowardTarget = target === 1 ? velocity : -velocity;
	const velocityAssist = clamp(velocityTowardTarget, 0, 1.8) * 0.05;
	return clamp(0.12 + distance * 0.3 - velocityAssist, 0.14, 0.38);
}
