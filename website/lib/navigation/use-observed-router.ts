"use client";

import * as React from "react";
import { useRouter } from "@/lib/intl/use-router";
import {
	beginNavigationIntent,
	cancelNavigationIntent,
	declareRouterNavigationOwner,
	navigationActionId,
} from "@/lib/navigation/observability";
import {
	armProgrammaticRouteMotion,
	cancelProgrammaticRouteMotion,
} from "@/lib/navigation/programmatic-route-motion";

/**
 * Semantic wrapper for direct App Router calls.
 *
 * Link clicks declare their component/action at the shared anchor layer.
 * Programmatic navigation has no DOM target, so callers use this hook to attach
 * stable ownership before Next emits the global router-transition start signal.
 * The returned object remains AppRouterInstance-compatible.
 */
export function useObservedRouter(componentId: string) {
	const router = useRouter();

	return React.useMemo(
		() => ({
			...router,
			push: (...args: Parameters<typeof router.push>) => {
				const intentId = beginNavigationIntent({
					target: args[0],
					trigger: "router",
					componentId,
					actionId: navigationActionId("push", args[0]),
				});
				const motionToken = armProgrammaticRouteMotion(args[0]);
				try {
					return router.push(...args);
				} catch (error) {
					cancelNavigationIntent(intentId);
					cancelProgrammaticRouteMotion(motionToken);
					throw error;
				}
			},
			replace: (...args: Parameters<typeof router.replace>) => {
				const intentId = beginNavigationIntent({
					target: args[0],
					trigger: "router",
					componentId,
					actionId: navigationActionId("replace", args[0]),
					replace: true,
				});
				const motionToken = armProgrammaticRouteMotion(args[0]);
				try {
					return router.replace(...args);
				} catch (error) {
					cancelNavigationIntent(intentId);
					cancelProgrammaticRouteMotion(motionToken);
					throw error;
				}
			},
			back: () => {
				declareRouterNavigationOwner(componentId, navigationActionId("back"));
				return router.back();
			},
			forward: () => {
				declareRouterNavigationOwner(
					componentId,
					navigationActionId("forward"),
				);
				return router.forward();
			},
			// Refresh keeps the same route and is not a navigation intent. In
			// particular, a refresh immediately following push/replace must not
			// supersede the still-pending destination transaction.
			refresh: () => {
				return router.refresh();
			},
		}),
		[componentId, router],
	);
}
