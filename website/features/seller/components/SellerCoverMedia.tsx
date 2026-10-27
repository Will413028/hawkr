import Image from "next/image";
import AppImage from "@/components/shared/AppImage";
import { buildImageUrl } from "@/lib/media/cdn";

interface SellerCoverMediaProps {
	src: string;
	sizes: string;
	quality?: number;
	loading?: "eager" | "lazy";
	className?: string;
	mode?: "cover" | "ambient-contain";
}

/**
 * Canonical seller surfaces use width-filling cover placement so the 4:1
 * banner never leaves side bands. Ambient-contain remains available for
 * legacy previews that must preserve a non-canonical image in full.
 */
export default function SellerCoverMedia({
	src,
	sizes,
	quality = 70,
	loading = "lazy",
	className = "",
	mode = "cover",
}: SellerCoverMediaProps) {
	// The configured media loader exposes named presets rather than arbitrary
	// width variants. This component already receives its final cover URL, so
	// asking next/image for a width ladder only creates duplicate srcset entries
	// and a missing-width warning.
	const unoptimized = true;
	const imageSrc = buildImageUrl(src, "banner");
	if (mode === "cover") {
		return (
			<AppImage
				alt=""
				fill
				src={imageSrc}
				preset="banner"
				sizes={sizes}
				quality={quality}
				loading={loading}
				unoptimized={unoptimized}
				className={className}
				style={{ objectFit: "cover", objectPosition: "center" }}
			/>
		);
	}

	return (
		<div
			className={`absolute inset-0 isolate overflow-hidden bg-[var(--hkui-content2)] ${className}`}
		>
			<Image
				aria-hidden="true"
				alt=""
				fill
				src={imageSrc}
				sizes={sizes}
				quality={quality}
				loading={loading}
				unoptimized={unoptimized}
				className="scale-[1.12] blur-[22px] saturate-[1.12]"
				style={{ objectFit: "cover" }}
			/>
			<span
				aria-hidden="true"
				className="absolute inset-0 bg-[linear-gradient(180deg,rgb(var(--color-ink-950-rgb)/0.08),rgb(var(--color-ink-950-rgb)/0.16))] dark:bg-[linear-gradient(180deg,rgb(var(--color-ink-950-rgb)/0.12),rgb(var(--color-ink-950-rgb)/0.28))]"
			/>
			<AppImage
				alt=""
				fill
				src={imageSrc}
				preset="banner"
				sizes={sizes}
				quality={quality}
				loading={loading}
				unoptimized={unoptimized}
				className="z-[1]"
				style={{ objectFit: "contain", objectPosition: "center" }}
			/>
		</div>
	);
}
