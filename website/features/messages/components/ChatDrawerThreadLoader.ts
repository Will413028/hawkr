import type { ComponentType } from "react";

export interface ChatDrawerThreadProps {
	targetId: string;
	embedded?: boolean;
	peerNameHint?: string;
	peerAvatarHint?: string;
}

interface ChatDrawerThreadModule {
	default: ComponentType<ChatDrawerThreadProps>;
}

export interface ChatDrawerThreadResource {
	peek: () => ComponentType<ChatDrawerThreadProps> | null;
	load: () => Promise<ComponentType<ChatDrawerThreadProps>>;
	prewarm: () => void;
}

/**
 * Keeps the drawer's code-split request stable across close/reopen cycles.
 * Concurrent opens share one request, successful modules stay warm, and a
 * rejected request is released so the visible retry can make a fresh attempt.
 */
export function createChatDrawerThreadResource(
	importer: () => Promise<ChatDrawerThreadModule>,
): ChatDrawerThreadResource {
	let loaded: ComponentType<ChatDrawerThreadProps> | null = null;
	let inFlight: Promise<ComponentType<ChatDrawerThreadProps>> | null = null;

	const load = (): Promise<ComponentType<ChatDrawerThreadProps>> => {
		if (loaded) return Promise.resolve(loaded);
		if (inFlight) return inFlight;

		let importRequest: Promise<ChatDrawerThreadModule>;
		try {
			importRequest = importer();
		} catch (error) {
			return Promise.reject(error);
		}
		const imported = importRequest.then((module) => {
			if (!module?.default) {
				throw new Error("Chat drawer thread module has no default export");
			}
			return module.default;
		});
		const tracked = imported.then(
			(component) => {
				if (inFlight === tracked) {
					loaded = component;
					inFlight = null;
				}
				return component;
			},
			(error: unknown) => {
				if (inFlight === tracked) inFlight = null;
				throw error;
			},
		);
		inFlight = tracked;
		return tracked;
	};

	return {
		peek: () => loaded,
		load,
		prewarm: () => {
			void load().catch(() => {
				// Intent prewarming is speculative. A later open retries visibly.
			});
		},
	};
}

export const chatDrawerThreadResource = createChatDrawerThreadResource(
	() => import("@/features/account/screens/messages/ThreadClient"),
);

/**
 * Optional intent hook for an owning entry point (pointer/focus). It never
 * mounts the thread or starts chat subscriptions, and speculative failures do
 * not become unhandled rejections.
 */
export function prewarmChatDrawerThread(): void {
	chatDrawerThreadResource.prewarm();
}
