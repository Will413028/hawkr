/** @jsxRuntime automatic */
/** @jsxImportSource react */
import type * as React from "react";
import {
	MATERIAL_SYMBOL_PATHS,
	MATERIAL_SYMBOL_VIEW_BOX,
} from "./material-symbols.gen";

export type MaterialSymbolName = keyof typeof MATERIAL_SYMBOL_PATHS;

export interface MaterialSymbolProps
	extends Omit<React.SVGProps<SVGSVGElement>, "name" | "fill"> {
	name: MaterialSymbolName;
	/** Square edge length: a number is px, a string is any CSS length. Defaults to `"1em"` (follows the font size). */
	size?: number | string;
	/** Use the filled variant. */
	fill?: boolean;
	/** Accessible name. When omitted the icon is decorative (`aria-hidden`). */
	title?: string;
}

export function MaterialSymbol({
	name,
	size = "1em",
	fill = false,
	title,
	...rest
}: MaterialSymbolProps) {
	const glyph = MATERIAL_SYMBOL_PATHS[name];
	return (
		<svg
			xmlns="http://www.w3.org/2000/svg"
			// Spread first: the decorative contract below is this component's own.
			{...rest}
			width={size}
			height={size}
			viewBox={MATERIAL_SYMBOL_VIEW_BOX}
			fill="currentColor"
			role={title ? "img" : undefined}
			aria-label={title}
			aria-hidden={title ? undefined : true}
		>
			{/* The path carries its own paint so a consumer's `svg { fill: none; stroke: … }`
					rule for line icons cannot hollow the glyph out. */}
			<path
				d={fill ? glyph.filled : glyph.outlined}
				fill="currentColor"
				stroke="none"
			/>
		</svg>
	);
}
