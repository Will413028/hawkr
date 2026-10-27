const STATUS_BADGE_CLASS: Record<string, string> = {
	pending_payment:
		"bg-warning-50 text-warning-700 dark:bg-[#f1cd70]/16 dark:!text-[#f1cd70]",
	paid: "bg-hkui-primary-50 text-hkui-primary-700 dark:bg-hk-ink-300/18 dark:!text-hk-ink-200",
	preparing:
		"bg-hkui-primary-50 text-hkui-primary-700 dark:bg-hk-ink-300/18 dark:!text-hk-ink-200",
	shipping:
		"bg-secondary-50 text-secondary-700 dark:bg-hk-ink-300/18 dark:!text-hk-ink-200",
	completed:
		"bg-success-50 text-success-700 dark:bg-hk-success/18 dark:!text-hk-ink-200",
	delivered:
		"bg-success-50 text-success-700 dark:bg-hk-success/18 dark:!text-hk-ink-200",
	cancelled:
		"bg-danger-50 text-danger-700 dark:bg-[#ed6334]/18 dark:!text-[#ffb69c]",
	refunded:
		"bg-danger-50 text-danger-700 dark:bg-[#ed6334]/18 dark:!text-[#ffb69c]",
};

const FALLBACK_STATUS_BADGE_CLASS =
	"bg-default-100 text-default-700 dark:bg-white/10 dark:!text-hk-ink-100";

export function buyerOrderStatusBadgeClass(status: string): string {
	return STATUS_BADGE_CLASS[status] ?? FALLBACK_STATUS_BADGE_CLASS;
}
