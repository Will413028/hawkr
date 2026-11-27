import type { Metadata } from "next";
import * as React from "react";
import SellerOnboardingClient from "@/features/seller/onboarding/SellerOnboardingClient";
import { noindexMetadata } from "@/lib/seo/metadata";

export const metadata: Metadata = {
	...noindexMetadata(),
	title: "開店申請",
};

export default function SellerOnboardingPage() {
	return (
		<React.Suspense fallback={null}>
			<SellerOnboardingClient />
		</React.Suspense>
	);
}
