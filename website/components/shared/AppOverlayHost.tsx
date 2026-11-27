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

	// still needs this cleanup so a global overlay never outlives its source page.
	// biome-ignore lint/correctness/useExhaustiveDependencies: pathname is an intentional re-run trigger
	React.useEffect(() => {
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
