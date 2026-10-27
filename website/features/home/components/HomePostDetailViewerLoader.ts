import type { ComponentType } from "react";
import type { OpenOriginRect } from "@/features/feed/components/PostDetailViewer";
import type { PostListItem } from "@/lib/api/openapi-types";

export interface HomePostDetailViewerProps {
	posts: PostListItem[];
	initialIndex: number;
	onClose: () => void;
	hasMore?: boolean;
	openRect?: OpenOriginRect | null;
}

interface HomePostDetailViewerModule {
	default: ComponentType<HomePostDetailViewerProps>;
}

export interface HomePostDetailViewerResource {
	peek: () => ComponentType<HomePostDetailViewerProps> | null;
	load: () => Promise<ComponentType<HomePostDetailViewerProps>>;
	prewarm: () => void;
}

/**
 * Keeps the home viewer import stable across hover, open, close, and retry.
 * A failed speculative request is evicted so an explicit retry can recover.
 */
export function createHomePostDetailViewerResource(
	importer: () => Promise<HomePostDetailViewerModule>,
): HomePostDetailViewerResource {
	let loaded: ComponentType<HomePostDetailViewerProps> | null = null;
	let inFlight: Promise<ComponentType<HomePostDetailViewerProps>> | null = null;

	const load = (): Promise<ComponentType<HomePostDetailViewerProps>> => {
		if (loaded) return Promise.resolve(loaded);
		if (inFlight) return inFlight;

		let importRequest: Promise<HomePostDetailViewerModule>;
		try {
			importRequest = importer();
		} catch (error) {
			return Promise.reject(error);
		}

		const imported = importRequest.then((module) => {
			if (!module?.default) {
				throw new Error("Home post detail viewer module has no default export");
			}
			return module.default;
		});
		const tracked = imported.then(
			(Component) => {
				if (inFlight === tracked) {
					loaded = Component;
					inFlight = null;
				}
				return Component;
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
				// Intent prewarming is speculative. A visible open can retry.
			});
		},
	};
}

export const homePostDetailViewerResource = createHomePostDetailViewerResource(
	() => import("@/features/feed/components/PostDetailViewer"),
);

export function prewarmHomePostDetailViewer(): void {
	homePostDetailViewerResource.prewarm();
}
