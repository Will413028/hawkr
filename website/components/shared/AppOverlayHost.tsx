"use client";

import * as React from "react";
import LoginRequiredModal from "@/components/shared/auth/LoginRequiredModal";
import { usePathname } from "@/lib/intl/navigation";
import {
	type LoginRequiredOverlayPayload,
	useAppUiStore,
} from "@/lib/state/app-ui-store";

function isLoginRequiredPayload(
	payload: unknown,
): payload is LoginRequiredOverlayPayload {
	return payload == null || typeof payload === "object";
}

export default function AppOverlayHost() {
	const overlay = useAppUiStore((state) => state.overlay);
	const closeOverlay = useAppUiStore((state) => state.closeOverlay);
	const pathname = usePathname() ?? "";

	// This host lives in the persistent AppProviders, so it survives ordinary
	// soft navigations. The login CTA now crosses the auth boundary with a hard
	// navigation (which resets this in-memory store), but every other route change
	// still needs this cleanup so a global overlay never outlives its source page.
	// `pathname` is the trigger (not a value read): it must stay in the deps so
	// the effect re-runs on navigation; dropping it would only run on mount.
	// biome-ignore lint/correctness/useExhaustiveDependencies: pathname is an intentional re-run trigger
	React.useEffect(() => {
		// Skip the mount pass and any run where nothing is open: the store write
		// is otherwise repeated on every single navigation for no reason.
		if (!useAppUiStore.getState().overlay) return;
		closeOverlay();
	}, [pathname, closeOverlay]);

	if (!overlay) return null;
	if (overlay.name !== "login-required") {
		if (process.env.NODE_ENV !== "production") {
			console.warn(
				`[AppOverlayHost] 未知的 overlay「${overlay.name}」，沒有對應的元件可渲染。`,
			);
		}
		return null;
	}
	const payload = isLoginRequiredPayload(overlay.payload)
		? overlay.payload
		: {};

	return (
		<LoginRequiredModal
			open
			onClose={closeOverlay}
			redirect={payload.redirect ?? "/"}
			title={payload.title}
			description={payload.description}
			actionLabel={payload.actionLabel}
		/>
	);
}
