"use client";

import Avatar from "@/components/shared/Avatar";
import { cn } from "@/lib/utils/cn";

type SellerAvatarImageProps = {
	src?: string | null;
	alt?: string;
	className?: string;
	loading?: "eager" | "lazy";
	/**
	 * Seller workspace identity chrome can render transparent merchant logos.
	 * Give those logos one canonical backing surface so the same image does not
	 * look different in the header and sidebar merely because their wrappers
	 * use different contextual backgrounds.
	 */
	variant?: "default" | "identity";
};

const SELLER_IDENTITY_IMAGE =
	"seller-identity-avatar-image bg-[var(--hkui-primary)]";

/** Shared seller identity image. Missing or broken URLs keep an empty,
 * aria-hidden slot; seller chrome must never substitute a placeholder image or
 * generated initial for the backend-selected logo.
 *
 * ⚠️ `alt` defaults to `""` (decorative) because most call sites render the shop
 * name next to the logo, where a second copy is noise. On a surface where the
 * logo IS the shop's identity (the storefront hero), pass the shop name — the
 * default cannot tell those two cases apart. */
export default function SellerAvatarImage({
	src,
	alt = "",
	className,
	loading,
	variant = "default",
}: SellerAvatarImageProps) {
	return (
		<Avatar
			src={src}
			alt={alt}
			className={cn(className, variant === "identity" && SELLER_IDENTITY_IMAGE)}
			entity="seller"
			fallback="empty"
			loading={loading}
		/>
	);
}
