"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { ossImageUrl } from "@/lib/media/cdn";

/**
 * Chat-peer avatar with an icon default. While the peer's real avatar is
 * still loading — or when there is none (a brand-new thread shows just
 * `對話對象 #id`) — we paint the "person" icon instead of a bare "?".
 * The real image fades in over it on load and the icon stays put if the URL
 * is empty or 404s.
 */

function normalizeAvatar(src: string | null | undefined): string {
	const trimmed = src?.trim() ?? "";
	if (!trimmed) return "";
	const lower = trimmed.toLowerCase();
	if (lower === "null" || lower === "undefined" || lower === "none") return "";
	return ossImageUrl(trimmed, "avatar");
}

export default function ChatPeerAvatar({
	src,
	alt,
	className,
	fallback = "icon",
}: {
	src?: string | null;
	alt?: string;
	className?: string;
	fallback?: "icon" | "empty";
}) {
	const resolved = React.useMemo(() => normalizeAvatar(src), [src]);
	const [loaded, setLoaded] = React.useState(false);
	const [broken, setBroken] = React.useState(false);

	// Reset load/error state whenever the source changes so switching threads
	// shows the icon default again until the new image arrives.
	const [lastSrc, setLastSrc] = React.useState(resolved);
	if (lastSrc !== resolved) {
		setLastSrc(resolved);
		setLoaded(false);
		setBroken(false);
	}

	const showImage = resolved !== "" && !broken;

	return (
		<span
			className={`${className ?? ""} relative grid place-items-center overflow-hidden`}
			role="img"
			aria-label={alt || "對話對象"}
		>
			{fallback === "icon" && (!showImage || !loaded) ? (
				<AppIcon name="user" size={28} className="text-[var(--hkui-primary)]" />
			) : null}
			{showImage ? (
				// biome-ignore lint/performance/noImgElement: peer avatars are user-supplied CDN paths, not known at build time.
				<img
					src={resolved}
					alt={alt ?? ""}
					decoding="async"
					referrerPolicy="no-referrer"
					onLoad={() => setLoaded(true)}
					onError={() => setBroken(true)}
					className="absolute inset-0 h-full w-full object-cover transition-opacity duration-200"
					style={{ opacity: loaded ? 1 : 0 }}
				/>
			) : null}
		</span>
	);
}
