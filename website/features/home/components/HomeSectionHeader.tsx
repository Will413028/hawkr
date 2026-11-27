import type { ReactNode } from "react";
import Link from "@/components/shared/Link";
import HomeEyebrow from "./HomeEyebrow";
import {
	type SectionAlign,
	sectionDescriptionClasses,
	sectionHeaderClasses,
	sectionHeadingClasses,
	sectionMoreClasses,
	sectionMoreFontStyle,
} from "./homeChrome.styles";

type HomeSectionHeaderProps = {
	eyebrow: ReactNode;
	align?: SectionAlign;
	description?: ReactNode;
	moreHref?: string;
	moreLabel?: ReactNode;
	children: ReactNode;
};

export default function HomeSectionHeader({
	eyebrow,
	align = "between",
	description,
	moreHref,
	moreLabel,
	children,
}: HomeSectionHeaderProps) {
	const heading = (
		<>
			<HomeEyebrow>{eyebrow}</HomeEyebrow>
			<h2 className={sectionHeadingClasses(Boolean(description))}>
				{children}
			</h2>
			{description ? (
				<p className={sectionDescriptionClasses}>{description}</p>
			) : null}
		</>
	);

	if (align === "center") {
		return (
			<header className={sectionHeaderClasses("center")}>{heading}</header>
		);
	}

	return (
		<header className={sectionHeaderClasses("between")}>
			<div>{heading}</div>
			{moreHref ? (
				<Link
					href={moreHref}
					className={sectionMoreClasses}
					style={sectionMoreFontStyle}
				>
					{moreLabel}
				</Link>
			) : null}
		</header>
	);
}
