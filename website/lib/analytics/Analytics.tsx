"use client";

import Script from "next/script";
import { Suspense, useEffect, useState } from "react";
import { getEnv } from "@/lib/config/env";
import { whenDocumentSettled } from "@/lib/hydration/document-settled";
import { bootstrapGtag } from "./gtag";
import NavigationEvents from "./NavigationEvents";
import { useAnalyticsIdentity } from "./useAnalyticsIdentity";

/**
 * Single GA4 mount point. Navigation lifecycle tracking remains mounted when
 * gaId is unset; only GA identity/events/script delivery are disabled.
 * Must live inside <AuthProvider> so the identity bridge can read useAuth().
 * No consent-default command is emitted (full analytics; see gtag.setConsent
 * for the future banner hook).
 */
export default function Analytics() {
	const gaId = getEnv().gaId;
	const [documentSettled, setDocumentSettled] = useState(false);
	useAnalyticsIdentity();

	useEffect(() => {
		if (!gaId) return;

		let active = true;
		bootstrapGtag(gaId);

		void whenDocumentSettled().then(() => {
			if (active) setDocumentSettled(true);
		});

		return () => {
			active = false;
		};
	}, [gaId]);

	return (
		<>
			<Suspense fallback={null}>
				<NavigationEvents />
			</Suspense>
			{gaId && documentSettled ? (
				<Script
					id="hk-ga"
					src={`https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(gaId)}`}
					strategy="lazyOnload"
				/>
			) : null}
		</>
	);
}
