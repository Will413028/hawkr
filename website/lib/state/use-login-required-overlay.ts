"use client";

import * as React from "react";
import {
	type LoginRequiredOverlayPayload,
	useAppUiStore,
} from "@/lib/state/app-ui-store";

function currentRedirect(): string {
	if (typeof window === "undefined") return "/";
	return `${window.location.pathname}${window.location.search}`;
}

export function useLoginRequiredOverlay(
	defaultPayload: LoginRequiredOverlayPayload = {},
) {
	const openLoginRequired = useAppUiStore((state) => state.openLoginRequired);

	return React.useCallback(
		(payload: LoginRequiredOverlayPayload = {}) => {
			openLoginRequired({
				redirect: currentRedirect(),
				...defaultPayload,
				...payload,
			});
		},
		[defaultPayload, openLoginRequired],
	);
}
