import { cn } from "@/lib/utils/cn";

/**
 * 抽屜（Sheet/Drawer）內容載入時的固定形狀骨架。
 *
 * 內容從「一行載入中」跳成整頁會讓抽屜內容高度大幅跳動（捲軸也跟著
 * 出現/消失），視覺上就是抽屜底部抖動；固定形狀的骨架把載入前後的
 * 高度落差降到最小。原本內嵌在 ProductEditDrawer，抽出供各抽屜共用。
 * @import import { DrawerSkeleton } from '@/components/ui/DrawerSkeleton';
 */
export function DrawerSkeleton({
	rows = 2,
	className,
}: {
	/** 頭像列之下的區塊數，依內容長度粗略對齊（訂單詳情用 3）。 */
	rows?: number;
	className?: string;
}) {
	return (
		<div
			className={cn("flex min-w-0 w-full flex-col gap-5", className)}
			aria-busy="true"
			role="status"
		>
			<div className="flex items-center gap-3">
				<span className="size-14 shrink-0 animate-pulse rounded-hkui-lg bg-hk-surface-muted motion-reduce:animate-none dark:bg-[var(--page-skeleton-bone)]" />
				<div className="flex min-w-0 flex-1 flex-col gap-1.5">
					<span className="h-3 w-1/2 animate-pulse rounded-sm bg-hk-surface-muted motion-reduce:animate-none dark:bg-[var(--page-skeleton-bone)]" />
					<span className="h-3 w-1/4 animate-pulse rounded-sm bg-hk-surface-muted motion-reduce:animate-none dark:bg-[var(--page-skeleton-bone)]" />
				</div>
			</div>
			{Array.from({ length: rows }, (_, i) => i + 1).map((row) => (
				<div
					key={row}
					className="h-24 animate-pulse rounded-hkui-lg bg-hk-surface-muted motion-reduce:animate-none dark:bg-[var(--page-skeleton-bone)]"
				/>
			))}
			<span className="sr-only">載入中</span>
		</div>
	);
}
