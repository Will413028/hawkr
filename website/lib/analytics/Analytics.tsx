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
		// This only initializes an in-memory dataLayer queue. It does not insert a
		// script node, so the first page_view and identity commands can be ordered
		// and retained even when full window.load is delayed by eager media.
		bootstrapGtag(gaId);

		// next/script appends its node to <body> from an effect. Gate only that
		// unmanaged DOM insertion; NavigationEvents can safely queue page_view now.
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
				// lazyOnload：節點插入已被 whenDocumentSettled 擋到 load + loader 交棒
				// 之後，這裡再把「開始下載 + 執行」排到瀏覽器閒置時段。gtag/js 連帶
				// 的 GTM 容器實測傳輸約 700KB、主執行緒 120ms（2026-09-04 手機模擬），
				// afterInteractive 會讓它跟首屏之後的圖片與 hydration 搶頻寬與 CPU。
				// 事件不會丟：bootstrapGtag 早已建好 dataLayer 佇列，page_view 與身分
				// 指令在 script 抵達前就排好序，抵達後一次送出。
				<Script
					id="hk-ga"
					src={`https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(gaId)}`}
					strategy="lazyOnload"
				/>
			) : null}
		</>
	);
}
