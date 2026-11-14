"use client";

import { loadConnectAndInitialize } from "@stripe/connect-js";
import {
	ConnectAccountOnboarding,
	ConnectComponentsProvider,
} from "@stripe/react-connect-js";
import * as React from "react";

/**
 * 金流商的內嵌開通元件。獨立成檔、由呼叫端動態載入：`@stripe/connect-js` 載入時會向 Stripe 取腳本，
 * 只有賣家真的要在頁面上開通時才該發生。
 */
export default function EmbeddedPaymentOnboarding({
	publishableKey,
	fetchClientSecret,
	onExit,
	onLoadError,
}: {
	publishableKey: string;
	/** 每次呼叫都建立新的 session（元件在 session 到期時會再呼叫）。 */
	fetchClientSecret: () => Promise<string>;
	onExit: () => void;
	onLoadError: () => void;
}) {
	const fetchRef = React.useRef(fetchClientSecret);
	fetchRef.current = fetchClientSecret;
	const [instance] = React.useState(() =>
		loadConnectAndInitialize({
			publishableKey,
			fetchClientSecret: () => fetchRef.current(),
		}),
	);
	return (
		<ConnectComponentsProvider connectInstance={instance}>
			<ConnectAccountOnboarding onExit={onExit} onLoadError={onLoadError} />
		</ConnectComponentsProvider>
	);
}
