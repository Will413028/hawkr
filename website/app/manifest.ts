import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
	return {
		name: "Hawkr — 在地職人的市集",
		short_name: "Hawkr",
		description: "在地小農與職人的生鮮市集，當季蔬果、海鮮、肉品產地直送。",
		start_url: "/",
		display: "standalone",
		background_color: "#fffdf7",
		theme_color: "#b8441f",
		icons: [
			{ src: "/brand/logo-mark.svg", sizes: "any", type: "image/svg+xml" },
		],
	};
}
