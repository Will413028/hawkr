import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";

interface RestockNotifyButtonProps {
	/** Product detail route, where the buyer picks a SKU and subscribes. */
	href: string;
	productName?: string;
	className?: string;
}

/**
 * Out-of-stock "補貨通知" entry on market cards. Restock subscriptions are
 * per SKU and a card carries no SKU, so this routes to the product page.
 *
 * Distinct from favouriting — see lib/restock-subscriptions/repo.ts.
 */
export default function RestockNotifyButton({
	href,
	productName,
	className,
}: RestockNotifyButtonProps) {
	return (
		<Link
			href={href}
			aria-label={`補貨通知${productName ? `：${productName}` : ""}`}
			className={`bg-hk-secondary-soft text-hk-primary-strong dark:bg-white/10 dark:text-[var(--page-restock-fg-on-dark)] dark:shadow-[inset_0_0_0_1px_var(--page-restock-ring-on-dark)] no-underline ${className ?? ""}`.trim()}
		>
			<span className="market-product-card__cart-btn-label">補貨通知</span>
			<AppIcon
				className="market-product-card__cart-btn-icon"
				name="notification"
				size={22}
				style={{ color: "var(--icon-accent)" }}
			/>
		</Link>
	);
}
