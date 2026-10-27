/**
 * Line icons for the content pages (FAQ, seller guide).
 *
 * Extracted from `FaqPage` so both pages draw from one set — same 24px grid,
 * same 2px stroke, `currentColor` by default so the caller controls colour.
 */
export default function ContentIcon({
	icon,
	stroke = "currentColor",
}: {
	icon: string;
	stroke?: string;
}) {
	// aria-hidden is written literally on each <svg> rather than spread in:
	// Biome's noSvgWithoutTitle only sees statically-present attributes.
	const common = {
		width: 20,
		height: 20,
		viewBox: "0 0 24 24",
		fill: "none",
		stroke,
		strokeWidth: 2,
	};

	switch (icon) {
		case "user":
			return (
				<svg aria-hidden="true" {...common}>
					<path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2" />
					<circle cx="12" cy="7" r="4" />
				</svg>
			);
		case "store":
			return (
				<svg aria-hidden="true" {...common}>
					<path d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z" />
					<polyline points="9 22 9 12 15 12 15 22" />
				</svg>
			);
		case "vegetable":
			return (
				<svg aria-hidden="true" {...common}>
					<path d="M12 2C8 2 4 6 4 10c0 6 8 12 8 12s8-6 8-12c0-4-4-8-8-8z" />
					<path d="M12 6v8M9 10h6" />
				</svg>
			);
		case "order":
			return (
				<svg aria-hidden="true" {...common}>
					<path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2" />
					<rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
					{stroke === "currentColor" && (
						<>
							<line x1="8" y1="10" x2="16" y2="10" />
							<line x1="8" y1="14" x2="16" y2="14" />
						</>
					)}
				</svg>
			);
		case "wallet":
			return (
				<svg aria-hidden="true" {...common}>
					<rect x="1" y="4" width="22" height="16" rx="2" ry="2" />
					<line x1="1" y1="10" x2="23" y2="10" />
				</svg>
			);
		case "follow":
			return (
				<svg aria-hidden="true" {...common}>
					<path d="M16 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" />
					<circle cx="8.5" cy="7" r="4" />
					<line x1="20" y1="8" x2="20" y2="14" />
					<line x1="23" y1="11" x2="17" y2="11" />
				</svg>
			);
		case "chat":
			return (
				<svg aria-hidden="true" {...common}>
					<path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" />
				</svg>
			);
		case "coupon":
			return (
				<svg aria-hidden="true" {...common}>
					<rect x="1" y="4" width="22" height="16" rx="2" />
					<line x1="1" y1="10" x2="23" y2="10" />
				</svg>
			);
		case "notify":
			return (
				<svg aria-hidden="true" {...common}>
					<path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9" />
					<path d="M13.73 21a2 2 0 01-3.46 0" />
				</svg>
			);
		case "dashboard":
			return (
				<svg aria-hidden="true" {...common}>
					<rect x="3" y="3" width="7" height="9" rx="1.5" />
					<rect x="14" y="3" width="7" height="5" rx="1.5" />
					<rect x="14" y="12" width="7" height="9" rx="1.5" />
					<rect x="3" y="16" width="7" height="5" rx="1.5" />
				</svg>
			);
		case "chart":
			return (
				<svg aria-hidden="true" {...common}>
					<line x1="4" y1="20" x2="20" y2="20" />
					<rect x="6" y="12" width="3" height="6" rx="1" />
					<rect x="11" y="8" width="3" height="10" rx="1" />
					<rect x="16" y="4" width="3" height="14" rx="1" />
				</svg>
			);
		case "truck":
			return (
				<svg aria-hidden="true" {...common}>
					<path d="M3 7h11v9H3z" />
					<path d="M14 10h4l3 3v3h-7z" />
					<circle cx="7" cy="18" r="2" />
					<circle cx="17" cy="18" r="2" />
				</svg>
			);
		default:
			return (
				<svg aria-hidden="true" {...common}>
					<circle cx="12" cy="12" r="10" />
					<path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" />
					<line x1="12" y1="17" x2="12.01" y2="17" />
				</svg>
			);
	}
}
