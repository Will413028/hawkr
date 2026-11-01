"use client";

import { FOOTER_TEXT_LINK } from "@/components/shared/footerChrome";
import Link from "@/components/shared/Link";
import { useAuth } from "@/lib/auth/context";

/**
 * The footer "登入" entry only makes sense for guests. Once a session is
 * present we drop it, and while the server's answer is still `unknown` nothing is shown
 * (no guest look before the answer). Kept as its own client island so the buyer Footer can
 * stay a server component.
 */
export default function FooterLoginLink({
	className = FOOTER_TEXT_LINK,
}: {
	className?: string;
}) {
	const { status } = useAuth();
	if (status === "unknown" || status === "signedIn") return null;
	return (
		<li>
			<Link href="/login" hardNavigation className={className}>
				登入
			</Link>
		</li>
	);
}
