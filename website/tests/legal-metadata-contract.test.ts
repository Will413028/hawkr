import assert from "node:assert/strict";
import test from "node:test";
import { generateMetadata as privacy } from "../features/legal/pages/PrivacyPage";
import { generateMetadata as returnPolicy } from "../features/legal/pages/ReturnPolicyPage";
import { generateMetadata as sellerTerms } from "../features/legal/pages/SellerTermsPage";
import { generateMetadata as terms } from "../features/legal/pages/TermsPage";
import { generateMetadata as userPolicy } from "../features/legal/pages/UserPolicyPage";

for (const [name, generateMetadata] of Object.entries({
	privacy,
	terms,
	returnPolicy,
	userPolicy,
	sellerTerms,
})) {
	for (const locale of ["zh-TW", "en"]) {
		test(`${name} (${locale}) metadata keeps og:url aligned with canonical`, async () => {
			const metadata = await generateMetadata({
				params: Promise.resolve({ locale }),
			});
			const canonical = metadata.alternates?.canonical;
			assert.ok(canonical, `${name} must declare a canonical URL`);
			assert.equal(metadata.openGraph?.url, canonical);
			assert.ok(
				metadata.openGraph?.images?.length,
				`${name} must keep a share image`,
			);
			// the legal pages stay indexable in every locale (they are platform-written)
			assert.deepEqual(metadata.robots, { index: true, follow: true });
			// the canonical is the page's own locale's URL
			const { pathname } = new URL(String(canonical));
			assert.equal(
				pathname.startsWith("/en/"),
				locale === "en",
				`${name} ${locale}: ${pathname}`,
			);
		});
	}
}
