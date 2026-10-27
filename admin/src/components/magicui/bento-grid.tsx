import type React from "react";
import { cn } from "../../lib/utils";

interface BentoGridProps {
	children: React.ReactNode;
	className?: string;
}

export const BentoGrid: React.FC<BentoGridProps> = ({
	children,
	className,
}) => {
	return (
		<div
			className={cn(
				"grid w-full auto-rows-[22rem] grid-cols-3 gap-4",
				className,
			)}
		>
			{children}
		</div>
	);
};

interface BentoCardProps {
	name: string;
	description?: string;
	className?: string;
	background?: React.ReactNode;
	Icon?: React.ElementType;
	href?: string;
	cta?: string;
	children?: React.ReactNode;
	onClick?: () => void;
}

export const BentoCard: React.FC<BentoCardProps> = ({
	name,
	description,
	className,
	background,
	Icon,
	href,
	cta,
	children,
	onClick,
}) => {
	const Wrapper = href ? "a" : "div";

	return (
		<Wrapper
			href={href}
			onClick={onClick}
			className={cn(
				"group relative col-span-3 flex flex-col justify-between overflow-hidden rounded-xl",
				"bg-white [box-shadow:0_0_0_1px_rgba(0,0,0,.03),0_2px_4px_rgba(0,0,0,.05),0_12px_24px_rgba(0,0,0,.05)]",
				"transform-gpu transition-all duration-300 hover:scale-[1.01] hover:[box-shadow:0_0_0_1px_rgba(0,0,0,.03),0_4px_8px_rgba(0,0,0,.08),0_16px_32px_rgba(0,0,0,.08)]",
				onClick || href ? "cursor-pointer" : "",
				className,
			)}
		>
			{background && (
				<div className="pointer-events-none absolute inset-0 overflow-hidden">
					{background}
				</div>
			)}
			<div className="pointer-events-none z-10 flex transform-gpu flex-col gap-1 p-6 transition-all duration-300 group-hover:-translate-y-2">
				{Icon && (
					<Icon className="h-12 w-12 origin-left transform-gpu text-neutral-700 transition-all duration-300 ease-in-out group-hover:scale-75" />
				)}
				<h3 className="text-xl font-semibold text-neutral-700">{name}</h3>
				{description && (
					<p className="max-w-lg text-neutral-400">{description}</p>
				)}
			</div>
			{children}
			{cta && (
				<div
					className={cn(
						"pointer-events-none absolute bottom-0 flex w-full translate-y-10 transform-gpu flex-row items-center p-4 opacity-0 transition-all duration-300 group-hover:translate-y-0 group-hover:opacity-100",
					)}
				>
					<span className="text-sm font-medium text-neutral-700">{cta}</span>
					<svg
						className="ml-2 h-4 w-4"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
						aria-hidden="true"
					>
						<path
							strokeLinecap="round"
							strokeLinejoin="round"
							strokeWidth={2}
							d="M17 8l4 4m0 0l-4 4m4-4H3"
						/>
					</svg>
				</div>
			)}
		</Wrapper>
	);
};
