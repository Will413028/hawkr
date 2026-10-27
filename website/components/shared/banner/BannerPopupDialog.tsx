"use client";

import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import { Dialog } from "@/components/ui/Dialog";
import {
	bannerActionLabel,
	resolveBannerActionHref,
} from "@/lib/banners/action";
import type { Banner } from "@/lib/banners/repo";

export default function BannerPopupDialog({
	banner,
	onClose,
}: {
	banner: Banner | null;
	onClose: () => void;
}) {
	if (!banner) return null;
	const href = resolveBannerActionHref(banner.action);
	const label = banner.buttonText || bannerActionLabel(banner.action);
	const ctaClass =
		"inline-flex min-h-11 w-full items-center justify-center rounded-hk-pill bg-hk-primary px-5 py-2.5 font-semibold text-white no-underline transition-opacity hover:opacity-90";
	const footer =
		href && label ? (
			href.startsWith("/") ? (
				<Link href={href} className={ctaClass} onClick={onClose}>
					{label}
				</Link>
			) : (
				<a
					href={href}
					className={ctaClass}
					rel="noopener noreferrer"
					target="_blank"
					// Same dismissal as the internal branch: leaving the dialog open
					// behind a new tab means coming back to a modal the user already
					// finished with.
					onClick={onClose}
				>
					{label}
					<span className="sr-only">（另開新視窗）</span>
				</a>
			)
		) : undefined;

	return (
		<Dialog
			open
			onClose={onClose}
			title={banner.title}
			size="sm"
			footer={footer}
			className="max-w-[min(360px,calc(100vw-2rem))]"
		>
			<div className="relative aspect-[360/404] w-full overflow-hidden rounded-hk-lg bg-hk-surface-muted">
				<AppImage
					image={banner.popupImage}
					alt={banner.title}
					fill
					preset="banner"
					sizes="(max-width: 420px) calc(100vw - 4rem), 360px"
					className="object-cover"
				/>
			</div>
		</Dialog>
	);
}
