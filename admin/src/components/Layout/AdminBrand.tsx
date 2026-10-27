import Image from "next/image";

interface AdminBrandProps {
	tone?: "dark" | "light";
	variant?: "horizontal" | "mark";
	priority?: boolean;
	className?: string;
}

export default function AdminBrand({
	tone = "dark",
	variant = "horizontal",
	priority = false,
	className = "",
}: AdminBrandProps) {
	const isMark = variant === "mark";
	const src = isMark
		? "/brand/logo-mark.svg"
		: tone === "light"
			? "/brand/logo-horizontal-light.svg"
			: "/brand/logo-horizontal.svg";

	return (
		<span
			className={`admin-brand admin-brand--${variant} admin-brand--${tone} ${className}`.trim()}
		>
			<Image
				src={src}
				alt="Hawkr"
				width={isMark ? 120 : 502}
				height={120}
				priority={priority}
			/>
		</span>
	);
}
