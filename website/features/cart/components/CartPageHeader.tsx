"use client";

import Button from "@/components/ui/Button";

interface CartPageHeaderProps {
	onClear: () => void;
	clearing?: boolean;
	disabled?: boolean;
}

export function CartPageHeader({
	onClear,
	clearing = false,
	disabled = false,
}: CartPageHeaderProps) {
	return (
		<header className="cart-page__header">
			<h1 className="m-0 text-hk-heading cart-type-page-title">購物車</h1>
			<Button
				type="button"
				variant="light"
				color="danger"
				size="md"
				className="cart-page__link-btn cart-page__link-btn--danger"
				onClick={onClear}
				disabled={disabled || clearing}
				isLoading={clearing}
				loadingLabel="清空中…"
			>
				清空購物車
			</Button>
		</header>
	);
}
