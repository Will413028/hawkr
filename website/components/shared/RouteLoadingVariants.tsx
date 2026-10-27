import type { ReactNode } from "react";

const PLACEHOLDER_CATEGORIES = Array.from({ length: 14 }, (_, index) => index);
const COMMERCE_ROWS = ["first", "second", "third"];

function Line({ wide = false }: { wide?: boolean }) {
	return (
		<span
			className={`route-loading-skeleton__line${wide ? " route-loading-skeleton__line--wide" : ""}`}
		/>
	);
}

/** Uses HomeCompactMarketStage's distinct post and three-product layouts. */
export function HomeSkeleton({ shelf }: { shelf?: ReactNode }) {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--home"
		>
			<div className="route-loading-skeleton__home-categories">
				{PLACEHOLDER_CATEGORIES.map((category) => (
					<div className="route-loading-skeleton__home-category" key={category}>
						<span className="route-loading-skeleton__media" />
						<Line />
					</div>
				))}
			</div>
			<div className="route-loading-skeleton__home-stage">
				<div className="route-loading-skeleton__home-panel">
					<div className="route-loading-skeleton__home-heading">
						<Line wide />
					</div>
					<div className="route-loading-skeleton__home-post">
						<span className="route-loading-skeleton__media" />
						<div className="route-loading-skeleton__home-post-body">
							<div className="route-loading-skeleton__home-author">
								<span className="route-loading-skeleton__store-avatar" />
								<div>
									<Line wide />
									<Line />
								</div>
							</div>
							<div className="route-loading-skeleton__home-caption">
								<Line wide />
								<Line />
								<Line />
							</div>
							<Line />
						</div>
					</div>
				</div>
				<div className="route-loading-skeleton__home-panel">
					<div className="route-loading-skeleton__home-heading">
						<Line wide />
					</div>
					<div className="route-loading-skeleton__home-products">
						{COMMERCE_ROWS.map((row) => (
							<div className="route-loading-skeleton__home-product" key={row}>
								<span className="route-loading-skeleton__media" />
								<div>
									<Line wide />
									<Line />
									<Line />
								</div>
							</div>
						))}
					</div>
				</div>
			</div>
			<div className="route-loading-skeleton__home-panel route-loading-skeleton__home-panel--shelf">
				<div className="route-loading-skeleton__home-heading">
					<Line wide />
				</div>
				<div className="route-loading-skeleton__home-shelf">{shelf}</div>
			</div>
		</div>
	);
}

export function DefaultSkeleton({ cards }: { cards: ReactNode }) {
	return (
		<div aria-hidden="true" className="route-loading-skeleton__canvas">
			<div className="route-loading-skeleton__masthead">
				<span className="route-loading-skeleton__pill" />
				<span className="route-loading-skeleton__title" />
				<span className="route-loading-skeleton__copy" />
				<span className="route-loading-skeleton__copy route-loading-skeleton__copy--short" />
				<div className="route-loading-skeleton__actions">
					<span />
					<span />
				</div>
			</div>
			{cards}
		</div>
	);
}

/** Welcome-panel geometry; verification and role forms have their own content. */
export function AuthSkeleton() {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--auth"
		>
			<div className="route-loading-skeleton__auth-panel">
				<span className="route-loading-skeleton__auth-icon" />
				<span className="route-loading-skeleton__pill" />
				<span className="route-loading-skeleton__title" />
				<span className="route-loading-skeleton__copy" />
				<div className="route-loading-skeleton__auth-tabs">
					<span />
					<span />
				</div>
				<div className="route-loading-skeleton__auth-actions">
					<span />
					<span />
				</div>
				<div className="route-loading-skeleton__auth-terms">
					<Line />
				</div>
			</div>
		</div>
	);
}

export function CheckoutSkeleton() {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--commerce"
		>
			<div className="route-loading-skeleton__commerce-heading">
				<Line />
				<span className="route-loading-skeleton__pill" />
				<span className="route-loading-skeleton__title" />
				<span className="route-loading-skeleton__copy" />
			</div>
			<div className="route-loading-skeleton__commerce-grid">
				<section className="route-loading-skeleton__commerce-main">
					<Line wide />
					{COMMERCE_ROWS.map((row) => (
						<div className="route-loading-skeleton__commerce-row" key={row}>
							<span />
							<div>
								<span />
								<span />
							</div>
							<span />
						</div>
					))}
				</section>
				<aside className="route-loading-skeleton__commerce-summary">
					<Line wide />
					<Line />
					<Line />
					<Line />
					<div className="route-loading-skeleton__actions">
						<span />
					</div>
				</aside>
			</div>
		</div>
	);
}

export function SearchSkeleton({ cards }: { cards: ReactNode }) {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--search"
		>
			<div className="route-loading-skeleton__search-head">
				<span className="route-loading-skeleton__pill" />
				<span className="route-loading-skeleton__title" />
				<div className="route-loading-skeleton__search-form">
					<span className="route-loading-skeleton__search-bar" />
				</div>
			</div>
			<div className="route-loading-skeleton__search-landing">
				<section className="route-loading-skeleton__search-section">
					<div className="route-loading-skeleton__search-section-heading">
						<Line wide />
						<Line />
					</div>
					{cards}
				</section>
			</div>
		</div>
	);
}

/** The header reflows to avatar/actions, then a full-width title on phones. */
export function SellerStorefrontSkeleton({ cards }: { cards: ReactNode }) {
	return (
		<div
			aria-hidden="true"
			className="route-loading-skeleton__canvas route-loading-skeleton__canvas--storefront"
		>
			<div className="route-loading-skeleton__store-cover" />
			<div className="route-loading-skeleton__store-head">
				<span className="route-loading-skeleton__store-avatar" />
				<div className="route-loading-skeleton__store-title">
					<span className="route-loading-skeleton__title" />
					<span className="route-loading-skeleton__copy route-loading-skeleton__copy--short" />
				</div>
				<div className="route-loading-skeleton__store-actions">
					<span className="route-loading-skeleton__pill" />
					<span className="route-loading-skeleton__pill" />
					<span className="route-loading-skeleton__pill" />
				</div>
			</div>
			<div className="route-loading-skeleton__store-tabs">
				<Line />
				<Line />
				<Line />
				<Line />
			</div>
			{cards}
		</div>
	);
}
