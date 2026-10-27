"use client";

import "@/styles/features/cart-typography.css";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { type Address, addressText } from "@/lib/addresses/repo";
import { CartSheetPortal } from "./CartSheetPortal";

interface Props {
	open: boolean;
	addresses: Address[];
	selectedId: string | null;
	loading?: boolean;
	onSelect: (id: string) => void;
	onClose: () => void;
}

/**
 * Bottom-sheet (mobile) / right-anchored dialog (desktop) for picking a
 * saved buyer address at cart-time. Tap a row to select + auto-close;
 * "新增地址" routes to /account/addresses/new with a redirect back to /cart.
 */
export function AddressPickerSheet({
	open,
	addresses,
	selectedId,
	loading,
	onSelect,
	onClose,
}: Props) {
	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") onClose();
		};
		window.addEventListener("keydown", onKey);
		return () => window.removeEventListener("keydown", onKey);
	}, [open, onClose]);

	if (!open) return null;

	return (
		<CartSheetPortal>
			<div
				className="cart-sheet cart-sheet--buyer-cart"
				role="dialog"
				aria-label="選擇收件地址"
				aria-modal="true"
			>
				<button
					type="button"
					className="cart-sheet__scrim"
					aria-label="關閉"
					onClick={onClose}
				/>
				<div className="cart-sheet__panel">
					<header className="cart-sheet__header">
						<h2>選擇收件地址</h2>
						<Button
							type="button"
							variant="light"
							color="default"
							size="md"
							radius="full"
							isIconOnly
							className="cart-sheet__close"
							onClick={onClose}
							aria-label="關閉"
						>
							<AppIcon name="close" size={24} />
						</Button>
					</header>
					<div className="cart-sheet__body">
						{loading && addresses.length === 0 ? (
							<p className="cart-sheet__hint">載入地址中…</p>
						) : addresses.length === 0 ? (
							<div className="cart-sheet__empty">
								<p>還沒有收件地址。</p>
								<Button
									href="/account/addresses/new?redirect=/cart"
									size="lg"
									className="cart-sheet__primary-action"
									onClick={onClose}
								>
									新增地址
								</Button>
							</div>
						) : (
							<ul className="cart-sheet__list">
								{addresses.map((addr) => {
									const active = selectedId === addr.id;
									return (
										<li key={addr.id}>
											<button
												type="button"
												className={`cart-sheet__row${active ? " cart-sheet__row--active" : ""}`}
												onClick={() => {
													onSelect(addr.id);
													onClose();
												}}
											>
												<span
													className="cart-sheet__row-radio"
													aria-hidden="true"
												>
													<span />
												</span>
												<div className="cart-sheet__row-body">
													<strong>
														{addr.receiverName}
														{addr.isDefault ? (
															<span className="cart-sheet__badge">預設</span>
														) : null}
													</strong>
													<span>{addr.receiverPhone}</span>
													<span>{addressText(addr)}</span>
												</div>
											</button>
										</li>
									);
								})}
							</ul>
						)}
					</div>
					<footer className="cart-sheet__footer">
						<Button
							href="/account/addresses/new?redirect=/cart"
							variant="bordered"
							size="md"
							fullWidth
							startContent={<AppIcon name="plus" size={20} />}
							className="cart-sheet__add"
							onClick={onClose}
						>
							新增收件地址
						</Button>
					</footer>
				</div>
			</div>
		</CartSheetPortal>
	);
}
