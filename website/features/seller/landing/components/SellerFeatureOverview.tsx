"use client";

import { useEffect, useState } from "react";
import { SELLER_FEATURE_OVERVIEW } from "../sellerFeatureOverviewData";
import SellerFeatureOverviewIcon from "./SellerFeatureOverviewIcon";

const FIRST_FEATURE_ID = SELLER_FEATURE_OVERVIEW[0]?.id ?? "";

export default function SellerFeatureOverview() {
	const [activeId, setActiveId] = useState(FIRST_FEATURE_ID);

	useEffect(() => {
		const ids = new Set(SELLER_FEATURE_OVERVIEW.map((group) => group.id));
		const sections = SELLER_FEATURE_OVERVIEW.map((group) =>
			document.getElementById(group.id),
		).filter((section): section is HTMLElement => section !== null);

		const syncFromHash = () => {
			const hashId = window.location.hash.slice(1);
			if (ids.has(hashId)) setActiveId(hashId);
		};

		syncFromHash();
		window.addEventListener("hashchange", syncFromHash);

		let observer: IntersectionObserver | null = null;
		if (typeof IntersectionObserver !== "undefined") {
			observer = new IntersectionObserver(
				(entries) => {
					const closestVisible = entries
						.filter((entry) => entry.isIntersecting)
						.sort(
							(a, b) =>
								Math.abs(a.boundingClientRect.top) -
								Math.abs(b.boundingClientRect.top),
						)[0];

					if (closestVisible?.target.id) setActiveId(closestVisible.target.id);
				},
				{
					rootMargin: "-18% 0px -68% 0px",
					threshold: [0, 0.01],
				},
			);

			for (const section of sections) observer.observe(section);
		}

		return () => {
			window.removeEventListener("hashchange", syncFromHash);
			observer?.disconnect();
		};
	}, []);

	return (
		<section
			className="seller-feature-overview"
			id="seller-tools"
			aria-labelledby="seller-feature-overview-title"
		>
			<header className="seller-feature-overview__intro">
				<h2 id="seller-feature-overview-title">賣家中心功能導覽</h2>
				<p>
					從品牌店面、商品與庫存，到訂單、顧客及營運，九個工作區各自負責一件事。需要商家認證、PayUni
					或物流商開通的能力，會在對應章節直接說明。
				</p>
			</header>

			<div className="seller-feature-overview__layout">
				<nav
					className="seller-feature-overview__nav"
					aria-label="賣家中心功能分類"
				>
					<ul>
						{SELLER_FEATURE_OVERVIEW.map((group) => (
							<li key={group.id}>
								<a
									href={`#${group.id}`}
									aria-current={activeId === group.id ? "location" : undefined}
									onClick={() => setActiveId(group.id)}
								>
									{group.title}
								</a>
							</li>
						))}
					</ul>
				</nav>

				<div className="seller-feature-overview__chapters">
					{SELLER_FEATURE_OVERVIEW.map((group) => (
						<section
							className="seller-feature-overview__chapter"
							id={group.id}
							key={group.id}
							aria-labelledby={`${group.id}-title`}
						>
							<div className="seller-feature-overview__chapter-copy">
								<div className="seller-feature-overview__chapter-heading">
									<h3 id={`${group.id}-title`}>{group.title}</h3>
									<SellerFeatureOverviewIcon icon={group.icon} />
								</div>
								<p>{group.summary}</p>
							</div>

							<ul className="seller-feature-overview__feature-list">
								{group.features.map((feature) => (
									<li key={feature}>{feature}</li>
								))}
							</ul>
						</section>
					))}
				</div>
			</div>
		</section>
	);
}
