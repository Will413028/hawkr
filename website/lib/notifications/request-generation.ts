/**
 * Sequence guard for async notification baselines.
 *
 * A boolean cancelled flag is unsafe across buyer/seller resets: the next
 * effect sets it back to false before the previous request necessarily settles.
 * Monotonic generations make every older response permanently obsolete.
 */
export function createNotificationRequestGeneration() {
	let current = 0;
	return {
		begin: () => {
			current += 1;
			return current;
		},
		invalidate: () => {
			current += 1;
		},
		isCurrent: (generation: number) => generation === current,
	};
}
