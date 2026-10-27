type Check = (ids: string[]) => Promise<ReadonlySet<string>>;

interface Batch {
	ids: Map<
		string,
		Array<{
			resolve: (value: boolean) => void;
			reject: (error: unknown) => void;
		}>
	>;
}

// Queries mounted in the same render are sent together, up to the API's 100-ID cap.
export function createMembershipBatcher(check: Check) {
	const pending = new Map<string, Batch>();

	return (scope: string, id: string): Promise<boolean> =>
		new Promise((resolve, reject) => {
			let batch = pending.get(scope);
			if (!batch) {
				const created: Batch = { ids: new Map() };
				batch = created;
				pending.set(scope, created);
				queueMicrotask(async () => {
					pending.delete(scope);
					const entries = [...created.ids.entries()];
					try {
						for (let start = 0; start < entries.length; start += 100) {
							const chunk = entries.slice(start, start + 100);
							const matched = await check(chunk.map(([value]) => value));
							for (const [value, listeners] of chunk) {
								for (const listener of listeners)
									listener.resolve(matched.has(value));
							}
						}
					} catch (error) {
						for (const [, listeners] of entries) {
							for (const listener of listeners) listener.reject(error);
						}
					}
				});
			}
			const listeners = batch.ids.get(id) ?? [];
			listeners.push({ resolve, reject });
			batch.ids.set(id, listeners);
		});
}
