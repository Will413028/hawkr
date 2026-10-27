import type { ReactNode } from "react";
import Link from "@/components/shared/Link";
import { cn } from "@/lib/utils/cn";
import {
	type ButtonSize,
	type ButtonVariant,
	buttonClasses,
} from "./homeChrome.styles";

type HomeButtonProps = {
	href: string;
	variant?: ButtonVariant;
	size?: ButtonSize;
	onDark?: boolean;
	children: ReactNode;
	className?: string;
};

export default function HomeButton({
	href,
	variant = "primary",
	size = "md",
	onDark = false,
	children,
	className,
}: HomeButtonProps) {
	return (
		<Link
			href={href}
			className={cn(buttonClasses({ variant, size, onDark }), className)}
		>
			{children}
		</Link>
	);
}
