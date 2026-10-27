"use client";

import { useEffect } from "react";
import { useAuth } from "@/lib/auth/context";
import { roleProperty } from "./events";
import { setUserProperties } from "./gtag";

/**
 * Bridges auth state → a categorical GA4 role property. Stable Hawkr
 * account identifiers are intentionally not sent to preserve the public
 * privacy notice's anonymous-statistics boundary.
 */
export function useAnalyticsIdentity(): void {
	const { identity, activeRole, hydrated } = useAuth();

	useEffect(() => {
		if (!hydrated) return;
		setUserProperties(roleProperty(identity ? activeRole : null));
	}, [identity, activeRole, hydrated]);
}
