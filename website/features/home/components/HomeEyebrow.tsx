import type { ReactNode } from "react";
import { cn } from "@/lib/utils/cn";
import { eyebrowClasses, eyebrowDotClasses } from "./homeChrome.styles";

type HomeEyebrowProps = {
	children: ReactNode;
	onDark?: boolean;
	className?: string;
};

export default function HomeEyebrow({
	children,
	onDark = false,
	className,
}: HomeEyebrowProps) {
	return (
		<span className={cn(eyebrowClasses(onDark), className)}>
			<span className={eyebrowDotClasses} aria-hidden="true" />
			{children}
		</span>
	);
}
