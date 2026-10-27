"use client";

import HomeCompactSearch from "@/features/home/components/HomeCompactSearch";
import { useAuth } from "@/lib/auth/context";

/**
 * Signed-in members need the full navbar action rail on mobile. Keep search in
 * the home content flow instead, where `home-fixed-section` gives it the exact
 * same horizontal bounds as the category bar directly below it.
 */
export default function HomeMemberMobileSearch() {
	const { hydrated, isLoggedIn } = useAuth();

	if (!hydrated || !isLoggedIn) return null;

	return (
		<section
			className="home-fixed-section relative z-[3] hidden [@media(max-width:840px)]:block"
			aria-label="商品搜尋"
			data-home-member-search
		>
			<HomeCompactSearch className="w-full" recoveryScope="member-mobile" />
		</section>
	);
}
