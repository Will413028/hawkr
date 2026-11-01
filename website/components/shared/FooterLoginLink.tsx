"use client";

import { FOOTER_TEXT_LINK } from "@/components/shared/footerChrome";
import Link from "@/components/shared/Link";
import { useAuth } from "@/lib/auth/context";

/**
 * The footer "登入" entry only makes sense for guests. Once a session is
 * present we drop it (a signed-in session and the `unknown` state both resolve
 * before this hides, so the login link may show until the answer arrives). Kept as its own client island so the buyer Footer can
 * stay a server component.
 */
export default function FooterLoginLink({
	className = FOOTER_TEXT_LINK,
}: {
	className?: string;
}) {
	const { isLoggedIn } = useAuth();
	if (isLoggedIn) return null;
	return (
		<li>
			<Link href="/login" hardNavigation className={className}>
				登入
			</Link>
		</li>
	);
}
