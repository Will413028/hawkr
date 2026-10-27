"use client";

import * as React from "react";
import { useBFCacheReset } from "@/lib/hooks/useBFCacheReset";
import { isApplePayUsable } from "@/lib/payment/apple-pay";
import { isGooglePayUsable } from "@/lib/payment/google-pay";

export interface NativePayAvailability {
	applePay: boolean;
	googlePay: boolean;
	loaded: boolean;
}

export interface NativePayAvailabilityOptions {
	applePay?: boolean;
	googlePay?: boolean;
}

/**
 * Reports which wallet flows the buyer's browser can open. Used by the
 * checkout page to:
 *
 *   - badge supported options as wallet-ready (so buyers know they
 *     won't leave the site), and
 *   - decide at submit-time whether to call the native flow or fall
 *     back to PayUni's hosted page via `redirectUrl`.
 *
 * Detection runs on mount and re-runs on BFCache restore (back/forward
 * nav) so a Google Pay button that briefly went unavailable in a prior
 * navigation can recover without forcing a full refresh. `loaded` gates
 * the badge on first paint so the row doesn't flash in then vanish.
 */
export function useNativePayAvailability({
	applePay: checkApplePay = true,
	googlePay: checkGooglePay = false,
}: NativePayAvailabilityOptions = {}): NativePayAvailability {
	const probeOptionsRef = React.useRef({
		applePay: checkApplePay,
		googlePay: checkGooglePay,
	});
	const [state, setState] = React.useState<NativePayAvailability>({
		applePay: false,
		googlePay: false,
		loaded: false,
	});
	const cancelledRef = React.useRef(false);

	const runDetection = React.useCallback(async () => {
		const { applePay: shouldCheckApplePay, googlePay: shouldCheckGooglePay } =
			probeOptionsRef.current;
		const [applePay, googlePay] = await Promise.all([
			shouldCheckApplePay ? isApplePayUsable() : false,
			shouldCheckGooglePay ? isGooglePayUsable() : false,
		]);
		if (cancelledRef.current) return;
		setState({ applePay, googlePay, loaded: true });
	}, []);

	React.useEffect(() => {
		cancelledRef.current = false;
		void runDetection();
		return () => {
			cancelledRef.current = true;
		};
	}, [runDetection]);

	// BFCache 還原時 component 沒 unmount,既有 effect 不會重跑,上一次造訪
	// 殘留的 loaded=true + googlePay=false 會直接被沿用。重 detection 讓
	// google-pay isReadyToPay 的 retry 邏輯有機會把 transient 失敗收斂回
	// usable=true,避免使用者必須 full refresh 才能恢復 Google Pay 按鈕。
	useBFCacheReset(() => {
		cancelledRef.current = false;
		void runDetection();
	});

	return state;
}
