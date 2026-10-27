"use client";

import * as React from "react";

function PrizeIconReservation() {
	return (
		<span
			aria-hidden="true"
			className="inline-block shrink-0"
			style={{ width: 11, height: 11 }}
		/>
	);
}

// SSR stays enabled: an actual award ships with the original SVG. Sellers
// without an award must not pull the entire icon library into the homepage.
const PrizeGlyph = React.lazy(() => import("./HomeMerchantPrizeGlyph"));

/** The authoritative award text stays outside this decorative-only boundary. */
export class HomeMerchantPrizeIconBoundary extends React.Component<
	React.PropsWithChildren,
	{ failed: boolean }
> {
	state = { failed: false };

	static getDerivedStateFromError() {
		return { failed: true };
	}

	render() {
		return this.state.failed ? <PrizeIconReservation /> : this.props.children;
	}
}

export default function HomeMerchantPrizeIcon() {
	return (
		<HomeMerchantPrizeIconBoundary>
			<React.Suspense fallback={<PrizeIconReservation />}>
				<PrizeGlyph />
			</React.Suspense>
		</HomeMerchantPrizeIconBoundary>
	);
}
