"use client";

import * as React from "react";
import type { AppIconProps } from "@/components/shared/AppIcon";

type AppIconComponent = typeof import("@/components/shared/AppIcon").default;

let loadedIcon: AppIconComponent | null = null;
let pendingIcon: Promise<AppIconComponent> | null = null;

function loadAppIcon(): Promise<AppIconComponent> {
	if (loadedIcon) return Promise.resolve(loadedIcon);
	pendingIcon ??= import("@/components/shared/AppIcon").then((module) => {
		loadedIcon = module.default;
		return module.default;
	});
	return pendingIcon;
}

/**
 * `AppIcon` for modules that must not import the icon library statically
 * (the guest and buyer navbar entry). Until the chunk resolves an inert box of
 * the final size holds the layout, so nothing reflows when the glyph appears.
 */
export default function LazyAppIcon(props: AppIconProps) {
	const [Icon, setIcon] = React.useState<AppIconComponent | null>(
		() => loadedIcon,
	);

	React.useEffect(() => {
		if (Icon) return;
		let mounted = true;
		void loadAppIcon().then((component) => {
			if (mounted) setIcon(() => component);
		});
		return () => {
			mounted = false;
		};
	}, [Icon]);

	if (!Icon) {
		const { size = 24, className, style } = props;
		return (
			<span
				aria-hidden="true"
				className={className}
				style={{ width: size, height: size, display: "block", ...style }}
			/>
		);
	}

	return <Icon {...props} />;
}
