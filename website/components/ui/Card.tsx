import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import { cn } from "@/lib/utils/cn";

type CardShadow = "none" | "sm" | "md" | "lg";
type CardRadius = "sm" | "md" | "lg" | "xl";

interface CardOwnProps {
	shadow?: CardShadow;
	radius?: CardRadius;
	isHoverable?: boolean;
	isPressable?: boolean;
	isBlurred?: boolean;
}

type CardProps<T extends React.ElementType = "div"> = CardOwnProps & {
	as?: T;
} & Omit<React.ComponentPropsWithRef<T>, keyof CardOwnProps | "as">;

type CardComponent = <T extends React.ElementType = "div">(
	props: CardProps<T>,
) => React.ReactElement | null;

/** @import import { Card } from '@/components/ui/Card'; */
export const Card = React.forwardRef(function Card(
	{
		shadow = "md",
		radius = "lg",
		isHoverable = false,
		isPressable = false,
		isBlurred = false,
		as: Tag = "div",
		className = "",
		children,
		onKeyDown,
		...rest
	}: CardProps<React.ElementType>,
	ref: React.ForwardedRef<HTMLElement>,
) {
	const classes = cn(
		"relative overflow-hidden border border-[var(--hkui-border)] bg-[var(--hkui-surface)] text-[var(--hkui-foreground)] transition-[box-shadow,transform,border-color,background] duration-200 ease-[var(--hkui-ease)]",
		cardShadowClass[shadow],
		cardRadiusClass[radius],
		isHoverable &&
			"hover:-translate-y-0.5 hover:shadow-[var(--hkui-shadow-lg)] motion-reduce:translate-none motion-reduce:scale-100",
		isPressable &&
			"cursor-pointer select-none hover:-translate-y-0.5 active:translate-y-0 motion-reduce:translate-none motion-reduce:scale-100 focus-visible:outline-none focus-visible:[box-shadow:var(--hkui-focus-ring)]",
		isBlurred &&
			"border-[var(--page-card-blur-border)] bg-[var(--hkui-surface)] backdrop-blur-xl",
		className,
	);

	const Component = Tag;

	// `role="button"` + `tabIndex={0}` promises keyboard operation, but a <div>
	// does not fire click on Enter/Space the way a native button does — the
	// card announced itself as a button and then ignored the keyboard. Forward
	// the key to a real click so the caller's own `onClick` runs unchanged.
	// (No `use client` needed: the handler is only attached when the caller
	// already opted into an interactive card.)
	const handleKeyDown = (event: React.KeyboardEvent<HTMLElement>) => {
		onKeyDown?.(event);
		if (event.defaultPrevented) return;
		if (event.key !== "Enter" && event.key !== " ") return;
		if (event.target !== event.currentTarget) return;
		event.preventDefault();
		event.currentTarget.click();
	};

	return (
		<Component
			ref={ref}
			className={classes}
			role={isPressable ? "button" : undefined}
			tabIndex={isPressable ? 0 : undefined}
			onKeyDown={isPressable ? handleKeyDown : onKeyDown}
			{...rest}
		>
			{children}
		</Component>
	);
}) as CardComponent;

/** @import import { CardHeader } from '@/components/ui/Card'; */
export const CardHeader = React.forwardRef<
	HTMLDivElement,
	React.HTMLAttributes<HTMLDivElement>
>(function CardHeader({ className = "", children, ...rest }, ref) {
	return (
		<div
			ref={ref}
			className={cn("flex items-center gap-3 p-5", className)}
			{...rest}
		>
			{children}
		</div>
	);
});

/** @import import { CardBody } from '@/components/ui/Card'; */
export const CardBody = React.forwardRef<
	HTMLDivElement,
	React.HTMLAttributes<HTMLDivElement>
>(function CardBody({ className = "", children, ...rest }, ref) {
	return (
		<div ref={ref} className={cn("p-5", className)} {...rest}>
			{children}
		</div>
	);
});

/** @import import { CardFooter } from '@/components/ui/Card'; */
export const CardFooter = React.forwardRef<
	HTMLDivElement,
	React.HTMLAttributes<HTMLDivElement>
>(function CardFooter({ className = "", children, ...rest }, ref) {
	return (
		<div
			ref={ref}
			className={cn(
				"flex items-center gap-3 border-t border-[var(--hkui-border)] p-5",
				className,
			)}
			{...rest}
		>
			{children}
		</div>
	);
});

interface CardMediaProps extends React.HTMLAttributes<HTMLDivElement> {
	src: string;
	/** Required: product / media artwork carries meaning. Pass `alt=""` only
	 *  when the image really is decorative and the name is already in text. */
	alt: string;
	ratio?: "square" | "video" | "portrait";
	sizes?: string;
	imgProps?: Omit<
		React.ImgHTMLAttributes<HTMLImageElement>,
		"src" | "alt" | "loading" | "width" | "height"
	>;
}

export const CardMedia = React.forwardRef<HTMLDivElement, CardMediaProps>(
	function CardMedia(
		{
			src,
			alt,
			ratio = "video",
			sizes = "(max-width: 768px) 100vw, 33vw",
			imgProps,
			className = "",
			children,
			...rest
		},
		ref,
	) {
		return (
			<div
				ref={ref}
				className={cn(
					"relative w-full overflow-hidden bg-[var(--hkui-default-100)]",
					cardMediaRatioClass[ratio],
					className,
				)}
				{...rest}
			>
				<AppImage
					src={src}
					alt={alt}
					fill
					sizes={sizes}
					{...imgProps}
					className={cn("object-cover", imgProps?.className)}
				/>
				{children}
			</div>
		);
	},
);

const cardShadowClass: Record<CardShadow, string> = {
	none: "shadow-none",
	sm: "shadow-[var(--hkui-shadow-sm)]",
	md: "shadow-[var(--hkui-shadow-md)]",
	lg: "shadow-[var(--hkui-shadow-lg)]",
};

const cardRadiusClass: Record<CardRadius, string> = {
	sm: "rounded-[var(--hkui-radius-sm)]",
	md: "rounded-[var(--hkui-radius-md)]",
	lg: "rounded-[var(--hkui-radius-lg)]",
	xl: "rounded-[var(--hkui-radius-xl)]",
};

const cardMediaRatioClass: Record<
	NonNullable<CardMediaProps["ratio"]>,
	string
> = {
	square: "aspect-square",
	video: "aspect-video",
	portrait: "aspect-[3/4]",
};

export type { CardMediaProps, CardProps, CardRadius, CardShadow };
