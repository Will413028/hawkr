"use client";

import * as React from "react";
import StorefrontProductPreviewDrawer from "./StorefrontProductPreviewDrawer";
import { productIdFromStorefrontPreviewHref } from "./storefrontProductPreview";

function interactiveTarget(target: EventTarget | null): Element | null {
	if (!(target instanceof Element)) return null;
	return target.closest(
		'a[href], button, form, input, select, textarea, [role="button"]',
	);
}

function isReadonlyPostPreviewInteraction(target: Element): boolean {
	return target.matches(
		'[data-storefront-post-preview-trigger="true"], [data-storefront-post-preview-control="true"]',
	);
}

/**
 * Preserve the buyer storefront's real visual/hover states while stopping its
 * write actions before child handlers run. Product cards open the existing
 * product drawer; post cards and the detail viewer's reading controls stay live
 * so sellers can inspect the full buyer-facing post without enabling likes,
 * comments, shares or navigation out of the preview. Storefront tabs sit
 * outside this boundary so sellers can still inspect every buyer-facing tab.
 */
export default function StorefrontPreviewBoundary({
	children,
	sellerId,
}: {
	children: React.ReactNode;
	sellerId: string;
}) {
	const [previewProduct, setPreviewProduct] = React.useState<{
		id: string;
		name?: string;
	} | null>(null);
	const openerRef = React.useRef<HTMLAnchorElement | null>(null);

	const closePreview = React.useCallback(() => {
		openerRef.current?.setAttribute("aria-expanded", "false");
		setPreviewProduct(null);
	}, []);

	const blockClick = (event: React.MouseEvent<HTMLDivElement>) => {
		const interactive = interactiveTarget(event.target);
		if (!interactive) return;
		const isPlainPrimaryClick =
			event.type === "click" &&
			event.button === 0 &&
			!event.altKey &&
			!event.ctrlKey &&
			!event.metaKey &&
			!event.shiftKey;

		if (isPlainPrimaryClick && isReadonlyPostPreviewInteraction(interactive)) {
			return;
		}

		event.preventDefault();
		event.stopPropagation();

		if (!isPlainPrimaryClick) return;

		const opener = interactive.closest<HTMLAnchorElement>(
			'a[data-storefront-product-preview-trigger="true"][href]',
		);
		if (!opener) return;
		const productId = productIdFromStorefrontPreviewHref(
			opener.getAttribute("href"),
		);
		if (!productId) return;

		openerRef.current?.setAttribute("aria-expanded", "false");
		openerRef.current = opener;
		opener.setAttribute("aria-expanded", "true");
		opener.focus({ preventScroll: true });
		const name = opener
			.getAttribute("aria-label")
			?.replace(/^查看商品[：:]\s*/, "")
			.trim();
		setPreviewProduct({ id: productId, name: name || undefined });
	};

	const blockSubmit = (event: React.FormEvent<HTMLDivElement>) => {
		event.preventDefault();
		event.stopPropagation();
	};

	return (
		<>
			<div
				data-storefront-preview-readonly="true"
				onClickCapture={blockClick}
				onAuxClickCapture={blockClick}
				onSubmitCapture={blockSubmit}
			>
				{children}
			</div>
			<StorefrontProductPreviewDrawer
				key={previewProduct?.id ?? "closed"}
				productId={previewProduct?.id ?? null}
				productName={previewProduct?.name}
				sellerId={sellerId}
				onClose={closePreview}
			/>
		</>
	);
}
