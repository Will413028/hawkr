"use client";

import { type Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import { Chip } from "@/components/ui";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useStoreCardViews } from "@/lib/seller/useStoreCardViews";

export function formatReportTime(time: Timestamp | undefined): string {
	return time ? formatTaipeiDateTime(timestampDate(time)) : "";
}

export function ReportOutcomeChip({ text }: { text: string }) {
	return (
		<Chip size="sm" variant="flat" color="default">
			{text}
		</Chip>
	);
}

export interface StoreNameState {
	name: string | undefined;
	settled: boolean;
}

/** 店名：還沒讀到時顯示骨架；讀完仍沒有卡片時不顯示文字（與 buyer-app 一致，不退回快照）。 */
export function StoreName({ name, settled }: StoreNameState) {
	if (name) return <span>{name}</span>;
	if (settled) return null;
	return (
		<span
			aria-hidden
			data-testid="store-name-skeleton"
			className="inline-block h-4 w-24 rounded bg-content2"
		/>
	);
}

/** 一個畫面一次的店卡讀取；`nameOf` 回傳某店的呈現狀態。 */
export function useStoreNames(storeIds: readonly string[]) {
	const { views, settled } = useStoreCardViews(storeIds);
	return (storeId: string): StoreNameState => ({
		name: views.get(storeId)?.name,
		settled,
	});
}
